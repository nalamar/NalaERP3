package apihttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/charmap"

	"nalaerp3/internal/accounting"
	"nalaerp3/internal/testutil"
)

// TestDatevExportEndpointEndToEnd deckt die letzte Subtask von Task E.3 ab
// (E.3.3.3): das eigentliche HTTP-Wiring von GET /accounting/datev-export.
// Die Aggregations-/Auflösungslogik selbst ist bereits in E.3.3.1
// (datev_export_test.go, DB-los) und E.3.3.2 (datev_export_query_test.go,
// Direktaufruf von DatevExportService) bewiesen - dieser Test beweist nur
// das Wiring: Route, Permission, Konfigurationsprüfung, HTTP-Statuscodes,
// Response-Header, Windows-1252-kodierter Body.
func TestDatevExportEndpointEndToEnd(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "datev-export@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "datev-export@example.com", "Secret123!")
	ctx := context.Background()

	// Exportzeitraum bewusst auf einen einzelnen Tag eingeschränkt (nicht
	// den ganzen Monat): server/internal/accounting/datev_export_query_test.go
	// (E.3.3.2) bucht eigene Testbuchungen auf 2026-03-10/11/12/31 für
	// dieselbe Company "default" - da testutil.SetupIntegrationEnv die DB
	// NICHT zwischen separaten go-test-Prozessaufrufen zurücksetzt (siehe
	// dortige Dokumentation), würde ein breiterer Bereich bei einem
	// gemeinsamen Testlauf (z. B. `go test ./...`) Buchungen aus jenem
	// Paket mit einsammeln und die Zeilen-Index-Annahmen unten verfälschen.
	bookingDate := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	const exportRange = "von=2026-03-20&bis=2026-03-20"

	// Ohne konfigurierte Berater-/Mandantennummer muss der Export mit 400
	// abgelehnt werden (Negativfall 1) - noch bevor irgendeine Buchung
	// geladen wird.
	unconfiguredReq := httptest.NewRequest(http.MethodGet, "/api/v1/accounting/datev-export?"+exportRange, nil)
	unconfiguredReq.Header.Set("Authorization", "Bearer "+accessToken)
	unconfiguredRec := httptest.NewRecorder()
	handler.ServeHTTP(unconfiguredRec, unconfiguredReq)
	if unconfiguredRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without configured Berater-/Mandantennummer, got %d with body %s", unconfiguredRec.Code, unconfiguredRec.Body.String())
	}

	// Negativfall 2: ungültiges Datumsformat.
	badDateReq := httptest.NewRequest(http.MethodGet, "/api/v1/accounting/datev-export?von=not-a-date&bis=2026-03-20", nil)
	badDateReq.Header.Set("Authorization", "Bearer "+accessToken)
	badDateRec := httptest.NewRecorder()
	handler.ServeHTTP(badDateRec, badDateReq)
	if badDateRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid date format, got %d with body %s", badDateRec.Code, badDateRec.Body.String())
	}

	// DATEV-Einstellungen konfigurieren (PATCH /settings/company/datev).
	configReq := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/company/datev", strings.NewReader(`{
		"datev_berater_nr": 1001,
		"datev_mandant_nr": 456,
		"datev_skr": "04",
		"datev_sachkontenlaenge": 4,
		"datev_fiscal_year_start_month": 1
	}`))
	configReq.Header.Set("Authorization", "Bearer "+accessToken)
	configReq.Header.Set("Content-Type", "application/json")
	configRec := httptest.NewRecorder()
	handler.ServeHTTP(configRec, configReq)
	if configRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for DATEV settings update, got %d with body %s", configRec.Code, configRec.Body.String())
	}

	// Eine Buchung direkt über JournalService buchen (kein HTTP-Handler für
	// JournalEntryInput vorhanden, analog E.1.3/E.2.3.2/E.3.3.2).
	journalSvc := accounting.NewJournalService(env.PG)
	if _, err := journalSvc.Create(ctx, accounting.JournalEntryInput{
		Date:        bookingDate,
		Description: "Wareneingang Endpoint-Test",
		SourceID:    "RE-2026-0777",
		Lines: []accounting.JournalLineInput{
			{AccountCode: "3400", Debit: 199.99, Memo: "Profile"},
			{AccountCode: "1200", Credit: 199.99},
		},
	}, "default"); err != nil {
		t.Fatalf("book journal entry: %v", err)
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/api/v1/accounting/datev-export?"+exportRange, nil)
	exportReq.Header.Set("Authorization", "Bearer "+accessToken)
	exportRec := httptest.NewRecorder()
	handler.ServeHTTP(exportRec, exportReq)
	if exportRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for DATEV export, got %d with body %s", exportRec.Code, exportRec.Body.String())
	}
	if ct := exportRec.Header().Get("Content-Type"); ct != "text/csv; charset=windows-1252" {
		t.Fatalf("expected Content-Type text/csv; charset=windows-1252, got %q", ct)
	}
	disposition := exportRec.Header().Get("Content-Disposition")
	if !strings.Contains(disposition, "EXTF_Buchungsstapel_20260320_20260320.csv") {
		t.Fatalf("expected filename in Content-Disposition, got %q", disposition)
	}

	decoded, err := charmap.Windows1252.NewDecoder().Bytes(exportRec.Body.Bytes())
	if err != nil {
		t.Fatalf("decode windows-1252 response body: %v", err)
	}
	lines := strings.Split(string(decoded), "\r\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least header+columns+1 booking row, got %d lines: %v", len(lines), lines)
	}
	headerFields := strings.Split(lines[0], ";")
	if len(headerFields) != 31 {
		t.Fatalf("expected 31 header fields, got %d", len(headerFields))
	}
	if headerFields[10] != "1001" {
		t.Fatalf("expected Berater=1001 in header field 11, got %q", headerFields[10])
	}
	if headerFields[11] != "456" {
		t.Fatalf("expected Mandant=456 in header field 12, got %q", headerFields[11])
	}

	bookingFields := strings.Split(lines[2], ";")
	if bookingFields[0] != "199,99" {
		t.Fatalf("expected Umsatz=199,99, got %q", bookingFields[0])
	}
	if bookingFields[10] != `"RE-2026-0777"` {
		t.Fatalf("expected Belegfeld1 from source_id, got %q", bookingFields[10])
	}
}
