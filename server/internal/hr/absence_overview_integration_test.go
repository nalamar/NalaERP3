package hr

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Abwesenheitsuebersicht (Backlog F.8.4.3).

func TestAbsenceOverview(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedWorkdaysCompany(t, env)
	other := seedWorkdaysCompany(t, env)
	actor := testutil.SeedAuthUser(t, env, "uebersicht-"+uuid.NewString()[:8]+"@example.com", "Secret123!", "hr")
	montage := seedTeam(t, env, "Montage", company)
	buero := seedTeam(t, env, "Büro", company)
	emps := NewEmployeeService(env.PG)
	mk := func(last string, team *uuid.UUID, comp string) uuid.UUID {
		t.Helper()
		e, err := emps.Create(ctx, EmployeeInput{FirstName: "Ü", LastName: last, TeamID: team}, comp)
		if err != nil {
			t.Fatal(err)
		}
		return e.ID
	}
	anna := mk("Anna", &montage, company)
	bert := mk("Bert", &montage, company)
	carl := mk("Carl", nil, company)
	fremd := mk("Fremd", nil, other)
	leaves := NewLeaveService(env.PG)
	absences := NewAbsenceService(env.PG)
	all := LeaveDecisionRight{All: true}
	vacation := func(emp uuid.UUID, m1, d1, m2, d2 int) {
		t.Helper()
		lr, err := leaves.Create(ctx, LeaveRequest{EmployeeID: emp, StartDate: leaveDay(2026, time.Month(m1), d1), EndDate: leaveDay(2026, time.Month(m2), d2)}, company)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := leaves.Decide(ctx, lr.ID, true, all, actor, nil, company); err != nil {
			t.Fatal(err)
		}
	}
	sick := func(emp uuid.UUID, d1, d2 int, comp string) *Absence {
		t.Helper()
		a, err := absences.ReportSick(ctx, SickReport{EmployeeID: emp, StartDate: leaveDay(2026, 6, d1), EndDate: leaveDay(2026, 6, d2)}, false, actor, comp)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	vacation(anna, 6, 8, 6, 12)
	vacation(anna, 7, 6, 7, 7)
	sick(bert, 10, 11, company)
	storno := sick(carl, 9, 9, company)
	if _, err := absences.Storno(ctx, storno.ID, "falsch", actor, company); err != nil {
		t.Fatal(err)
	}
	sick(fremd, 10, 10, other)
	// Offene Antraege sind keine Abwesenheit.
	if _, err := leaves.Create(ctx, LeaveRequest{EmployeeID: carl, StartDate: leaveDay(2026, 6, 15), EndDate: leaveDay(2026, 6, 16)}, company); err != nil {
		t.Fatal(err)
	}

	names := func(f AbsenceFilter) string {
		t.Helper()
		list, err := absences.List(ctx, f, company)
		if err != nil {
			t.Fatalf("Liste %+v: %v", f, err)
		}
		var out []string
		for _, e := range list {
			out = append(out, e.LastName+":"+e.Typ+":"+e.StartDate.Format("01-02"))
		}
		return strings.Join(out, ",")
	}
	june := AbsenceFilter{From: leaveDay(2026, 6, 1), To: leaveDay(2026, 6, 30)}
	if got := names(june); got != "Anna:vacation:06-08,Bert:sick:06-10" {
		t.Fatalf("Juni: %s", got)
	}
	withStorno := june
	withStorno.IncludeStorniert = true
	if got := names(withStorno); got != "Anna:vacation:06-08,Carl:sick:06-09,Bert:sick:06-10" {
		t.Fatalf("Juni mit Stornierten: %s", got)
	}
	// Beruehrung an den Raendern zaehlt; danach nichts mehr.
	if got := names(AbsenceFilter{From: leaveDay(2026, 6, 12), To: leaveDay(2026, 6, 12)}); got != "Anna:vacation:06-08" {
		t.Fatalf("12.06.: %s", got)
	}
	if got := names(AbsenceFilter{From: leaveDay(2026, 6, 13), To: leaveDay(2026, 7, 5)}); got != "" {
		t.Fatalf("13.06.-05.07.: %s", got)
	}
	byEmployee := AbsenceFilter{From: leaveDay(2026, 6, 1), To: leaveDay(2026, 7, 31), EmployeeID: &anna}
	if got := names(byEmployee); got != "Anna:vacation:06-08,Anna:vacation:07-06" {
		t.Fatalf("nur Anna: %s", got)
	}
	team := june
	team.TeamID = &montage
	if got := names(team); got != "Anna:vacation:06-08,Bert:sick:06-10" {
		t.Fatalf("Team Montage: %s", got)
	}
	team.TeamID = &buero
	if got := names(team); got != "" {
		t.Fatalf("Team Büro: %s", got)
	}
	// Fremder Mandant: nur seine eigene Abwesenheit.
	if list, err := absences.List(ctx, june, other); err != nil || len(list) != 1 || list[0].LastName != "Fremd" {
		t.Fatalf("fremder Mandant: %+v, %v", list, err)
	}
	// Fehler und Grenzen.
	_, err := absences.List(ctx, AbsenceFilter{To: leaveDay(2026, 6, 1)}, company)
	wantEmployeeError(t, err, "from/to erforderlich")
	_, err = absences.List(ctx, AbsenceFilter{From: leaveDay(2026, 6, 2), To: leaveDay(2026, 6, 1)}, company)
	wantEmployeeError(t, err, "Enddatum darf nicht vor Startdatum liegen")
	_, err = absences.List(ctx, AbsenceFilter{From: leaveDay(2026, 1, 1), To: leaveDay(2027, 1, 2)}, company)
	wantEmployeeError(t, err, "Ungültiger Zeitraum: höchstens 366 Tage")
	if _, err := absences.List(ctx, AbsenceFilter{From: leaveDay(2026, 1, 1), To: leaveDay(2027, 1, 1)}, company); err != nil {
		t.Fatalf("366 Tage: %v", err)
	}
}
