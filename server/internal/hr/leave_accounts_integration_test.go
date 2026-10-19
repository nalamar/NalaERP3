package hr

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Urlaubskonto (Backlog F.8.3.1). Eigener Mandant je Test, ohne
// Bundesland: es gelten nur die bundesweiten Feiertage.

func setLeaveStatus(t *testing.T, env *testutil.IntegrationEnv, id uuid.UUID, status string) {
	t.Helper()
	if _, err := env.PG.Exec(context.Background(), `UPDATE hr_leave_requests SET status=$2 WHERE id=$1`, id, status); err != nil {
		t.Fatalf("Status setzen: %v", err)
	}
}

func wantAccount(t *testing.T, acc *LeaveAccount, err error, anspruch, uebertrag, genommen, verplant float64, festgelegt bool) {
	t.Helper()
	if err != nil {
		t.Fatalf("Konto: %v", err)
	}
	if acc.AnspruchTage != anspruch || acc.UebertragTage != uebertrag || acc.GenommenTage != genommen || acc.VerplantTage != verplant ||
		acc.Festgelegt != festgelegt || acc.RestTage != anspruch+uebertrag-genommen || acc.VerfuegbarTage != anspruch+uebertrag-genommen-verplant {
		t.Fatalf("Konto %+v, erwartet Anspruch %v, Übertrag %v, genommen %v, verplant %v, festgelegt %v", acc, anspruch, uebertrag, genommen, verplant, festgelegt)
	}
}

func TestEmployeeLeaveEntitlementMinimum(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedWorkdaysCompany(t, env)
	emps := NewEmployeeService(env.PG)
	f := func(v float64) *float64 { return &v }

	_, err := emps.Create(ctx, EmployeeInput{FirstName: "a", LastName: "b", UrlaubstageJahr: f(19)}, company)
	wantEmployeeError(t, err, "Ungültiger Urlaubsanspruch: mindestens 20 Tage bei 5 Arbeitstagen je Woche (gesetzlicher Mindesturlaub)")
	part, err := emps.Create(ctx, EmployeeInput{FirstName: "Teil", LastName: "Zeit", Arbeitstage: []int{1, 2, 3}, UrlaubstageJahr: f(12)}, company)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := emps.Get(ctx, part.ID, company)
	if got.UrlaubstageJahr == nil || *got.UrlaubstageJahr != 12 {
		t.Fatalf("Anspruch nicht gespeichert: %+v", got)
	}
	// Mehr Arbeitstage heben die Untergrenze: 12 Tage bei 4 Tagen je Woche
	// reichen nicht. Nichts wird geaendert.
	wantEmployeeError(t, emps.Update(ctx, part.ID, map[string]any{"arbeitstage": []any{float64(1), float64(2), float64(3), float64(4)}}, company),
		"Ungültiger Urlaubsanspruch: mindestens 16 Tage bei 4 Arbeitstagen je Woche (gesetzlicher Mindesturlaub)")
	wantEmployeeError(t, emps.Update(ctx, part.ID, map[string]any{"urlaubstage_jahr": float64(11)}, company),
		"Ungültiger Urlaubsanspruch: mindestens 12 Tage bei 3 Arbeitstagen je Woche (gesetzlicher Mindesturlaub)")
	wantEmployeeError(t, emps.Update(ctx, part.ID, map[string]any{"urlaubstage_jahr": "zwanzig"}, company), "Ungültiger Wert für Feld: urlaubstage_jahr")
	got, _ = emps.Get(ctx, part.ID, company)
	if len(got.Arbeitstage) != 3 || *got.UrlaubstageJahr != 12 {
		t.Fatalf("abgelehnte Aenderung hat gespeichert: %+v", got)
	}
	// Beides zusammen geht; zurueck auf die Vorgabe mit null.
	if err := emps.Update(ctx, part.ID, map[string]any{"arbeitstage": []any{float64(1), float64(2), float64(3), float64(4)}, "urlaubstage_jahr": float64(18.5)}, company); err != nil {
		t.Fatal(err)
	}
	if err := emps.Update(ctx, part.ID, map[string]any{"urlaubstage_jahr": nil}, company); err != nil {
		t.Fatal(err)
	}
	got, _ = emps.Get(ctx, part.ID, company)
	if got.UrlaubstageJahr != nil {
		t.Fatalf("Vorgabe nicht wiederhergestellt: %+v", got)
	}
	// Die Datenbank sichert die Untergrenze selbst ab.
	if _, err := env.PG.Exec(ctx, `UPDATE hr_employees SET urlaubstage_jahr=10 WHERE id=$1`, part.ID); err == nil {
		t.Fatal("CHECK der Untergrenze fehlt")
	}
	wantEmployeeError(t, emps.Update(ctx, uuid.New(), map[string]any{"urlaubstage_jahr": float64(30)}, company), "Mitarbeiter nicht gefunden")
}

