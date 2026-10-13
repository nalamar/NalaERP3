package hr

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Stornieren und Korrigieren (ADR 0024, Backlog F.1.4.2) gegen echtes
// Postgres. Helfer und Konventionen wie in
// time_entries_integration_test.go.

func mustCreateClosed(t *testing.T, svc *TimeEntryService, employeeID uuid.UUID, start, end string, breakMinutes int) *TimeEntry {
	t.Helper()
	e := berlin(t, end)
	created, err := svc.Create(context.Background(), TimeEntryInput{EmployeeID: employeeID, StartedAt: berlin(t, start), EndedAt: &e, BreakMinutes: breakMinutes}, "", "default")
	if err != nil {
		t.Fatalf("Eintrag %s-%s anlegen: %v", start, end, err)
	}
	return created
}

// sameRecordedFacts vergleicht alles, was ein Storno NICHT anfassen darf.
func sameRecordedFacts(a, b *TimeEntry) bool {
	endEqual := (a.EndedAt == nil) == (b.EndedAt == nil) && (a.EndedAt == nil || a.EndedAt.Equal(*b.EndedAt))
	projectEqual := (a.ProjectID == nil) == (b.ProjectID == nil) && (a.ProjectID == nil || *a.ProjectID == *b.ProjectID)
	return a.ID == b.ID && a.EmployeeID == b.EmployeeID && a.WorkDate.Equal(b.WorkDate) &&
		a.StartedAt.Equal(b.StartedAt) && endEqual && a.BreakMinutes == b.BreakMinutes &&
		projectEqual && a.Note == b.Note && a.CreatedAt.Equal(b.CreatedAt)
}

func TestTimeEntryStornoKeepsOriginalAndIsOneTime(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "Storno")
	userID := testutil.SeedAuthUser(t, env, "zeit-storno@example.invalid", "geheim-123456", "hr")

	original := mustCreateClosed(t, svc, emp.ID, "2026-03-02 07:00", "2026-03-02 16:00", 45)

	before := time.Now().Add(-time.Minute)
	storniert, err := svc.Storno(ctx, original.ID, "  Falscher Tag erfasst  ", userID, "default")
	if err != nil {
		t.Fatalf("Storno: %v", err)
	}
	if storniert.Status != TimeEntryStatusStorniert {
		t.Fatalf("erwartet Status storniert, bekommen %q", storniert.Status)
	}
	if storniert.StornoReason != "Falscher Tag erfasst" {
		t.Fatalf("erwartet getrimmte Begruendung, bekommen %q", storniert.StornoReason)
	}
	if storniert.StorniertAt == nil || storniert.StorniertAt.Before(before) {
		t.Fatalf("erwartet aktuellen Storno-Zeitpunkt, bekommen %v", storniert.StorniertAt)
	}
	if storniert.StorniertByUserID == nil || *storniert.StorniertByUserID != userID {
		t.Fatalf("erwartet storniert_by_user_id=%s, bekommen %v", userID, storniert.StorniertByUserID)
	}
	// Das Original bleibt nachlesbar: der Storno schreibt nur die
	// Storno-Spalten.
	if !sameRecordedFacts(original, storniert) {
		t.Fatalf("Storno hat erfasste Angaben veraendert:\nvorher  %+v\nnachher %+v", original, storniert)
	}

	// Einmalig: ein zweiter Storno wird abgelehnt und ueberschreibt weder
	// Begruendung noch Zeitpunkt noch Urheber.
	_, err = svc.Storno(ctx, original.ID, "Zweiter Versuch", "", "default")
	wantTimeEntryError(t, err, "Zeiteintrag ist bereits storniert")
	again, err := svc.Get(ctx, original.ID, "default")
	if err != nil {
		t.Fatalf("Eintrag laden: %v", err)
	}
	if again.StornoReason != "Falscher Tag erfasst" || !again.StorniertAt.Equal(*storniert.StorniertAt) || again.StorniertByUserID == nil {
		t.Fatalf("zweiter Storno hat den ersten ueberschrieben: %+v", again)
	}

	// Ein stornierter Eintrag ist auch nicht mehr korrigierbar.
	end := berlin(t, "2026-03-02 15:00")
	_, err = svc.Correct(ctx, original.ID, TimeEntryInput{StartedAt: berlin(t, "2026-03-02 07:00"), EndedAt: &end}, "Nachkorrektur", "", "default")
	wantTimeEntryError(t, err, "Zeiteintrag ist bereits storniert")
}

