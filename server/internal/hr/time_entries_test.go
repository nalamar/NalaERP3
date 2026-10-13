package hr

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Create und Stop pruefen Mandant, IDs und (Create) den Eintrag selbst,
// BEVOR sie Postgres anfassen. Das ist mit nil-Pool ueber die echten
// Einstiegspunkte testbar: kaeme einer dieser Faelle bis zum DB-Zugriff
// durch, wuerde der Test mit einer Nil-Pointer-Panic abbrechen statt mit
// einer falschen Meldung.

func TestTimeEntryCreateValidatesBeforeTouchingDatabase(t *testing.T) {
	past := time.Now().Add(-2 * time.Hour)
	beforePast := past.Add(-time.Hour)
	future := time.Now().Add(time.Hour)
	emp := uuid.New()

	cases := []struct {
		name      string
		in        TimeEntryInput
		companyID string
		wantErr   string
	}{
		{"Mandant fehlt", TimeEntryInput{EmployeeID: emp, StartedAt: past}, " ", "Mandant erforderlich"},
		{"Mitarbeiter fehlt", TimeEntryInput{StartedAt: past}, "default", "employee_id erforderlich"},
		{"Beginn fehlt", TimeEntryInput{EmployeeID: emp}, "default", "Beginn erforderlich"},
		{"Beginn in der Zukunft", TimeEntryInput{EmployeeID: emp, StartedAt: future}, "default", "Beginn darf nicht in der Zukunft liegen"},
		{"Ende vor Beginn", TimeEntryInput{EmployeeID: emp, StartedAt: past, EndedAt: &beforePast}, "default", "Ende muss nach dem Beginn liegen"},
		{"Pause an laufendem Eintrag", TimeEntryInput{EmployeeID: emp, StartedAt: past, BreakMinutes: 15}, "default", "Pause kann erst beim Beenden angegeben werden"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewTimeEntryService(nil).Create(context.Background(), tc.in, "", tc.companyID)
			if got != nil {
				t.Fatalf("erwartet keinen Eintrag, bekommen %+v", got)
			}
			if err == nil {
				t.Fatalf("erwartet Fehler %q, bekommen nil", tc.wantErr)
			}
			if err.Error() != tc.wantErr {
				t.Fatalf("erwartet %q, bekommen %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestTimeEntryStopValidatesBeforeTouchingDatabase(t *testing.T) {
	s := NewTimeEntryService(nil)

	if _, err := s.Stop(context.Background(), uuid.New(), time.Now(), 0, ""); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("erwartet 'Mandant erforderlich', bekommen %v", err)
	}
	if _, err := s.Stop(context.Background(), uuid.Nil, time.Now(), 0, "default"); err == nil || err.Error() != "Zeiteintrag-ID erforderlich" {
		t.Fatalf("erwartet 'Zeiteintrag-ID erforderlich', bekommen %v", err)
	}
}

func TestTimeEntryStornoAndCorrectValidateBeforeTouchingDatabase(t *testing.T) {
	s := NewTimeEntryService(nil)
	ctx := context.Background()
	id := uuid.New()
	past := time.Now().Add(-2 * time.Hour)
	beforePast := past.Add(-time.Hour)
	valid := TimeEntryInput{StartedAt: past}

	cases := []struct {
		name      string
		id        uuid.UUID
		reason    string
		companyID string
		wantErr   string
	}{
		{"Mandant fehlt", id, "Tippfehler", "", "Mandant erforderlich"},
		{"ID fehlt", uuid.Nil, "Tippfehler", "default", "Zeiteintrag-ID erforderlich"},
		{"Begruendung fehlt", id, "", "default", "Storno-Begründung erforderlich"},
		{"Begruendung nur Leerzeichen", id, "  \t ", "default", "Storno-Begründung erforderlich"},
	}
	for _, tc := range cases {
		t.Run("Storno/"+tc.name, func(t *testing.T) {
			_, err := s.Storno(ctx, tc.id, tc.reason, "", tc.companyID)
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("erwartet %q, bekommen %v", tc.wantErr, err)
			}
		})
		t.Run("Correct/"+tc.name, func(t *testing.T) {
			_, err := s.Correct(ctx, tc.id, valid, tc.reason, "", tc.companyID)
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("erwartet %q, bekommen %v", tc.wantErr, err)
			}
		})
	}

	// Ein ungueltiger Ersatzeintrag wird abgelehnt, bevor irgendetwas
	// storniert wird.
	_, err := s.Correct(ctx, id, TimeEntryInput{StartedAt: past, EndedAt: &beforePast}, "Tippfehler", "", "default")
	if err == nil || err.Error() != "Ende muss nach dem Beginn liegen" {
		t.Fatalf("erwartet 'Ende muss nach dem Beginn liegen', bekommen %v", err)
	}
}

func TestTimeEntryListValidatesBeforeTouchingDatabase(t *testing.T) {
	s := NewTimeEntryService(nil)
	emp := uuid.New()
	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }

	cases := []struct {
		name      string
		employee  uuid.UUID
		from, to  time.Time
		companyID string
		wantErr   string
	}{
		{"Mandant fehlt", emp, d(2026, 3, 1), d(2026, 3, 2), "", "Mandant erforderlich"},
		{"Mitarbeiter fehlt", uuid.Nil, d(2026, 3, 1), d(2026, 3, 2), "default", "employee_id erforderlich"},
		{"Von fehlt", emp, time.Time{}, d(2026, 3, 2), "default", "Zeitraum (von/bis) erforderlich"},
		{"Bis fehlt", emp, d(2026, 3, 1), time.Time{}, "default", "Zeitraum (von/bis) erforderlich"},
		{"Bis vor Von", emp, d(2026, 3, 2), d(2026, 3, 1), "default", "Bis-Datum darf nicht vor dem Von-Datum liegen"},
		// 01.01.2026 bis 02.01.2027 sind 367 Tage.
		{"367 Tage", emp, d(2026, 1, 1), d(2027, 1, 2), "default", "Zeitraum darf höchstens 366 Tage umfassen"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.List(context.Background(), tc.employee, tc.from, tc.to, false, tc.companyID)
			if got != nil {
				t.Fatalf("erwartet kein Ergebnis, bekommen %+v", got)
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("erwartet %q, bekommen %v", tc.wantErr, err)
			}
		})
	}
}

