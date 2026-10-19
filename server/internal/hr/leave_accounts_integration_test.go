package hr

import (
	"context"
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
