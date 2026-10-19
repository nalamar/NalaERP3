package hr

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Krankmeldungen und Rueckbuchung nach §9 BUrlG (Backlog F.8.4.2).

func TestSickReportsLifecycle(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedWorkdaysCompany(t, env)
	other := seedWorkdaysCompany(t, env)
	actor := testutil.SeedAuthUser(t, env, "krank-"+uuid.NewString()[:8]+"@example.com", "Secret123!", "hr")
	emps := NewEmployeeService(env.PG)
	emp, err := emps.Create(ctx, EmployeeInput{FirstName: "Kr", LastName: "Ank"}, company)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := emps.Create(ctx, EmployeeInput{FirstName: "Fr", LastName: "Emd"}, other)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewAbsenceService(env.PG)
	rep := func(start, end int, attest, bySelf bool) (*Absence, error) {
		return svc.ReportSick(ctx, SickReport{EmployeeID: emp.ID, StartDate: leaveDay(2026, 3, start), EndDate: leaveDay(2026, 3, end), Attest: attest}, bySelf, actor, company)
	}

	// Personalabteilung: bestaetigt, Attest wie angegeben.
	byHR, err := rep(2, 4, true, false)
	if err != nil || byHR.Typ != "sick" || byHR.GemeldetVon != "personal" || !byHR.Attest || byHR.BestaetigtAt == nil ||
		byHR.SourceRequestID != nil || byHR.StartDate.Format("2006-01-02") != "2026-03-02" || byHR.GemeldetByUserID == nil {
		t.Fatalf("Meldung der Personalabteilung: %+v, %v", byHR, err)
	}
	// Selbstmeldung: Attest wird ignoriert, nicht bestaetigt.
	self, err := rep(9, 10, true, true)
	if err != nil || self.GemeldetVon != "selbst" || self.Attest || self.BestaetigtAt != nil {
		t.Fatalf("Selbstmeldung: %+v, %v", self, err)
	}
	if _, err := svc.ReportSick(ctx, SickReport{EmployeeID: foreign.ID, StartDate: leaveDay(2026, 3, 9), EndDate: leaveDay(2026, 3, 9)}, true, actor, other); err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListNewSick(ctx, company)
	if err != nil || len(list) != 1 || list[0].ID != self.ID {
		t.Fatalf("neue Krankmeldungen: %+v, %v", list, err)
	}

	// Ueberschneidung, Fehler, fremder Mandant.
	_, err = rep(4, 6, false, false)
	wantEmployeeError(t, err, "Zeitraum überschneidet sich mit einer bestehenden Krankmeldung")
	if _, err := rep(5, 6, false, false); err != nil {
		t.Fatalf("direkt anschliessend: %v", err)
	}
	_, err = rep(12, 11, false, false)
	wantEmployeeError(t, err, "Enddatum darf nicht vor Startdatum liegen")
	_, err = svc.ReportSick(ctx, SickReport{EmployeeID: emp.ID}, false, actor, company)
	wantEmployeeError(t, err, "start/end erforderlich")
	_, err = svc.ReportSick(ctx, SickReport{EmployeeID: emp.ID, StartDate: leaveDay(2026, 3, 20), EndDate: leaveDay(2026, 3, 20)}, false, actor, other)
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
	_, err = svc.Get(ctx, self.ID, other)
	wantEmployeeError(t, err, "Abwesenheit nicht gefunden")
	_, err = svc.Confirm(ctx, self.ID, actor, other)
	wantEmployeeError(t, err, "Abwesenheit nicht gefunden")

	// Bestaetigen, Attest, Storno.
	if a, err := svc.SetAttest(ctx, self.ID, true, company); err != nil || !a.Attest {
		t.Fatalf("Attest: %+v, %v", a, err)
	}
	c, err := svc.Confirm(ctx, self.ID, actor, company)
	if err != nil || c.BestaetigtAt == nil || c.BestaetigtByUserID == nil || *c.BestaetigtByUserID != actor {
		t.Fatalf("bestaetigen: %+v, %v", c, err)
	}
	_, err = svc.Confirm(ctx, self.ID, actor, company)
	wantEmployeeError(t, err, "Krankmeldung ist bereits bestätigt")
	if list, _ := svc.ListNewSick(ctx, company); len(list) != 0 {
		t.Fatalf("bestaetigte noch in der Liste: %+v", list)
	}
	_, err = svc.Storno(ctx, byHR.ID, " ", actor, company)
	wantEmployeeError(t, err, "Begründung erforderlich")
	st, err := svc.Storno(ctx, byHR.ID, " falscher Mitarbeiter ", actor, company)
	if err != nil || st.Status != "storniert" || st.StornoReason != "falscher Mitarbeiter" || st.StorniertAt == nil {
		t.Fatalf("Storno: %+v, %v", st, err)
	}
	_, err = svc.Storno(ctx, byHR.ID, "nochmal", actor, company)
	wantEmployeeError(t, err, "Krankmeldung ist bereits storniert")
	_, err = svc.SetAttest(ctx, byHR.ID, false, company)
	wantEmployeeError(t, err, "Krankmeldung ist bereits storniert")
	// Nach dem Storno ist der Zeitraum frei.
	if _, err := rep(2, 4, false, false); err != nil {
		t.Fatalf("nach Storno: %v", err)
	}
	// Eine stornierte Selbstmeldung erscheint nicht als neu.
	s2, _ := rep(23, 23, false, true)
	if _, err := svc.Storno(ctx, s2.ID, "doch gesund", actor, company); err != nil {
		t.Fatal(err)
	}
	if list, _ := svc.ListNewSick(ctx, company); len(list) != 0 {
		t.Fatalf("stornierte als neu: %+v", list)
	}

	// Urlaubsabwesenheiten werden ueber den Antrag storniert.
	leaves := NewLeaveService(env.PG)
	lr, err := leaves.Create(ctx, LeaveRequest{EmployeeID: emp.ID, StartDate: leaveDay(2026, 4, 13), EndDate: leaveDay(2026, 4, 14)}, company)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := leaves.Decide(ctx, lr.ID, true, LeaveDecisionRight{All: true}, actor, nil, company); err != nil {
		t.Fatal(err)
	}
	var vacID uuid.UUID
	if err := env.PG.QueryRow(ctx, `SELECT id FROM hr_absences WHERE source_request_id=$1`, lr.ID).Scan(&vacID); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Storno(ctx, vacID, "x", actor, company)
	wantEmployeeError(t, err, "Ungültige Abwesenheit: keine Krankmeldung - Urlaub wird über den Urlaubsantrag storniert")

	// Krankheit wird nicht beantragt.
	_, err = leaves.Create(ctx, LeaveRequest{EmployeeID: emp.ID, StartDate: leaveDay(2026, 5, 4), EndDate: leaveDay(2026, 5, 4), Typ: "sick"}, company)
	wantEmployeeError(t, err, "Ungültiger Antragstyp: Krankheit wird als Krankmeldung erfasst, nicht beantragt")
	// Die DB laesst keine Abwesenheit ohne Antrag zu, die keine Krankmeldung ist.
	if _, err := env.PG.Exec(ctx, `INSERT INTO hr_absences (id, employee_id, typ, start_date, end_date, gemeldet_von) VALUES ($1,$2,'vacation','2026-05-04','2026-05-04','personal')`, uuid.New(), emp.ID); err == nil {
		t.Fatal("Urlaub ohne Antrag angenommen")
	}
}

