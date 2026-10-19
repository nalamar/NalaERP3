package hr

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Haertung des LeaveService vor der HTTP-Anbindung (ADR 0025, Backlog
// F.4.5). Helfer aus employee_hardening_integration_test.go und
// time_entries_integration_test.go.

func leaveDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func seedLeaveEmployee(t *testing.T, env *testutil.IntegrationEnv, lastName string) Employee {
	t.Helper()
	emp, err := NewEmployeeService(env.PG).Create(context.Background(), EmployeeInput{FirstName: "Urlaub", LastName: lastName}, "default")
	if err != nil {
		t.Fatalf("Mitarbeiter anlegen: %v", err)
	}
	return *emp
}

func mustCreateLeave(t *testing.T, svc *LeaveService, employeeID uuid.UUID, start, end time.Time) *LeaveRequest {
	t.Helper()
	lr, err := svc.Create(context.Background(), LeaveRequest{EmployeeID: employeeID, StartDate: start, EndDate: end}, "default")
	if err != nil {
		t.Fatalf("Antrag anlegen: %v", err)
	}
	return lr
}

// Befund 6 der ADR: ein entschiedener Antrag liess sich beliebig oft neu
// entscheiden, die erste Entscheidung wurde ueberschrieben.
func TestLeaveDecideIsOneTimeAndRecordsDecider(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewLeaveService(env.PG)
	emp := seedLeaveEmployee(t, env, "Einmalig")
	approverEmp := seedLeaveEmployee(t, env, "Entscheiderin")
	deciderID := testutil.SeedAuthUser(t, env, "leave-decider@example.invalid", "geheim-123456", "hr")
	otherDeciderID := testutil.SeedAuthUser(t, env, "leave-decider-2@example.invalid", "geheim-123456", "hr")

	lr := mustCreateLeave(t, svc, emp.ID, leaveDay(2027, 3, 1), leaveDay(2027, 3, 5))
	if lr.Status != "pending" || lr.DecidedByUserID != nil || lr.DecidedAt != nil {
		t.Fatalf("neuer Antrag muss offen und unentschieden sein: %+v", lr)
	}

	before := time.Now().Add(-time.Minute)
	decided, err := svc.Decide(ctx, lr.ID, true, LeaveDecisionRight{All: true}, deciderID, &approverEmp.ID, "default")
	if err != nil {
		t.Fatalf("Genehmigen: %v", err)
	}
	if decided.Status != "approved" {
		t.Fatalf("erwartet approved, bekommen %q", decided.Status)
	}
	if decided.DecidedByUserID == nil || *decided.DecidedByUserID != deciderID {
		t.Fatalf("erwartet decided_by_user_id=%s, bekommen %v", deciderID, decided.DecidedByUserID)
	}
	if decided.ApproverID == nil || *decided.ApproverID != approverEmp.ID {
		t.Fatalf("erwartet approver_id=%s, bekommen %v", approverEmp.ID, decided.ApproverID)
	}
	if decided.DecidedAt == nil || decided.DecidedAt.Before(before) {
		t.Fatalf("erwartet aktuellen Entscheidungszeitpunkt, bekommen %v", decided.DecidedAt)
	}

	// Zweiter Versuch - mit umgekehrter Entscheidung und anderem Login:
	// abgelehnt, und NICHTS an der ersten Entscheidung aendert sich.
	_, err = svc.Decide(ctx, lr.ID, false, LeaveDecisionRight{All: true}, otherDeciderID, nil, "default")
	wantEmployeeError(t, err, "Urlaubsantrag ist bereits entschieden")
	_, err = svc.Decide(ctx, lr.ID, true, LeaveDecisionRight{All: true}, otherDeciderID, nil, "default")
	wantEmployeeError(t, err, "Urlaubsantrag ist bereits entschieden")

	again, err := svc.Get(ctx, lr.ID, "default")
	if err != nil {
		t.Fatalf("Antrag laden: %v", err)
	}
	if again.Status != "approved" || again.DecidedByUserID == nil || *again.DecidedByUserID != deciderID ||
		again.ApproverID == nil || *again.ApproverID != approverEmp.ID || !again.DecidedAt.Equal(*decided.DecidedAt) {
		t.Fatalf("zweiter Entscheidungsversuch hat die erste Entscheidung veraendert: %+v", again)
	}

	// Auch eine Ablehnung ist endgueltig.
	rejected := mustCreateLeave(t, svc, emp.ID, leaveDay(2027, 4, 1), leaveDay(2027, 4, 2))
	if _, err := svc.Decide(ctx, rejected.ID, false, LeaveDecisionRight{All: true}, deciderID, nil, "default"); err != nil {
		t.Fatalf("Ablehnen: %v", err)
	}
	_, err = svc.Decide(ctx, rejected.ID, true, LeaveDecisionRight{All: true}, deciderID, nil, "default")
	wantEmployeeError(t, err, "Urlaubsantrag ist bereits entschieden")
	if got, _ := svc.Get(ctx, rejected.ID, "default"); got == nil || got.Status != "rejected" {
		t.Fatalf("abgelehnter Antrag wurde nachtraeglich genehmigt: %+v", got)
	}
}

// Befund 7 der ADR: wer entscheidet, musste selbst einen
// Mitarbeiterdatensatz haben. Jetzt genuegt der Login.
func TestLeaveDecideWorksWithoutApproverEmployee(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewLeaveService(env.PG)
	seedOtherCompany(t, env)
	emp := seedLeaveEmployee(t, env, "OhneApprover")
	deciderID := testutil.SeedAuthUser(t, env, "leave-admin@example.invalid", "geheim-123456", "admin")

	lr := mustCreateLeave(t, svc, emp.ID, leaveDay(2027, 5, 3), leaveDay(2027, 5, 7))
	decided, err := svc.Decide(ctx, lr.ID, true, LeaveDecisionRight{All: true}, deciderID, nil, "default")
	if err != nil {
		t.Fatalf("Entscheiden ohne Mitarbeiterdatensatz: %v", err)
	}
	if decided.ApproverID != nil {
		t.Fatalf("erwartet kein approver_id, bekommen %v", *decided.ApproverID)
	}
	if decided.DecidedByUserID == nil || *decided.DecidedByUserID != deciderID {
		t.Fatalf("erwartet decided_by_user_id=%s, bekommen %v", deciderID, decided.DecidedByUserID)
	}

	// Ein genannter Mitarbeiter muss aber existieren und zum Mandanten
	// gehoeren - und ein so abgelehnter Aufruf entscheidet nichts.
	pending := mustCreateLeave(t, svc, emp.ID, leaveDay(2027, 6, 1), leaveDay(2027, 6, 2))
	unknown := uuid.New()
	_, err = svc.Decide(ctx, pending.ID, true, LeaveDecisionRight{All: true}, deciderID, &unknown, "default")
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
	foreignEmp, err := NewEmployeeService(env.PG).Create(ctx, EmployeeInput{FirstName: "Fremd", LastName: "Mandant"}, "other-company")
	if err != nil {
		t.Fatalf("Mitarbeiter im zweiten Mandanten anlegen: %v", err)
	}
	_, err = svc.Decide(ctx, pending.ID, true, LeaveDecisionRight{All: true}, deciderID, &foreignEmp.ID, "default")
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
	if got, _ := svc.Get(ctx, pending.ID, "default"); got == nil || got.Status != "pending" {
		t.Fatalf("abgelehnter Aufruf hat den Antrag entschieden: %+v", got)
	}
}

func TestLeaveGetAndDecideAreScopedToCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewLeaveService(env.PG)
	seedOtherCompany(t, env)
	emp := seedLeaveEmployee(t, env, "Mandant")
	deciderID := testutil.SeedAuthUser(t, env, "leave-foreign@example.invalid", "geheim-123456", "admin")
	lr := mustCreateLeave(t, svc, emp.ID, leaveDay(2027, 7, 5), leaveDay(2027, 7, 9))

	_, err := svc.Get(ctx, lr.ID, "other-company")
	wantEmployeeError(t, err, "Urlaubsantrag nicht gefunden")
	_, err = svc.Get(ctx, uuid.New(), "default")
	wantEmployeeError(t, err, "Urlaubsantrag nicht gefunden")
	_, err = svc.Decide(ctx, lr.ID, true, LeaveDecisionRight{All: true}, deciderID, nil, "other-company")
	wantEmployeeError(t, err, "Urlaubsantrag nicht gefunden")
	_, err = svc.Decide(ctx, uuid.New(), true, LeaveDecisionRight{All: true}, deciderID, nil, "default")
	wantEmployeeError(t, err, "Urlaubsantrag nicht gefunden")

	got, err := svc.Get(ctx, lr.ID, "default")
	if err != nil {
		t.Fatalf("Antrag laden: %v", err)
	}
	if got.Status != "pending" || got.EmployeeID != emp.ID || got.Days != 5 {
		t.Fatalf("Antrag nicht wie angelegt oder aus fremdem Mandanten entschieden: %+v", got)
	}
}

