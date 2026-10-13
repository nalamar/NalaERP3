package hr

import (
	"context"
	"reflect"
	"testing"
	"time"

	"nalaerp3/internal/testutil"
)

// Lesen samt Tagesauswertung (ADR 0024, Backlog F.1.4.3) gegen echtes
// Postgres. Helfer und Konventionen wie in
// time_entries_integration_test.go.

func day(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("Datum %q nicht lesbar: %v", s, err)
	}
	return d
}

func dayByDate(t *testing.T, sheet *TimeSheet, date string) DayEvaluation {
	t.Helper()
	for _, d := range sheet.Days {
		if d.WorkDate == date {
			return d
		}
	}
	t.Fatalf("Tag %s fehlt in der Auswertung: %+v", date, sheet.Days)
	return DayEvaluation{}
}

func TestTimeEntryListReturnsRangeWithDayEvaluation(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "Liste")
	colleague := seedTimeEmployee(t, env, "ListeKollege")

	// Sonntag, VOR dem Zeitraum: Spaetschicht bis 22:00.
	mustCreateClosed(t, svc, emp.ID, "2026-03-01 15:00", "2026-03-01 22:00", 0)
	// Montag: Beginn 07:00 (nur 9 h Ruhezeit), 11 h ohne Pause.
	monday := mustCreateClosed(t, svc, emp.ID, "2026-03-02 07:00", "2026-03-02 18:00", 0)
	// Dienstag: zwei Eintraege mit ausgestempelter Pause, unauffaellig.
	tueLate := mustCreateClosed(t, svc, emp.ID, "2026-03-03 12:30", "2026-03-03 16:00", 0)
	tueEarly := mustCreateClosed(t, svc, emp.ID, "2026-03-03 08:00", "2026-03-03 12:00", 0)
	// Donnerstag, NACH dem Zeitraum.
	mustCreateClosed(t, svc, emp.ID, "2026-03-05 08:00", "2026-03-05 12:00", 0)
	// Ein Kollege arbeitet im selben Zeitraum; seine Zeiten gehoeren nicht dazu.
	mustCreateClosed(t, svc, colleague.ID, "2026-03-02 06:00", "2026-03-02 14:00", 30)

	sheet, err := svc.List(ctx, emp.ID, day(t, "2026-03-02"), day(t, "2026-03-04"), false, "default")
	if err != nil {
		t.Fatalf("Liste: %v", err)
	}

	// Eintraege: nur der Zeitraum, nur dieser Mitarbeiter, nach Beginn sortiert.
	gotIDs := make([]string, 0, len(sheet.Entries))
	for _, e := range sheet.Entries {
		gotIDs = append(gotIDs, e.ID.String())
	}
	wantIDs := []string{monday.ID.String(), tueEarly.ID.String(), tueLate.ID.String()}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("erwartet Eintraege %v, bekommen %v", wantIDs, gotIDs)
	}

	// Auswertung: genau die Tage mit Eintraegen im Zeitraum. Der Sonntag
	// wurde nur fuer die Ruhezeit mitgeladen und erscheint nicht.
	if len(sheet.Days) != 2 || sheet.Days[0].WorkDate != "2026-03-02" || sheet.Days[1].WorkDate != "2026-03-03" {
		t.Fatalf("erwartet Auswertung fuer 02.03. und 03.03., bekommen %+v", sheet.Days)
	}

	mon := dayByDate(t, sheet, "2026-03-02")
	if mon.NetMinutes != 660 || mon.BreakMinutes != 0 || mon.Open {
		t.Fatalf("Montag: erwartet 660 min netto, 0 min Pause, nicht offen; bekommen %+v", mon)
	}
	// Der Ruhezeit-Hinweis beweist, dass der Vortag mitgeladen wurde.
	wantMon := []string{HinweisUeber8h, HinweisUeber10h, HinweisPauseZuKurz, HinweisRuhezeitUnterschritten}
	if !reflect.DeepEqual(mon.Hinweise, wantMon) {
		t.Fatalf("Montag: erwartet Hinweise %v, bekommen %v", wantMon, mon.Hinweise)
	}

	tue := dayByDate(t, sheet, "2026-03-03")
	if tue.NetMinutes != 450 || tue.BreakMinutes != 30 || len(tue.Hinweise) != 0 {
		t.Fatalf("Dienstag: erwartet 450 min netto, 30 min Pause, keine Hinweise; bekommen %+v", tue)
	}
}