func TestSickDuringVacationReturnsDays(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedWorkdaysCompany(t, env)
	actor := testutil.SeedAuthUser(t, env, "neun-"+uuid.NewString()[:8]+"@example.com", "Secret123!", "hr")
	emp, err := NewEmployeeService(env.PG).Create(ctx, EmployeeInput{FirstName: "Ur", LastName: "Laub"}, company)
	if err != nil {
		t.Fatal(err)
	}
	leaves := NewLeaveService(env.PG)
	absences := NewAbsenceService(env.PG)
	accounts := NewLeaveAccountService(env.PG)
	// Urlaub Mo 08.06.-Fr 12.06.2026 (5 Tage), genehmigt; ein offener
	// Antrag im Juli.
	lr, err := leaves.Create(ctx, LeaveRequest{EmployeeID: emp.ID, StartDate: leaveDay(2026, 6, 8), EndDate: leaveDay(2026, 6, 12)}, company)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := leaves.Decide(ctx, lr.ID, true, LeaveDecisionRight{All: true}, actor, nil, company); err != nil {
		t.Fatal(err)
	}
	if _, err := leaves.Create(ctx, LeaveRequest{EmployeeID: emp.ID, StartDate: leaveDay(2026, 7, 6), EndDate: leaveDay(2026, 7, 7)}, company); err != nil {
		t.Fatal(err)
	}
	check := func(genommen, krank float64) {
		t.Helper()
		acc, err := accounts.Get(ctx, emp.ID, 2026, company)
		if err != nil {
			t.Fatal(err)
		}
		if acc.GenommenTage != genommen || acc.KrankImUrlaubTage != krank || acc.RestTage != 20-genommen || acc.VerplantTage != 2 {
			t.Fatalf("Konto %+v, erwartet genommen %v, krank %v", acc, genommen, krank)
		}
	}
	check(5, 0)
	// Krank Mi-Do ohne Attest: zaehlt weiter als Urlaub.
	sick, err := absences.ReportSick(ctx, SickReport{EmployeeID: emp.ID, StartDate: leaveDay(2026, 6, 10), EndDate: leaveDay(2026, 6, 11)}, true, actor, company)
	if err != nil {
		t.Fatal(err)
	}
	check(5, 0)
	if _, err := absences.SetAttest(ctx, sick.ID, true, company); err != nil {
		t.Fatal(err)
	}
	check(3, 2)
	// Attest wieder entfernt: Urlaub zaehlt wieder voll.
	if _, err := absences.SetAttest(ctx, sick.ID, false, company); err != nil {
		t.Fatal(err)
	}
	check(5, 0)
	if _, err := absences.SetAttest(ctx, sick.ID, true, company); err != nil {
		t.Fatal(err)
	}
	if _, err := absences.Storno(ctx, sick.ID, "falsch", actor, company); err != nil {
		t.Fatal(err)
	}
	check(5, 0)
	// Krank Do 11.06.-Di 16.06. mit Attest: nur Do und Fr liegen im Urlaub.
	if _, err := absences.ReportSick(ctx, SickReport{EmployeeID: emp.ID, StartDate: leaveDay(2026, 6, 11), EndDate: leaveDay(2026, 6, 16), Attest: true}, false, actor, company); err != nil {
		t.Fatal(err)
	}
	check(3, 2)
	// Eine Krankmeldung mit Attest in einem offenen Antrag aendert nichts
	// am Verplanten.
	if _, err := absences.ReportSick(ctx, SickReport{EmployeeID: emp.ID, StartDate: leaveDay(2026, 7, 6), EndDate: leaveDay(2026, 7, 6), Attest: true}, false, actor, company); err != nil {
		t.Fatal(err)
	}
	check(3, 2)
}