func TestLeaveCreateValidatesTypeAndDays(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewLeaveService(env.PG)
	emp := seedLeaveEmployee(t, env, "TypUndTage")
	start, end := leaveDay(2027, 8, 2), leaveDay(2027, 8, 6) // 5 Kalendertage

	_, err := svc.Create(ctx, LeaveRequest{EmployeeID: emp.ID, StartDate: start, EndDate: end, Typ: "sabbatical"}, "default")
	wantEmployeeError(t, err, "Ungültiger Antragstyp")
	_, err = svc.Create(ctx, LeaveRequest{EmployeeID: emp.ID, StartDate: start, EndDate: end, Days: 5.5}, "default")
	wantEmployeeError(t, err, "Tage dürfen die Arbeitstage des Zeitraums nicht überschreiten")

	// Weniger Tage als der Zeitraum lang ist, sind zulaessig (halbe Tage).
	half, err := svc.Create(ctx, LeaveRequest{EmployeeID: emp.ID, StartDate: start, EndDate: end, Days: 4.5, Typ: "unpaid", Reason: "Umzug"}, "default")
	if err != nil {
		t.Fatalf("4,5 Tage in 5 Kalendertagen: %v", err)
	}
	got, err := svc.Get(ctx, half.ID, "default")
	if err != nil {
		t.Fatalf("Antrag laden: %v", err)
	}
	if got.Days != 4.5 || got.Typ != "unpaid" || got.Reason != "Umzug" || got.Status != "pending" {
		t.Fatalf("Antrag nicht wie angegeben gespeichert: %+v", got)
	}

	// Vom Aufrufer mitgegebene Entscheidungsangaben werden ignoriert:
	// ein neuer Antrag ist immer offen.
	sneaky, err := svc.Create(ctx, LeaveRequest{EmployeeID: emp.ID, StartDate: leaveDay(2027, 9, 1), EndDate: leaveDay(2027, 9, 1), Status: "approved", ApproverID: &emp.ID}, "default")
	if err != nil {
		t.Fatalf("Antrag anlegen: %v", err)
	}
	if got, _ := svc.Get(ctx, sneaky.ID, "default"); got == nil || got.Status != "pending" || got.ApproverID != nil || got.DecidedAt != nil {
		t.Fatalf("Antrag wurde mit mitgegebener Entscheidung angelegt: %+v", got)
	}
}