// Beginnt der Zeitraum am ersten Arbeitstag nach einem freien Tag, darf
// kein Ruhezeit-Hinweis entstehen - und der Tag vor dem Zeitraum darf
// auch dann nicht in der Auswertung auftauchen, wenn er Eintraege hat.
func TestTimeEntryListRestPeriodAtRangeStart(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "ListeRuhe")

	mustCreateClosed(t, svc, emp.ID, "2026-03-01 15:00", "2026-03-01 22:00", 30)
	mustCreateClosed(t, svc, emp.ID, "2026-03-03 06:00", "2026-03-03 12:00", 0)

	sheet, err := svc.List(ctx, emp.ID, day(t, "2026-03-03"), day(t, "2026-03-03"), false, "default")
	if err != nil {
		t.Fatalf("Liste: %v", err)
	}
	if len(sheet.Days) != 1 || len(sheet.Days[0].Hinweise) != 0 {
		t.Fatalf("erwartet einen Tag ohne Hinweise, bekommen %+v", sheet.Days)
	}

	// Zeitraum genau ein Tag, der Vortag hat Eintraege: nur der eine Tag.
	sheet, err = svc.List(ctx, emp.ID, day(t, "2026-03-02"), day(t, "2026-03-02"), false, "default")
	if err != nil {
		t.Fatalf("Liste: %v", err)
	}
	if len(sheet.Entries) != 0 || len(sheet.Days) != 0 {
		t.Fatalf("erwartet leeres Ergebnis fuer einen Tag ohne Eintraege, bekommen %+v", sheet)
	}
	if sheet.Entries == nil || sheet.Days == nil {
		t.Fatal("erwartet leere Listen, nicht nil")
	}
}

func TestTimeEntryListStornierteOnlyOnRequestAndNeverEvaluated(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "ListeStorno")

	// Falsch erfasst: 11 h. Korrigiert auf 8 h mit Pause.
	wrong := mustCreateClosed(t, svc, emp.ID, "2026-03-02 06:00", "2026-03-02 17:00", 0)
	end := berlin(t, "2026-03-02 15:30")
	right, err := svc.Correct(ctx, wrong.ID, TimeEntryInput{StartedAt: berlin(t, "2026-03-02 07:00"), EndedAt: &end, BreakMinutes: 30}, "Zeiten vertauscht", "", "default")
	if err != nil {
		t.Fatalf("Korrektur: %v", err)
	}

	standard, err := svc.List(ctx, emp.ID, day(t, "2026-03-02"), day(t, "2026-03-02"), false, "default")
	if err != nil {
		t.Fatalf("Liste: %v", err)
	}
	if len(standard.Entries) != 1 || standard.Entries[0].ID != right.ID {
		t.Fatalf("Standardansicht: erwartet nur den gueltigen Eintrag, bekommen %+v", standard.Entries)
	}

	full, err := svc.List(ctx, emp.ID, day(t, "2026-03-02"), day(t, "2026-03-02"), true, "default")
	if err != nil {
		t.Fatalf("Liste mit Stornierten: %v", err)
	}
	if len(full.Entries) != 2 {
		t.Fatalf("erwartet Original und Korrektur, bekommen %d Eintraege", len(full.Entries))
	}
	// Die Korrekturkette ist aus der Liste allein nachvollziehbar.
	var sawOriginal, sawReplacement bool
	for _, e := range full.Entries {
		switch e.ID {
		case wrong.ID:
			sawOriginal = e.Status == TimeEntryStatusStorniert && e.StornoReason == "Zeiten vertauscht"
		case right.ID:
			sawReplacement = e.ReplacesEntryID != nil && *e.ReplacesEntryID == wrong.ID
		}
	}
	if !sawOriginal || !sawReplacement {
		t.Fatalf("Korrekturkette nicht nachvollziehbar: %+v", full.Entries)
	}

	// Die Auswertung ist in beiden Ansichten dieselbe und rechnet nur den
	// gueltigen Eintrag: 8 h, keine Hinweise. Zaehlte der stornierte mit,
	// waeren es 19 h.
	for name, sheet := range map[string]*TimeSheet{"Standard": standard, "mit Stornierten": full} {
		if len(sheet.Days) != 1 || sheet.Days[0].NetMinutes != 480 || len(sheet.Days[0].Hinweise) != 0 {
			t.Fatalf("%s: erwartet 480 min ohne Hinweise, bekommen %+v", name, sheet.Days)
		}
	}
}

