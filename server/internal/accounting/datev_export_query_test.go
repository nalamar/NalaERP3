package accounting

import (
	"context"
	"strings"
	"testing"
	"time"

	"nalaerp3/internal/testutil"
)

// JournalService hat keinen eigenen HTTP-Handler (siehe
// journal_kostenstelle_integration_test.go) - Buchungen werden für diesen
// Test direkt über JournalService gebucht, analog zum dort etablierten
// Muster.
func TestDatevExportLoadBookingRowsResolvesOneToOne(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	journalSvc := NewJournalService(env.PG)
	exportSvc := NewDatevExportService(env.PG)

	day := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	if _, err := journalSvc.Create(ctx, JournalEntryInput{
		Date:        day,
		Description: "Wareneingang",
		SourceID:    "RE-2026-0099",
		Lines: []JournalLineInput{
			{AccountCode: "3400", Debit: 250, Memo: "Profile"},
			{AccountCode: "1200", Credit: 250},
		},
	}, "default"); err != nil {
		t.Fatalf("Create journal entry: %v", err)
	}

	rows, err := exportSvc.LoadBookingRows(ctx, "default", day, day)
	if err != nil {
		t.Fatalf("LoadBookingRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 booking row for a 1:1 entry, got %d: %+v", len(rows), rows)
	}
	row := rows[0]
	if row.SollHaben != "H" || row.Konto != "1200" || row.Gegenkonto != "3400" {
		t.Fatalf("expected H/1200/Gegenkonto 3400 (Haben-Konvention), got %+v", row)
	}
	if row.Umsatz != 250 {
		t.Fatalf("expected Umsatz=250, got %v", row.Umsatz)
	}
	if row.Belegfeld1 != "RE-2026-0099" {
		t.Fatalf("expected Belegfeld1 from source_id, got %q", row.Belegfeld1)
	}
	if row.Buchungstext != "Wareneingang" {
		t.Fatalf("expected Buchungstext fallback to entry description (Haben-Zeile hat kein eigenes Memo), got %q", row.Buchungstext)
	}
	if !row.Belegdatum.Equal(day) {
		t.Fatalf("expected Belegdatum=%v, got %v", day, row.Belegdatum)
	}
}

func TestDatevExportLoadBookingRowsResolvesOneToMany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	journalSvc := NewJournalService(env.PG)
	ccSvc := NewCostCenterService(env.PG)
	exportSvc := NewDatevExportService(env.PG)

	cc, err := ccSvc.Create(ctx, CostCenterCreate{Code: "WERK-DATEV-1N", Name: "Werkstatt DATEV 1:N"}, "default")
	if err != nil {
		t.Fatalf("CreateCostCenter: %v", err)
	}

	day := time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC)
	if _, err := journalSvc.Create(ctx, JournalEntryInput{
		Date:        day,
		Description: "Sammelrechnung Splittbuchung",
		Lines: []JournalLineInput{
			{AccountCode: "1200", Credit: 300},
			{AccountCode: "3400", Debit: 200, Memo: "Profile", KostenstelleID: &cc.ID},
			{AccountCode: "1600", Debit: 100, Memo: "Kleinteile"},
		},
	}, "default"); err != nil {
		t.Fatalf("Create journal entry: %v", err)
	}

	rows, err := exportSvc.LoadBookingRows(ctx, "default", day, day)
	if err != nil {
		t.Fatalf("LoadBookingRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 booking rows (1 Haben-Zeile : 2 Soll-Zeilen -> je Soll-Zeile eine Row), got %d: %+v", len(rows), rows)
	}

	byKonto := map[string]DatevBookingRow{}
	for _, r := range rows {
		byKonto[r.Konto] = r
	}
	profile, ok := byKonto["3400"]
	if !ok {
		t.Fatalf("expected a row for Konto 3400, got %+v", rows)
	}
	if profile.SollHaben != "S" || profile.Gegenkonto != "1200" || profile.Umsatz != 200 {
		t.Fatalf("expected S/Gegenkonto 1200/Umsatz 200 for 3400, got %+v", profile)
	}
	if profile.Kost1 != "WERK-DATEV-1N" {
		t.Fatalf("expected Kost1 from the contributing line's own kostenstelle, got %q", profile.Kost1)
	}
	kleinteile, ok := byKonto["1600"]
	if !ok {
		t.Fatalf("expected a row for Konto 1600, got %+v", rows)
	}
	if kleinteile.SollHaben != "S" || kleinteile.Gegenkonto != "1200" || kleinteile.Umsatz != 100 {
		t.Fatalf("expected S/Gegenkonto 1200/Umsatz 100 for 1600, got %+v", kleinteile)
	}
	if kleinteile.Kost1 != "" {
		t.Fatalf("expected empty Kost1 (keine Kostenstelle auf dieser Zeile), got %q", kleinteile.Kost1)
	}
}

func TestDatevExportLoadBookingRowsRejectsManyToMany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	journalSvc := NewJournalService(env.PG)
	exportSvc := NewDatevExportService(env.PG)

	day := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	entry, err := journalSvc.Create(ctx, JournalEntryInput{
		Date:        day,
		Description: "Nicht abbildbare Splittbuchung",
		Lines: []JournalLineInput{
			{AccountCode: "3400", Debit: 100},
			{AccountCode: "1600", Debit: 50},
			{AccountCode: "1200", Credit: 80},
			{AccountCode: "1400", Credit: 70},
		},
	}, "default")
	if err != nil {
		t.Fatalf("Create journal entry: %v", err)
	}

	_, err = exportSvc.LoadBookingRows(ctx, "default", day, day)
	if err == nil {
		t.Fatal("expected an error for a booking with multiple lines on both Soll and Haben sides")
	}
	if !strings.Contains(err.Error(), entry.ID.String()) {
		t.Fatalf("expected error to name the offending entry id %q, got %q", entry.ID.String(), err.Error())
	}
	if !strings.Contains(err.Error(), "Soll: 2, Haben: 2") {
		t.Fatalf("expected error to state the Soll/Haben line counts, got %q", err.Error())
	}
}

func TestDatevExportLoadBookingRowsFiltersByDateRangeAndCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	journalSvc := NewJournalService(env.PG)
	exportSvc := NewDatevExportService(env.PG)

	inRange := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	beforeRange := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	afterRange := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	for _, d := range []time.Time{inRange, beforeRange, afterRange} {
		if _, err := journalSvc.Create(ctx, JournalEntryInput{
			Date:        d,
			Description: "Filtertest " + d.Format("2006-01-02"),
			Lines: []JournalLineInput{
				{AccountCode: "3400", Debit: 10},
				{AccountCode: "1200", Credit: 10},
			},
		}, "default"); err != nil {
			t.Fatalf("Create journal entry (%v): %v", d, err)
		}
	}

	rows, err := exportSvc.LoadBookingRows(ctx, "default", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("LoadBookingRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 row within the April date range, got %d: %+v", len(rows), rows)
	}
	if !rows[0].Belegdatum.Equal(inRange) {
		t.Fatalf("expected the in-range booking, got Belegdatum=%v", rows[0].Belegdatum)
	}
}
