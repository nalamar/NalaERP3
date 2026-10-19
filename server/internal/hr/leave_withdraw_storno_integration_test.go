package hr

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Abwesenheit aus der Genehmigung, Zurueckziehen, Storno und die eine
// Stelle fuer das Entscheidungsrecht (Backlog F.8.4.1).

type absenceRow struct {
	Typ, Status, Reason string
	Start, End          string
}

func absencesOf(t *testing.T, env *testutil.IntegrationEnv, requestID uuid.UUID) []absenceRow {
	t.Helper()
	rows, err := env.PG.Query(context.Background(), `SELECT typ, status, COALESCE(storno_reason,''), start_date::text, end_date::text FROM hr_absences WHERE source_request_id=$1 ORDER BY created_at`, requestID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []absenceRow
	for rows.Next() {
		var a absenceRow
		if err := rows.Scan(&a.Typ, &a.Status, &a.Reason, &a.Start, &a.End); err != nil {
			t.Fatal(err)
		}
		out = append(out, a)
	}
	return out
}

func TestLeaveDecideCreatesAbsenceAndChecksRight(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedWorkdaysCompany(t, env)
	decider := testutil.SeedAuthUser(t, env, "decider-"+uuid.NewString()[:8]+"@example.com", "Secret123!", "hr")
	emp, err := NewEmployeeService(env.PG).Create(ctx, EmployeeInput{FirstName: "Ab", LastName: "Wesend"}, company)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewLeaveService(env.PG)
	all := LeaveDecisionRight{All: true}

	lr, err := svc.Create(ctx, LeaveRequest{EmployeeID: emp.ID, StartDate: leaveDay(2026, 6, 8), EndDate: leaveDay(2026, 6, 12)}, company)
	if err != nil {
		t.Fatal(err)
	}
	// Ohne Recht: abgelehnt, nichts aendert sich.
	_, err = svc.Decide(ctx, lr.ID, true, LeaveDecisionRight{}, decider, nil, company)
	if !errors.Is(err, ErrNoDecisionRight) {
		t.Fatalf("ohne Recht: %v", err)
	}
	if got, _ := svc.Get(ctx, lr.ID, company); got.Status != "pending" || len(absencesOf(t, env, lr.ID)) != 0 {
		t.Fatalf("Entscheidung ohne Recht hat etwas veraendert: %+v", got)
	}
	approved, err := svc.Decide(ctx, lr.ID, true, all, decider, nil, company)
	if err != nil || approved.Status != "approved" {
		t.Fatalf("genehmigen: %+v, %v", approved, err)
	}
	abs := absencesOf(t, env, lr.ID)
	if len(abs) != 1 || abs[0] != (absenceRow{Typ: "vacation", Status: "approved", Start: "2026-06-08", End: "2026-06-12"}) {
		t.Fatalf("Abwesenheit: %+v", abs)
	}
	// Eine zweite geltende Abwesenheit zum selben Antrag verhindert die DB.
	if _, err := env.PG.Exec(ctx, `INSERT INTO hr_absences (id, employee_id, typ, start_date, end_date, status, source_request_id) VALUES ($1,$2,'vacation','2026-06-08','2026-06-12','approved',$3)`, uuid.New(), emp.ID, lr.ID); err == nil {
		t.Fatal("zweite geltende Abwesenheit angenommen")
	}

	// Ablehnung: keine Abwesenheit.
	rej, err := svc.Create(ctx, LeaveRequest{EmployeeID: emp.ID, StartDate: leaveDay(2026, 7, 6), EndDate: leaveDay(2026, 7, 7), Typ: "unpaid"}, company)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, rej.ID, false, all, decider, nil, company); err != nil {
		t.Fatal(err)
	}
	if abs := absencesOf(t, env, rej.ID); len(abs) != 0 {
		t.Fatalf("Ablehnung mit Abwesenheit: %+v", abs)
	}
	// Unbezahlter Urlaub wird ebenfalls Abwesenheit, mit seinem Typ.
	unpaid, _ := svc.Create(ctx, LeaveRequest{EmployeeID: emp.ID, StartDate: leaveDay(2026, 7, 6), EndDate: leaveDay(2026, 7, 7), Typ: "unpaid"}, company)
	if _, err := svc.Decide(ctx, unpaid.ID, true, all, decider, nil, company); err != nil {
		t.Fatal(err)
	}
	if abs := absencesOf(t, env, unpaid.ID); len(abs) != 1 || abs[0].Typ != "unpaid" {
		t.Fatalf("unbezahlt: %+v", abs)
	}
}