func TestLeaveAccountBalance(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedWorkdaysCompany(t, env)
	other := seedWorkdaysCompany(t, env)
	emps := NewEmployeeService(env.PG)
	leaves := NewLeaveService(env.PG)
	accounts := NewLeaveAccountService(env.PG)

	emp, err := emps.Create(ctx, EmployeeInput{FirstName: "Konto", LastName: "Inhaber"}, company)
	if err != nil {
		t.Fatal(err)
	}
	acc, err := accounts.Get(ctx, emp.ID, 2026, company)
	wantAccount(t, acc, err, 20, 0, 0, 0, false)

	// Ausgehandelter Anspruch am Mitarbeiter gilt fuer jedes Jahr ohne Konto.
	if err := emps.Update(ctx, emp.ID, map[string]any{"urlaubstage_jahr": float64(28)}, company); err != nil {
		t.Fatal(err)
	}
	newLeave := func(start, end time.Time, typ string, days float64) uuid.UUID {
		t.Helper()
		lr, err := leaves.Create(ctx, LeaveRequest{EmployeeID: emp.ID, StartDate: start, EndDate: end, Typ: typ, Days: days}, company)
		if err != nil {
			t.Fatalf("Antrag %s: %v", start.Format("2006-01-02"), err)
		}
		return lr.ID
	}
	// Genehmigt 5, offen 2, genehmigt unbezahlt (zaehlt nicht), abgelehnt (zaehlt nicht).
	setLeaveStatus(t, env, newLeave(leaveDay(2026, 6, 8), leaveDay(2026, 6, 12), "vacation", 0), "approved")
	newLeave(leaveDay(2026, 7, 6), leaveDay(2026, 7, 7), "vacation", 0)
	setLeaveStatus(t, env, newLeave(leaveDay(2026, 8, 3), leaveDay(2026, 8, 4), "unpaid", 0), "approved")
	setLeaveStatus(t, env, newLeave(leaveDay(2026, 9, 7), leaveDay(2026, 9, 11), "vacation", 0), "rejected")
	acc, err = accounts.Get(ctx, emp.ID, 2026, company)
	wantAccount(t, acc, err, 28, 0, 5, 2, false)

	// Ueber den Jahreswechsel: 28.12.2026-08.01.2027 = 4 + 5 Arbeitstage
	// (01.01. Feiertag); 8,5 Tage beantragt, anteilig 4 und 4,5.
	setLeaveStatus(t, env, newLeave(leaveDay(2026, 12, 28), leaveDay(2027, 1, 8), "vacation", 8.5), "approved")
	acc, err = accounts.Get(ctx, emp.ID, 2026, company)
	wantAccount(t, acc, err, 28, 0, 9, 2, false)
	acc, err = accounts.Get(ctx, emp.ID, 2027, company)
	wantAccount(t, acc, err, 28, 0, 4.5, 0, false)

	// Konto festlegen: Teilurlaub darf unter der Untergrenze liegen.
	hint := time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC)
	acc, err = accounts.Set(ctx, emp.ID, 2026, LeaveAccountInput{AnspruchTage: 10, UebertragTage: 3.5, HinweisErteiltAm: &hint, Notiz: " Eintritt Juli "}, "", company)
	wantAccount(t, acc, err, 10, 3.5, 9, 2, true)
	if acc.HinweisErteiltAm == nil || acc.HinweisErteiltAm.Format("2006-01-02") != "2026-10-01" || acc.Notiz != "Eintritt Juli" || acc.UpdatedAt == nil {
		t.Fatalf("Angaben nicht gespeichert: %+v", acc)
	}
	// Ersatz: ohne Hinweis ist der Hinweis wieder leer.
	acc, err = accounts.Set(ctx, emp.ID, 2026, LeaveAccountInput{AnspruchTage: 14}, "", company)
	wantAccount(t, acc, err, 14, 0, 9, 2, true)
	if acc.HinweisErteiltAm != nil || acc.Notiz != "" {
		t.Fatalf("Ersatz unvollstaendig: %+v", acc)
	}
	// Das Folgejahr bleibt unberuehrt.
	acc, err = accounts.Get(ctx, emp.ID, 2027, company)
	wantAccount(t, acc, err, 28, 0, 4.5, 0, false)

	_, err = accounts.Set(ctx, emp.ID, 2026, LeaveAccountInput{AnspruchTage: -1}, "", company)
	wantEmployeeError(t, err, "Ungültiger Wert für anspruch_tage: ganze oder halbe Tage von 0 bis 366")
	_, err = accounts.Set(ctx, emp.ID, 2026, LeaveAccountInput{AnspruchTage: 20, UebertragTage: 0.3}, "", company)
	wantEmployeeError(t, err, "Ungültiger Wert für uebertrag_tage: ganze oder halbe Tage von 0 bis 366")
	_, err = accounts.Set(ctx, emp.ID, 1800, LeaveAccountInput{AnspruchTage: 20}, "", company)
	wantEmployeeError(t, err, "Ungültiges Jahr")
	_, err = accounts.Get(ctx, emp.ID, 2201, company)
	wantEmployeeError(t, err, "Ungültiges Jahr")
	// Fremder Mandant: weder lesen noch festlegen.
	_, err = accounts.Get(ctx, emp.ID, 2026, other)
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
	// Auch fuer einen Mitarbeiter ohne Antraege - sonst fiele die
	// fehlende Pruefung erst bei der Zaehlung ueber den Jahreswechsel auf.
	quiet, err := emps.Create(ctx, EmployeeInput{FirstName: "Ohne", LastName: "Antrag"}, company)
	if err != nil {
		t.Fatal(err)
	}
	_, err = accounts.Get(ctx, quiet.ID, 2026, other)
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
	_, err = accounts.Set(ctx, emp.ID, 2026, LeaveAccountInput{AnspruchTage: 99}, "", other)
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
	acc, err = accounts.Get(ctx, emp.ID, 2026, company)
	wantAccount(t, acc, err, 14, 0, 9, 2, true)
}