func TestTimeEntryStornoOfRunningEntryAllowsNewStart(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "Fehlstempel")

	running, err := svc.Create(ctx, TimeEntryInput{EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-02 07:00")}, "", "default")
	if err != nil {
		t.Fatalf("Stempeln: %v", err)
	}
	storniert, err := svc.Storno(ctx, running.ID, "Versehentlich gestempelt", "", "default")
	if err != nil {
		t.Fatalf("laufenden Eintrag stornieren: %v", err)
	}
	if storniert.EndedAt != nil {
		t.Fatalf("Storno darf kein Ende erfinden, bekommen %v", storniert.EndedAt)
	}
	if storniert.StorniertByUserID != nil {
		t.Fatalf("leerer Akteur muss NULL ergeben, bekommen %v", *storniert.StorniertByUserID)
	}
	if _, err := svc.Create(ctx, TimeEntryInput{EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-02 07:30")}, "", "default"); err != nil {
		t.Fatalf("nach dem Storno muss wieder gestempelt werden koennen, bekommen %v", err)
	}
}

func TestTimeEntryCorrectReplacesEntryAndLinksIt(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "Korrektur")
	userID := testutil.SeedAuthUser(t, env, "zeit-korrektur@example.invalid", "geheim-123456", "hr")

	original := mustCreateClosed(t, svc, emp.ID, "2026-03-02 07:00", "2026-03-02 16:00", 30)

	// Der Ersatz ueberschneidet sich mit dem Original (gleicher Tag,
	// anderes Ende). Das muss gehen: das Original ist in derselben
	// Transaktion bereits storniert.
	newEnd := berlin(t, "2026-03-02 17:30")
	replacement, err := svc.Correct(ctx, original.ID, TimeEntryInput{StartedAt: berlin(t, "2026-03-02 07:00"), EndedAt: &newEnd, BreakMinutes: 45, Note: "Ende berichtigt"}, "Ende falsch erfasst", userID, "default")
	if err != nil {
		t.Fatalf("Korrektur: %v", err)
	}
	if replacement.ID == original.ID {
		t.Fatal("die Korrektur muss einen NEUEN Eintrag liefern")
	}
	if replacement.ReplacesEntryID == nil || *replacement.ReplacesEntryID != original.ID {
		t.Fatalf("erwartet replaces_entry_id=%s, bekommen %v", original.ID, replacement.ReplacesEntryID)
	}
	if replacement.EmployeeID != emp.ID || replacement.Status != TimeEntryStatusActive {
		t.Fatalf("Ersatzeintrag: falscher Mitarbeiter oder Status: %+v", replacement)
	}
	if !replacement.EndedAt.Equal(newEnd) || replacement.BreakMinutes != 45 || replacement.Note != "Ende berichtigt" {
		t.Fatalf("Ersatzeintrag traegt nicht die berichtigten Angaben: %+v", replacement)
	}
	if replacement.CreatedByUserID == nil || *replacement.CreatedByUserID != userID {
		t.Fatalf("erwartet created_by_user_id=%s am Ersatz, bekommen %v", userID, replacement.CreatedByUserID)
	}

	old, err := svc.Get(ctx, original.ID, "default")
	if err != nil {
		t.Fatalf("Original laden: %v", err)
	}
	if old.Status != TimeEntryStatusStorniert || old.StornoReason != "Ende falsch erfasst" {
		t.Fatalf("Original muss mit der Korrekturbegruendung storniert sein: %+v", old)
	}
	if old.StorniertByUserID == nil || *old.StorniertByUserID != userID {
		t.Fatalf("erwartet storniert_by_user_id=%s am Original, bekommen %v", userID, old.StorniertByUserID)
	}
	if !sameRecordedFacts(original, old) {
		t.Fatalf("Korrektur hat die Angaben des Originals veraendert:\nvorher  %+v\nnachher %+v", original, old)
	}

	// Korrektur der Korrektur: die Kette waechst um ein Glied.
	secondEnd := berlin(t, "2026-03-02 17:00")
	second, err := svc.Correct(ctx, replacement.ID, TimeEntryInput{EmployeeID: emp.ID, StartedAt: berlin(t, "2026-03-02 07:00"), EndedAt: &secondEnd, BreakMinutes: 45}, "Doch 17 Uhr", userID, "default")
	if err != nil {
		t.Fatalf("zweite Korrektur: %v", err)
	}
	if second.ReplacesEntryID == nil || *second.ReplacesEntryID != replacement.ID {
		t.Fatalf("erwartet Verweis auf den ersten Ersatz, bekommen %v", second.ReplacesEntryID)
	}
	var active, total int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='active'), count(*) FROM hr_time_entries WHERE employee_id=$1`, emp.ID).Scan(&active, &total); err != nil {
		t.Fatalf("Eintraege zaehlen: %v", err)
	}
	if active != 1 || total != 3 {
		t.Fatalf("erwartet 1 aktiven von 3 Eintraegen (Original, 1. und 2. Korrektur), bekommen %d von %d", active, total)
	}
}

// Kern von F.1.4.2: Storno und Ersatz sind EINE Transaktion. Scheitert der
// Ersatz, darf der Storno nicht stehen bleiben - sonst waere die erfasste
// Arbeitszeit ersatzlos verschwunden.
func TestTimeEntryCorrectIsAtomic(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	seedOtherCompany(t, env)
	emp := seedTimeEmployee(t, env, "Atomar")
	otherEmp := seedTimeEmployee(t, env, "AtomarKollege")

	original := mustCreateClosed(t, svc, emp.ID, "2026-03-02 07:00", "2026-03-02 12:00", 0)
	mustCreateClosed(t, svc, emp.ID, "2026-03-02 13:00", "2026-03-02 17:00", 0)

	assertUntouched := func(t *testing.T) {
		t.Helper()
		got, err := svc.Get(ctx, original.ID, "default")
		if err != nil {
			t.Fatalf("Original laden: %v", err)
		}
		if got.Status != TimeEntryStatusActive || got.StorniertAt != nil || got.StornoReason != "" || got.StorniertByUserID != nil {
			t.Fatalf("gescheiterte Korrektur hat einen Storno hinterlassen: %+v", got)
		}
		if !sameRecordedFacts(original, got) {
			t.Fatalf("gescheiterte Korrektur hat das Original veraendert: %+v", got)
		}
		var count int
		if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM hr_time_entries WHERE employee_id=$1`, emp.ID).Scan(&count); err != nil {
			t.Fatalf("Eintraege zaehlen: %v", err)
		}
		if count != 2 {
			t.Fatalf("erwartet weiterhin genau 2 Eintraege, bekommen %d", count)
		}
	}

	t.Run("Ersatz ueberschneidet sich mit anderem Eintrag", func(t *testing.T) {
		end := berlin(t, "2026-03-02 14:00")
		_, err := svc.Correct(ctx, original.ID, TimeEntryInput{StartedAt: berlin(t, "2026-03-02 07:00"), EndedAt: &end}, "Ende berichtigt", "", "default")
		wantTimeEntryError(t, err, "Zeitraum überschneidet sich mit einem bestehenden Zeiteintrag")
		assertUntouched(t)
	})

	t.Run("Ersatz mit unbekanntem Projekt", func(t *testing.T) {
		end := berlin(t, "2026-03-02 12:00")
		unknown := uuid.New()
		_, err := svc.Correct(ctx, original.ID, TimeEntryInput{StartedAt: berlin(t, "2026-03-02 07:00"), EndedAt: &end, ProjectID: &unknown}, "Projekt ergänzt", "", "default")
		wantTimeEntryError(t, err, "Projekt nicht gefunden")
		assertUntouched(t)
	})

	t.Run("Mitarbeiter ist nicht korrigierbar", func(t *testing.T) {
		end := berlin(t, "2026-03-02 12:00")
		_, err := svc.Correct(ctx, original.ID, TimeEntryInput{EmployeeID: otherEmp.ID, StartedAt: berlin(t, "2026-03-02 07:00"), EndedAt: &end}, "Falscher Mitarbeiter", "", "default")
		wantTimeEntryError(t, err, "Korrektur kann den Mitarbeiter nicht ändern")
		assertUntouched(t)
	})

	t.Run("fremder Mandant", func(t *testing.T) {
		end := berlin(t, "2026-03-02 12:00")
		_, err := svc.Correct(ctx, original.ID, TimeEntryInput{StartedAt: berlin(t, "2026-03-02 07:00"), EndedAt: &end}, "Fremdzugriff", "", "other-company")
		wantTimeEntryError(t, err, "Zeiteintrag nicht gefunden")
		_, err = svc.Storno(ctx, original.ID, "Fremdzugriff", "", "other-company")
		wantTimeEntryError(t, err, "Zeiteintrag nicht gefunden")
		assertUntouched(t)
	})
}

