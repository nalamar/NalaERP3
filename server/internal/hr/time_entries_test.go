package hr

import (
	"context"
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
