package accounting

import (
	"context"
	"testing"
	"time"

	"nalaerp3/internal/testutil"
)

// JournalService hat (wie APService vor D.3.3.3) keinen direkten
// HTTP-Handler, der JournalEntryInput entgegennimmt - Rechnungen/
// Zahlungen werden ausschließlich über ARService/PaymentService gebucht,
// deren eigene Input-Structs (noch) kein kostenstelle_id-Feld haben. Die
// additive journal_lines.kostenstelle_id-Verknüpfung (E.1.3, ADR 0019)
// wird daher direkt gegen echtes Postgres getestet, analog zum bereits
// etablierten Muster in bank_integration_test.go (Backlog 0.37) und
// ap_integration_test.go (D.3.3.2).
func TestJournalCreateWithKostenstelleIDPersists(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	journalSvc := NewJournalService(env.PG)
	ccSvc := NewCostCenterService(env.PG)

	cc, err := ccSvc.Create(ctx, CostCenterCreate{Code: "WERK-JOURNAL", Name: "Werkstatt Journaltest"}, "default")
	if err != nil {
		t.Fatalf("CreateCostCenter: %v", err)
	}

	entry, err := journalSvc.Create(ctx, JournalEntryInput{
		Date:        time.Now(),
		Description: "Testbuchung mit Kostenstelle",
		Lines: []JournalLineInput{
			{AccountCode: "1600", Debit: 100, Credit: 0, KostenstelleID: &cc.ID},
			{AccountCode: "1400", Debit: 0, Credit: 100},
		},
	}, "default")
	if err != nil {
		t.Fatalf("Create journal entry: %v", err)
	}

	rows, err := env.PG.Query(ctx, `SELECT account_code, kostenstelle_id FROM journal_lines WHERE entry_id=$1 ORDER BY account_code`, entry.ID)
	if err != nil {
		t.Fatalf("query journal_lines: %v", err)
	}
	defer rows.Close()

	type persistedLine struct {
		accountCode    string
		kostenstelleID *string
	}
	var lines []persistedLine
	for rows.Next() {
		var l persistedLine
		if err := rows.Scan(&l.accountCode, &l.kostenstelleID); err != nil {
			t.Fatalf("scan journal_lines row: %v", err)
		}
		lines = append(lines, l)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 persisted lines, got %+v", lines)
	}

	// Zeilen sind nach account_code sortiert: "1400" vor "1600".
	if lines[0].accountCode != "1400" || lines[0].kostenstelleID != nil {
		t.Fatalf("expected line 1400 without kostenstelle_id, got %+v", lines[0])
	}
	if lines[1].accountCode != "1600" || lines[1].kostenstelleID == nil || *lines[1].kostenstelleID != cc.ID {
		t.Fatalf("expected line 1600 with kostenstelle_id=%q, got %+v", cc.ID, lines[1])
	}
}