func TestLeaveAccountProposalAndRemaining(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedWorkdaysCompany(t, env)
	other := seedWorkdaysCompany(t, env)
	emps := NewEmployeeService(env.PG)
	accounts := NewLeaveAccountService(env.PG)
	leaves := NewLeaveService(env.PG)
	date := func(y int, m time.Month, d int) *time.Time {
		v := leaveDay(y, m, d)
		return &v
	}
	inactive := false
	mk := func(last string, in EmployeeInput) uuid.UUID {
		t.Helper()
		in.FirstName, in.LastName = "Rest", last
		e, err := emps.Create(ctx, in, company)
		if err != nil {
			t.Fatalf("%s: %v", last, err)
		}
		return e.ID
	}
	newcomer := mk("Neu", EmployeeInput{HireDate: date(2026, 7, 2)})
	veteran := mk("Alt", EmployeeInput{HireDate: date(2020, 4, 1)})
	leftThisYear := mk("Gegangen", EmployeeInput{HireDate: date(2020, 4, 1), Termination: date(2026, 3, 31), Active: &inactive})
	leftLastYear := mk("Frueher", EmployeeInput{HireDate: date(2020, 4, 1), Termination: date(2025, 6, 30), Active: &inactive})
	noRest := mk("Leer", EmployeeInput{})
	traveller := mk("Bereist", EmployeeInput{})
	if _, err := emps.Create(ctx, EmployeeInput{FirstName: "Fremd", LastName: "Firma"}, other); err != nil {
		t.Fatal(err)
	}

	// Vorschlag im Konto, aber nicht uebernommen.
	acc, err := accounts.Get(ctx, newcomer, 2026, company)
	wantAccount(t, acc, err, 20, 0, 0, 0, false)
	if acc.Teilurlaub == nil || acc.Teilurlaub.Tage != 8.5 || acc.Teilurlaub.Monate != 5 {
		t.Fatalf("Vorschlag Eintritt 02.07.: %+v", acc.Teilurlaub)
	}
	// Im Folgejahr kein Vorschlag mehr.
	if acc, err = accounts.Get(ctx, newcomer, 2027, company); err != nil || acc.Teilurlaub != nil {
		t.Fatalf("Folgejahr: %+v, %v", acc, err)
	}
	if acc, err = accounts.Get(ctx, veteran, 2026, company); err != nil || acc.Teilurlaub != nil {
		t.Fatalf("ganzes Jahr: %+v, %v", acc, err)
	}
	// Der Vorschlag rechnet mit dem Jahresanspruch des Mitarbeiters, nicht
	// mit dem festgelegten Konto.
	if err := emps.Update(ctx, newcomer, map[string]any{"urlaubstage_jahr": float64(30)}, company); err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.Set(ctx, newcomer, 2026, LeaveAccountInput{AnspruchTage: 12.5}, "", company); err != nil {
		t.Fatal(err)
	}
	acc, err = accounts.Get(ctx, newcomer, 2026, company)
	wantAccount(t, acc, err, 12.5, 0, 0, 0, true)
	// 30 * 5/12 = 12,5 - ein halber Tag wird aufgerundet (§5 Abs. 2).
	if acc.Teilurlaub == nil || acc.Teilurlaub.Tage != 13 {
		t.Fatalf("Vorschlag bei 30 Tagen: %+v", acc.Teilurlaub)
	}

	// Resturlaub: ohne Rest und ausserhalb des Jahres nicht dabei.
	if _, err := accounts.Set(ctx, noRest, 2026, LeaveAccountInput{AnspruchTage: 0}, "", company); err != nil {
		t.Fatal(err)
	}
	lr, err := leaves.Create(ctx, LeaveRequest{EmployeeID: traveller, StartDate: leaveDay(2026, 6, 8), EndDate: leaveDay(2026, 6, 12)}, company)
	if err != nil {
		t.Fatal(err)
	}
	setLeaveStatus(t, env, lr.ID, "approved")
	list, err := accounts.Remaining(ctx, 2026, company)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range list {
		names = append(names, r.LastName)
		switch r.EmployeeID {
		case traveller:
			if r.Konto.RestTage != 15 {
				t.Fatalf("Rest nach Urlaub: %+v", r.Konto)
			}
		case leftThisYear:
			if r.Konto.Teilurlaub == nil || r.Konto.Teilurlaub.Tage != 5 {
				t.Fatalf("Austritt 31.03.: %+v", r.Konto.Teilurlaub)
			}
		case leftLastYear, noRest:
			t.Fatalf("gehoert nicht in die Liste: %+v", r)
		}
	}
	if got := strings.Join(names, ","); got != "Alt,Bereist,Gegangen,Neu" {
		t.Fatalf("Liste: %s", got)
	}
	_, err = accounts.Remaining(ctx, 1800, company)
	wantEmployeeError(t, err, "Ungültiges Jahr")
	// Der andere Mandant sieht nur seinen Mitarbeiter.
	otherList, err := accounts.Remaining(ctx, 2026, other)
	if err != nil || len(otherList) != 1 || otherList[0].LastName != "Firma" {
		t.Fatalf("fremder Mandant: %+v, %v", otherList, err)
	}
}
