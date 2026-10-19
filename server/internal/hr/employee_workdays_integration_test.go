package hr

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Arbeitstage eines Mitarbeiters und Zaehlung von Urlaubsantraegen
// (Backlog F.8.2.3). Jeder Test in einem eigenen Mandanten, damit das
// eingestellte Bundesland keine anderen Tests beeinflusst.

func seedWorkdaysCompany(t *testing.T, env *testutil.IntegrationEnv) string {
	t.Helper()
	company := "wd-" + uuid.NewString()[:8]
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO company_profiles (id, name) VALUES ($1,$2)`, company, "Arbeitstage "+company); err != nil {
		t.Fatalf("Mandant anlegen: %v", err)
	}
	return company
}

func TestEmployeeWorkweekFieldsStored(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedWorkdaysCompany(t, env)
	svc := NewEmployeeService(env.PG)

	std, err := svc.Create(ctx, EmployeeInput{FirstName: "Voll", LastName: "Zeit"}, company)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(ctx, std.ID, company)
	if err != nil || got.Bundesland != nil || !reflect.DeepEqual(got.Arbeitstage, []int{1, 2, 3, 4, 5}) {
		t.Fatalf("Vorgaben: %+v, %v", got, err)
	}

	land := "sn"
	part, err := svc.Create(ctx, EmployeeInput{FirstName: "Teil", LastName: "Zeit", Bundesland: &land, Arbeitstage: []int{3, 1, 2}}, company)
	if err != nil {
		t.Fatal(err)
	}
	got, err = svc.Get(ctx, part.ID, company)
	if err != nil || got.Bundesland == nil || *got.Bundesland != "SN" || !reflect.DeepEqual(got.Arbeitstage, []int{1, 2, 3}) {
		t.Fatalf("Teilzeit: %+v, %v", got, err)
	}
	list, err := svc.List(ctx, 50, 0, company)
	if err != nil || len(list) != 2 {
		t.Fatalf("Liste: %v, %v", list, err)
	}
	for _, e := range list {
		if e.ID == part.ID && (!reflect.DeepEqual(e.Arbeitstage, []int{1, 2, 3}) || e.Bundesland == nil) {
			t.Fatalf("Liste liest Felder nicht: %+v", e)
		}
	}

	bad := "XX"
	_, err = svc.Create(ctx, EmployeeInput{FirstName: "a", LastName: "b", Bundesland: &bad}, company)
	wantEmployeeError(t, err, "Ungültiges Bundesland")
	_, err = svc.Create(ctx, EmployeeInput{FirstName: "a", LastName: "b", Arbeitstage: []int{}}, company)
	wantEmployeeError(t, err, "Mindestens ein Arbeitstag erforderlich")

	// Patch: setzen, zuruecksetzen, Fehler ohne Aenderung.
	if err := svc.Update(ctx, std.ID, map[string]any{"bundesland": "by", "arbeitstage": []any{float64(2), float64(4)}}, company); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.Get(ctx, std.ID, company)
	if got.Bundesland == nil || *got.Bundesland != "BY" || !reflect.DeepEqual(got.Arbeitstage, []int{2, 4}) {
		t.Fatalf("Patch: %+v", got)
	}
	wantEmployeeError(t, svc.Update(ctx, std.ID, map[string]any{"arbeitstage": []any{}}, company), "Mindestens ein Arbeitstag erforderlich")
	wantEmployeeError(t, svc.Update(ctx, std.ID, map[string]any{"arbeitstage": nil}, company), "Ungültiger Wert für Feld: arbeitstage")
	wantEmployeeError(t, svc.Update(ctx, std.ID, map[string]any{"bundesland": "XX"}, company), "Ungültiges Bundesland")
	if err := svc.Update(ctx, std.ID, map[string]any{"bundesland": nil}, company); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.Get(ctx, std.ID, company)
	if got.Bundesland != nil || !reflect.DeepEqual(got.Arbeitstage, []int{2, 4}) {
		t.Fatalf("Zuruecksetzen: %+v", got)
	}
}

func TestEmployeeWorkDaysAndLeaveCounting(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedWorkdaysCompany(t, env)
	other := seedWorkdaysCompany(t, env)
	emps := NewEmployeeService(env.PG)
	cal := NewCalendarService(env.PG)
	leaves := NewLeaveService(env.PG)

	emp, err := emps.Create(ctx, EmployeeInput{FirstName: "Mo", LastName: "Fr"}, company)
	if err != nil {
		t.Fatal(err)
	}
	// Woche 01.-07.06.2026: Mo-So, Fronleichnam am Donnerstag 04.06.
	mon, thu, fri, sun := leaveDay(2026, 6, 1), leaveDay(2026, 6, 4), leaveDay(2026, 6, 5), leaveDay(2026, 6, 7)
	wd := func(id uuid.UUID) int {
		t.Helper()
		n, err := cal.EmployeeWorkDays(ctx, id, mon, sun, company)
		if err != nil {
			t.Fatalf("Arbeitstage: %v", err)
		}
		return n
	}

	// Ohne Bundesland: Fronleichnam ist kein bundesweiter Feiertag.
	if n := wd(emp.ID); n != 5 {
		t.Fatalf("ohne Bundesland %d, erwartet 5", n)
	}
	if _, err := cal.SetSettings(ctx, CalendarSettings{Bundesland: "NW"}, "", company); err != nil {
		t.Fatal(err)
	}
	if n := wd(emp.ID); n != 4 {
		t.Fatalf("NW %d, erwartet 4 (Fronleichnam)", n)
	}
	// Mitarbeiter in Sachsen: dort kein Fronleichnam.
	if err := emps.Update(ctx, emp.ID, map[string]any{"bundesland": "SN"}, company); err != nil {
		t.Fatal(err)
	}
	if n := wd(emp.ID); n != 5 {
		t.Fatalf("Mitarbeiter SN %d, erwartet 5", n)
	}
	if err := emps.Update(ctx, emp.ID, map[string]any{"bundesland": nil}, company); err != nil {
		t.Fatal(err)
	}
	// Betrieblicher freier Tag am Freitag; im fremden Mandanten zaehlt er nicht.
	if _, err := cal.CreateCompanyDay(ctx, fri, "Brückentag", "", company); err != nil {
		t.Fatal(err)
	}
	if _, err := cal.CreateCompanyDay(ctx, mon, "fremd", "", other); err != nil {
		t.Fatal(err)
	}
	if n := wd(emp.ID); n != 3 {
		t.Fatalf("mit Brückentag %d, erwartet 3", n)
	}
	// Teilzeit Mo-Mi: drei Tage, keiner frei.
	if err := emps.Update(ctx, emp.ID, map[string]any{"arbeitstage": []any{float64(1), float64(2), float64(3)}}, company); err != nil {
		t.Fatal(err)
	}
	if n := wd(emp.ID); n != 3 {
		t.Fatalf("Teilzeit %d, erwartet 3", n)
	}
	if n, _ := cal.EmployeeWorkDays(ctx, emp.ID, thu, sun, company); n != 0 {
		t.Fatalf("Teilzeit Do-So %d, erwartet 0", n)
	}
	// In einer Woche ohne freie Tage (08.-14.06.) zaehlen nur Mo-Mi - die
	// Woche davor waere mit der 5-Tage-Woche zufaellig gleich.
	if n, _ := cal.EmployeeWorkDays(ctx, emp.ID, leaveDay(2026, 6, 8), leaveDay(2026, 6, 14), company); n != 3 {
		t.Fatalf("Teilzeit volle Woche %d, erwartet 3", n)
	}
	// Fremder Mandant kennt den Mitarbeiter nicht; Ende vor Beginn = 0.
	_, err = cal.EmployeeWorkDays(ctx, emp.ID, mon, sun, other)
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
	if n, err := cal.EmployeeWorkDays(ctx, emp.ID, sun, mon, company); err != nil || n != 0 {
		t.Fatalf("Ende vor Beginn: %d, %v", n, err)
	}

	// Urlaubsantrag zaehlt dieselben Arbeitstage.
	if err := emps.Update(ctx, emp.ID, map[string]any{"arbeitstage": []any{float64(1), float64(2), float64(3), float64(4), float64(5)}}, company); err != nil {
		t.Fatal(err)
	}
	lr, err := leaves.Create(ctx, LeaveRequest{EmployeeID: emp.ID, StartDate: mon, EndDate: sun}, company)
	if err != nil {
		t.Fatal(err)
	}
	if lr.Days != 3 {
		t.Fatalf("Antrag %v Tage, erwartet 3 (Mo-Mi; Do Fronleichnam, Fr Brückentag)", lr.Days)
	}
	emp2, _ := emps.Create(ctx, EmployeeInput{FirstName: "Zwei", LastName: "Te"}, company)
	_, err = leaves.Create(ctx, LeaveRequest{EmployeeID: emp2.ID, StartDate: mon, EndDate: sun, Days: 3.5}, company)
	wantEmployeeError(t, err, "Tage dürfen die Arbeitstage des Zeitraums nicht überschreiten")
	half, err := leaves.Create(ctx, LeaveRequest{EmployeeID: emp2.ID, StartDate: mon, EndDate: mon, Days: 0.5}, company)
	if err != nil || half.Days != 0.5 {
		t.Fatalf("halber Tag: %+v, %v", half, err)
	}
	// Nur freie Tage: kein Antrag.
	_, err = leaves.Create(ctx, LeaveRequest{EmployeeID: emp2.ID, StartDate: thu, EndDate: sun}, company)
	wantEmployeeError(t, err, "Ungültiger Zeitraum: er enthält keinen Arbeitstag")
}