func TestLeaveWithdrawAndStorno(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedWorkdaysCompany(t, env)
	other := seedWorkdaysCompany(t, env)
	actor := testutil.SeedAuthUser(t, env, "storno-"+uuid.NewString()[:8]+"@example.com", "Secret123!", "hr")
	emp, err := NewEmployeeService(env.PG).Create(ctx, EmployeeInput{FirstName: "Zu", LastName: "Rueck"}, company)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewLeaveService(env.PG)
	accounts := NewLeaveAccountService(env.PG)
	all := LeaveDecisionRight{All: true}
	june := func() *LeaveRequest {
		t.Helper()
		lr, err := svc.Create(ctx, LeaveRequest{EmployeeID: emp.ID, StartDate: leaveDay(2026, 6, 8), EndDate: leaveDay(2026, 6, 12)}, company)
		if err != nil {
			t.Fatalf("Antrag Juni: %v", err)
		}
		return lr
	}

	// Zurueckziehen: offen -> withdrawn; der Zeitraum ist danach frei.
	lr := june()
	_, err = svc.Withdraw(ctx, lr.ID, other)
	wantEmployeeError(t, err, "Urlaubsantrag nicht gefunden")
	w, err := svc.Withdraw(ctx, lr.ID, company)
	if err != nil || w.Status != "withdrawn" || w.WithdrawnAt == nil {
		t.Fatalf("zurueckziehen: %+v, %v", w, err)
	}
	_, err = svc.Withdraw(ctx, lr.ID, company)
	wantEmployeeError(t, err, "Urlaubsantrag ist bereits entschieden")
	_, err = svc.Decide(ctx, lr.ID, true, all, actor, nil, company)
	wantEmployeeError(t, err, "Urlaubsantrag ist bereits entschieden")
	if acc, _ := accounts.Get(ctx, emp.ID, 2026, company); acc.VerplantTage != 0 {
		t.Fatalf("zurueckgezogen zaehlt noch als verplant: %+v", acc)
	}

	// Genehmigen, dann stornieren.
	lr = june()
	if _, err := svc.Decide(ctx, lr.ID, true, all, actor, nil, company); err != nil {
		t.Fatal(err)
	}
	if acc, _ := accounts.Get(ctx, emp.ID, 2026, company); acc.GenommenTage != 5 {
		t.Fatalf("genommen vor Storno: %+v", acc)
	}
	_, err = svc.Withdraw(ctx, lr.ID, company)
	wantEmployeeError(t, err, "Urlaubsantrag ist bereits entschieden")
	_, err = svc.Storno(ctx, lr.ID, "  ", all, actor, company)
	wantEmployeeError(t, err, "Begründung erforderlich")
	if _, err := svc.Storno(ctx, lr.ID, "Fehlbuchung", LeaveDecisionRight{}, actor, company); !errors.Is(err, ErrNoDecisionRight) {
		t.Fatalf("Storno ohne Recht: %v", err)
	}
	_, err = svc.Storno(ctx, lr.ID, "Fehlbuchung", all, actor, other)
	wantEmployeeError(t, err, "Urlaubsantrag nicht gefunden")
	if got, _ := svc.Get(ctx, lr.ID, company); got.Status != "approved" {
		t.Fatalf("abgelehnte Stornos haben etwas veraendert: %+v", got)
	}
	st, err := svc.Storno(ctx, lr.ID, " Projekt verschoben ", all, actor, company)
	if err != nil || st.Status != "storniert" || st.StornoReason != "Projekt verschoben" || st.StorniertAt == nil || st.StorniertByUserID == nil || *st.StorniertByUserID != actor {
		t.Fatalf("Storno: %+v, %v", st, err)
	}
	if abs := absencesOf(t, env, lr.ID); len(abs) != 1 || abs[0].Status != "storniert" || abs[0].Reason != "Projekt verschoben" {
		t.Fatalf("Abwesenheit nicht mitstorniert: %+v", abs)
	}
	if acc, _ := accounts.Get(ctx, emp.ID, 2026, company); acc.GenommenTage != 0 || acc.RestTage != 20 {
		t.Fatalf("Tage nicht zurueck im Konto: %+v", acc)
	}
	_, err = svc.Storno(ctx, lr.ID, "nochmal", all, actor, company)
	wantEmployeeError(t, err, "Urlaubsantrag ist bereits storniert")

	// Offene und abgelehnte Antraege werden nicht storniert.
	open := june()
	_, err = svc.Storno(ctx, open.ID, "x", all, actor, company)
	wantEmployeeError(t, err, "Urlaubsantrag ist nicht im Status genehmigt - offene Anträge werden zurückgezogen")

	// Die DB laesst weder unbekannte Status noch Storno ohne Begruendung zu.
	if _, err := env.PG.Exec(ctx, `UPDATE hr_leave_requests SET status='erledigt' WHERE id=$1`, open.ID); err == nil {
		t.Fatal("unbekannter Status angenommen")
	}
	if _, err := env.PG.Exec(ctx, `UPDATE hr_leave_requests SET status='storniert' WHERE id=$1`, open.ID); err == nil {
		t.Fatal("Storno ohne Begruendung angenommen")
	}
}