func TestTimeEntryListMarksRunningDayAsOpen(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "ListeOffen")

	mustCreateClosed(t, svc, emp.ID, "2026-03-02 07:00", "2026-03-02 12:00", 0)
	if _, err := svc.Create(ctx, TimeEntryInput{EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-02 12:30")}, "", "default"); err != nil {
		t.Fatalf("Stempeln: %v", err)
	}

	sheet, err := svc.List(ctx, emp.ID, day(t, "2026-03-02"), day(t, "2026-03-02"), false, "default")
	if err != nil {
		t.Fatalf("Liste: %v", err)
	}
	if len(sheet.Entries) != 2 {
		t.Fatalf("erwartet beide Eintraege, bekommen %d", len(sheet.Entries))
	}
	if len(sheet.Days) != 1 || !sheet.Days[0].Open || sheet.Days[0].NetMinutes != 300 {
		t.Fatalf("erwartet offenen Tag mit 300 min, bekommen %+v", sheet.Days)
	}
}

func TestTimeEntryListIsScopedToCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	seedOtherCompany(t, env)
	emp := seedTimeEmployee(t, env, "ListeMandant")
	mustCreateClosed(t, svc, emp.ID, "2026-03-02 07:00", "2026-03-02 12:00", 0)

	_, err := svc.List(ctx, emp.ID, day(t, "2026-03-02"), day(t, "2026-03-02"), true, "other-company")
	wantTimeEntryError(t, err, "Mitarbeiter nicht gefunden")

	sheet, err := svc.List(ctx, emp.ID, day(t, "2026-03-02"), day(t, "2026-03-02"), true, "default")
	if err != nil {
		t.Fatalf("Liste fuer eigenen Mandanten: %v", err)
	}
	if len(sheet.Entries) != 1 {
		t.Fatalf("erwartet einen Eintrag, bekommen %d", len(sheet.Entries))
	}
}

// Die Grenzen des Zeitraums sind Arbeitstage, keine Zeitpunkte: eine
// Uhrzeit oder Zeitzone im uebergebenen Wert darf nichts verschieben.
func TestTimeEntryListUsesDateOfBoundsOnly(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "ListeGrenzen")
	mustCreateClosed(t, svc, emp.ID, "2026-03-02 07:00", "2026-03-02 08:00", 0)
	mustCreateClosed(t, svc, emp.ID, "2026-03-03 07:00", "2026-03-03 08:00", 0)

	from := time.Date(2026, 3, 2, 23, 59, 0, 0, time.UTC)
	to := time.Date(2026, 3, 3, 0, 1, 0, 0, time.UTC)
	sheet, err := svc.List(ctx, emp.ID, from, to, false, "default")
	if err != nil {
		t.Fatalf("Liste: %v", err)
	}
	if len(sheet.Entries) != 2 || len(sheet.Days) != 2 {
		t.Fatalf("erwartet beide Tage vollstaendig, bekommen %d Eintraege, %d Tage", len(sheet.Entries), len(sheet.Days))
	}

	// Genau 366 Tage (01.01.2026 bis 01.01.2027 einschliesslich) sind die
	// groesste zulaessige Spanne; 367 lehnt der DB-lose Test ab.
	if _, err := svc.List(ctx, emp.ID, day(t, "2026-01-01"), day(t, "2027-01-01"), false, "default"); err != nil {
		t.Fatalf("366 Tage muessen zulaessig sein, bekommen %v", err)
	}
}