func TestSickReturnCappedAndLegacySickRequests(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedWorkdaysCompany(t, env)
	actor := testutil.SeedAuthUser(t, env, "halb-"+uuid.NewString()[:8]+"@example.com", "Secret123!", "hr")
	emp, err := NewEmployeeService(env.PG).Create(ctx, EmployeeInput{FirstName: "Halb", LastName: "Tag"}, company)
	if err != nil {
		t.Fatal(err)
	}
	leaves := NewLeaveService(env.PG)
	absences := NewAbsenceService(env.PG)
	// Halber Urlaubstag am 22.06., krank mit Attest: zurueck kommt nur der
	// halbe Tag, nicht mehr als genommen wurde.
	lr, err := leaves.Create(ctx, LeaveRequest{EmployeeID: emp.ID, StartDate: leaveDay(2026, 6, 22), EndDate: leaveDay(2026, 6, 22), Days: 0.5}, company)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := leaves.Decide(ctx, lr.ID, true, LeaveDecisionRight{All: true}, actor, nil, company); err != nil {
		t.Fatal(err)
	}
	if _, err := absences.ReportSick(ctx, SickReport{EmployeeID: emp.ID, StartDate: leaveDay(2026, 6, 22), EndDate: leaveDay(2026, 6, 22), Attest: true}, false, actor, company); err != nil {
		t.Fatal(err)
	}
	acc, err := NewLeaveAccountService(env.PG).Get(ctx, emp.ID, 2026, company)
	if err != nil || acc.GenommenTage != 0 || acc.KrankImUrlaubTage != 0.5 || acc.RestTage != 20 {
		t.Fatalf("halber Tag: %+v, %v", acc, err)
	}

	// Alt-Bestand: ein frueher als Antrag genehmigter Krank-Antrag hat eine
	// Abwesenheit mit Antrag - sie wird ueber den Antrag storniert, nicht
	// als Krankmeldung.
	legacy := uuid.New()
	if _, err := env.PG.Exec(ctx, `INSERT INTO hr_leave_requests (id, employee_id, typ, status, start_date, end_date, days) VALUES ($1,$2,'sick','approved','2025-02-02','2025-02-03',2)`, legacy, emp.ID); err != nil {
		t.Fatal(err)
	}
	absID := uuid.New()
	if _, err := env.PG.Exec(ctx, `INSERT INTO hr_absences (id, employee_id, typ, start_date, end_date, status, source_request_id) VALUES ($1,$2,'sick','2025-02-02','2025-02-03','approved',$3)`, absID, emp.ID, legacy); err != nil {
		t.Fatal(err)
	}
	_, err = absences.Storno(ctx, absID, "x", actor, company)
	wantEmployeeError(t, err, "Ungültige Abwesenheit: keine Krankmeldung - Urlaub wird über den Urlaubsantrag storniert")
}