// Waechter fuer die Unveraenderbarkeit (ADR 0024). Nicht entfernen.
//
// Der Service darf genau zwei UPDATEs auf hr_time_entries enthalten - das
// einmalige Beenden und das einmalige Stornieren - und keinen DELETE. Wer
// einen dritten Schreibpfad ergaenzt, soll hier anhalten und begruenden,
// warum er die Unveraenderbarkeit nicht verletzt. Der Test prueft den
// Quelltext, weil sich "es gibt keinen weiteren Pfad" nicht durch Aufrufen
// der vorhandenen Pfade beweisen laesst.
func TestTimeEntryServiceHasNoOtherWritePaths(t *testing.T) {
	src, err := os.ReadFile("time_entries.go")
	if err != nil {
		t.Fatalf("Quelltext lesen: %v", err)
	}
	code := string(src)

	if got := strings.Count(code, "UPDATE hr_time_entries"); got != 2 {
		t.Fatalf("erwartet genau 2 UPDATEs auf hr_time_entries (Beenden, Stornieren), gefunden %d", got)
	}
	// Jeder der beiden UPDATEs muss auf seinen Ausgangszustand beschraenkt
	// sein, sonst waere der Uebergang wiederholbar.
	for _, want := range []string{
		"UPDATE hr_time_entries SET ended_at=$2, break_minutes=$3",
		"WHERE id=$1 AND ended_at IS NULL AND status='active'",
		"UPDATE hr_time_entries SET status='storniert', storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3",
		"WHERE id=$1 AND status='active'",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("erwartetes Fragment fehlt im Service: %s", want)
		}
	}
	if strings.Contains(code, "DELETE FROM") {
		t.Fatal("Zeiteintraege duerfen nicht geloescht werden")
	}
}