// Ein abgeschlossener Eintrag wird ueber keinen Pfad des Service
// veraendert: Beenden ist abgelehnt, Storno und Korrektur lassen die
// erfassten Angaben stehen.
func TestTimeEntryClosedEntryFactsNeverChange(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewTimeEntryService(env.PG)
	emp := seedTimeEmployee(t, env, "Unveraendert")

	original := mustCreateClosed(t, svc, emp.ID, "2026-03-02 07:00", "2026-03-02 16:00", 45)

	_, err := svc.Stop(ctx, original.ID, berlin(t, "2026-03-02 18:00"), 0, "default")
	wantTimeEntryError(t, err, "Zeiteintrag ist bereits beendet")

	end := berlin(t, "2026-03-02 15:00")
	if _, err := svc.Correct(ctx, original.ID, TimeEntryInput{StartedAt: berlin(t, "2026-03-02 08:00"), EndedAt: &end}, "Zeiten berichtigt", "", "default"); err != nil {
		t.Fatalf("Korrektur: %v", err)
	}

	_, err = svc.Stop(ctx, original.ID, berlin(t, "2026-03-02 18:00"), 0, "default")
	wantTimeEntryError(t, err, "Zeiteintrag ist storniert")

	got, err := svc.Get(ctx, original.ID, "default")
	if err != nil {
		t.Fatalf("Original laden: %v", err)
	}
	if !sameRecordedFacts(original, got) {
		t.Fatalf("erfasste Angaben des Originals haben sich geaendert:\nvorher  %+v\nnachher %+v", original, got)
	}
}