// Die Liste traegt die Entscheidungsangaben, damit sie ohne Einzelabruf
// auskommt.
func TestLeaveListCarriesDecision(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewLeaveService(env.PG)
	emp := seedLeaveEmployee(t, env, "Liste")
	deciderID := testutil.SeedAuthUser(t, env, "leave-list@example.invalid", "geheim-123456", "hr")
	open := mustCreateLeave(t, svc, emp.ID, leaveDay(2027, 10, 4), leaveDay(2027, 10, 5))
	done := mustCreateLeave(t, svc, emp.ID, leaveDay(2027, 10, 11), leaveDay(2027, 10, 12))
	if _, err := svc.Decide(ctx, done.ID, false, LeaveDecisionRight{All: true}, deciderID, nil, "default"); err != nil {
		t.Fatalf("Ablehnen: %v", err)
	}

	list, err := svc.List(ctx, &emp.ID, 50, 0, "default")
	if err != nil {
		t.Fatalf("Liste: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("erwartet 2 Antraege, bekommen %d", len(list))
	}
	for _, lr := range list {
		switch lr.ID {
		case open.ID:
			if lr.Status != "pending" || lr.DecidedByUserID != nil {
				t.Fatalf("offener Antrag traegt eine Entscheidung: %+v", lr)
			}
		case done.ID:
			if lr.Status != "rejected" || lr.DecidedByUserID == nil || *lr.DecidedByUserID != deciderID || lr.DecidedAt == nil {
				t.Fatalf("entschiedener Antrag ohne Entscheidungsangaben: %+v", lr)
			}
		default:
			t.Fatalf("fremder Antrag in der Liste: %+v", lr)
		}
	}
}
