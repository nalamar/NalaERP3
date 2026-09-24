package apihttp

// End-to-End-Test fuer POST /invoices-in/from-e-invoice (Backlog E.6):
// die Uebernahme einer geparsten Eingangs-E-Rechnung nach invoices_in.
//
// Der Test prueft vor allem die Eigenschaft, die diesen Endpunkt von
// einem gewoehnlichen Anlegen unterscheidet: die Rechnungsdaten kommen
// aus der DATEI, nicht vom Client.

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nalaerp3/internal/testutil"
)

// uploadEInvoiceTakeover schickt Datei plus Entscheidungsfelder an den
// Uebernahme-Endpunkt.
func uploadEInvoiceTakeover(t *testing.T, handler http.Handler, token string, content []byte, felder map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "rechnung.xml")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("Datei schreiben: %v", err)
	}
	for k, v := range felder {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatalf("Feld %s schreiben: %v", k, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Writer schließen: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/invoices-in/from-e-invoice", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

type takeoverResult struct {
	Rechnung struct {
		ID              string `json:"id"`
		LieferantID     string `json:"lieferant_id"`
		BestellungID    string `json:"bestellung_id"`
		Rechnungsnummer string `json:"rechnungsnummer"`
		Rechnungsdatum  string `json:"rechnungsdatum"`
		Waehrung        string `json:"waehrung"`
		Status          string `json:"status"`
		Notiz           string `json:"notiz"`
	} `json:"rechnung"`
	Positionen []struct {
		Bezeichnung string  `json:"bezeichnung"`
		Menge       float64 `json:"menge"`
		Preis       float64 `json:"preis"`
		Waehrung    string  `json:"waehrung"`
	} `json:"positionen"`
}

func TestTakeOverEInvoiceEndpointEndToEnd(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "einvoice-takeover@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "einvoice-takeover@example.com", "Secret123!")
	ctx := context.Background()

	if _, err := env.PG.Exec(ctx, `
        INSERT INTO contacts (id, typ, rolle, name, vat_id, company_id)
        VALUES ('c-uebernahme','org','supplier','Dichtungshandel Nord GmbH','DE811122233','default')
        ON CONFLICT (id) DO NOTHING
    `); err != nil {
		t.Fatalf("seed supplier: %v", err)
	}

	// --- Happy Path ---
	rec := uploadEInvoiceTakeover(t, handler, accessToken, []byte(inboundCIIForUpload), map[string]string{
		"lieferant_id": "c-uebernahme",
		"notiz":        "Vom Sachbearbeiter geprüft.",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("erwartet 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var got takeoverResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Antwort dekodieren: %v (Body: %s)", err, rec.Body.String())
	}

	// Kopfdaten stammen aus der DATEI, nicht vom Client.
	if got.Rechnung.Rechnungsnummer != "EIN-2026-0042" {
		t.Errorf("Rechnungsnummer = %q, erwartet aus der Datei EIN-2026-0042", got.Rechnung.Rechnungsnummer)
	}
	if got.Rechnung.LieferantID != "c-uebernahme" {
		t.Errorf("Lieferant = %q", got.Rechnung.LieferantID)
	}
	if got.Rechnung.Waehrung != "EUR" {
		t.Errorf("Währung = %q", got.Rechnung.Waehrung)
	}
	if !strings.HasPrefix(got.Rechnung.Rechnungsdatum, "2026-06-10") {
		t.Errorf("Rechnungsdatum = %q, erwartet den Wert aus der Datei", got.Rechnung.Rechnungsdatum)
	}
	if len(got.Positionen) != 1 {
		t.Fatalf("erwartet 1 Position, got %d", len(got.Positionen))
	}
	if got.Positionen[0].Bezeichnung != "Dichtungsprofil EPDM" ||
		got.Positionen[0].Menge != 50 || got.Positionen[0].Preis != 4 {
		t.Errorf("Position falsch übernommen: %+v", got.Positionen[0])
	}

	// Die Notiz haelt Herkunft und die nicht abbildbaren Angaben fest.
	for _, want := range []string{
		"Vom Sachbearbeiter geprüft.",
		"Übernommen aus E-Rechnung (CII)",
		"DE811122233",
		"brutto 238.00 EUR",
		"Steuer S 19.00%",
	} {
		if !strings.Contains(got.Rechnung.Notiz, want) {
			t.Errorf("Notiz enthält %q nicht.\nNotiz: %s", want, got.Rechnung.Notiz)
		}
	}

	// Tatsächlich persistiert?
	var count int
	if err := env.PG.QueryRow(ctx,
		`SELECT count(*) FROM invoices_in WHERE id=$1`, got.Rechnung.ID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("Eingangsrechnung wurde nicht gespeichert")
	}

	// --- Doppelerfassung wird verhindert ---
	dup := uploadEInvoiceTakeover(t, handler, accessToken, []byte(inboundCIIForUpload), map[string]string{
		"lieferant_id": "c-uebernahme",
	})
	if dup.Code != http.StatusConflict {
		t.Fatalf("erwartet 409 bei doppelter Übernahme, got %d: %s", dup.Code, dup.Body.String())
	}
	if !strings.Contains(dup.Body.String(), "bereits erfasst") {
		t.Errorf("Fehlermeldung nennt die Ursache nicht: %s", dup.Body.String())
	}

	// Es darf durch den abgelehnten zweiten Versuch nichts entstanden sein.
	var nachDup int
	if err := env.PG.QueryRow(ctx,
		`SELECT count(*) FROM invoices_in WHERE supplier_id='c-uebernahme'`).Scan(&nachDup); err != nil {
		t.Fatalf("count: %v", err)
	}
	if nachDup != 1 {
		t.Errorf("nach abgelehnter Doppelerfassung erwartet 1 Rechnung, gefunden %d", nachDup)
	}
}

func TestTakeOverEInvoiceEndpointRejectsBadInput(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "einvoice-takeover-neg@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "einvoice-takeover-neg@example.com", "Secret123!")
	ctx := context.Background()

	if _, err := env.PG.Exec(ctx, `
        INSERT INTO contacts (id, typ, rolle, name, company_id)
        VALUES ('c-uebernahme-neg','org','supplier','Negativfall GmbH','default')
        ON CONFLICT (id) DO NOTHING
    `); err != nil {
		t.Fatalf("seed supplier: %v", err)
	}

	cases := []struct {
		name     string
		content  []byte
		felder   map[string]string
		wantCode int
		wantSub  string
	}{
		{
			"ohne bestätigten Lieferanten",
			[]byte(inboundCIIForUpload),
			map[string]string{},
			http.StatusBadRequest,
			"Lieferant muss bestätigt werden",
		},
		{
			"unbekannter Lieferant",
			[]byte(inboundCIIForUpload),
			map[string]string{"lieferant_id": "gibt-es-nicht"},
			http.StatusNotFound,
			"Lieferant nicht gefunden",
		},
		{
			"unbekannte Bestellung",
			[]byte(inboundCIIForUpload),
			map[string]string{"lieferant_id": "c-uebernahme-neg", "bestellung_id": "po-gibt-es-nicht"},
			http.StatusNotFound,
			"Bestellung nicht gefunden",
		},
		{
			"keine E-Rechnung",
			[]byte("Guten Tag, anbei meine Rechnung."),
			map[string]string{"lieferant_id": "c-uebernahme-neg"},
			http.StatusBadRequest,
			"nicht gelesen werden",
		},
		{
			"Position ohne Menge",
			[]byte(strings.Replace(inboundCIIForUpload,
				`<ram:BilledQuantity unitCode="MTR">50</ram:BilledQuantity>`,
				`<ram:BilledQuantity unitCode="MTR">0</ram:BilledQuantity>`, 1)),
			map[string]string{"lieferant_id": "c-uebernahme-neg"},
			http.StatusBadRequest,
			"keine Menge größer 0",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			vorher := countRows(t, env.PG, "invoices_in")

			rec := uploadEInvoiceTakeover(t, handler, accessToken, c.content, c.felder)
			if rec.Code != c.wantCode {
				t.Fatalf("erwartet %d, got %d: %s", c.wantCode, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), c.wantSub) {
				t.Errorf("Antwort nennt %q nicht: %s", c.wantSub, rec.Body.String())
			}
			if nachher := countRows(t, env.PG, "invoices_in"); nachher != vorher {
				t.Errorf("ein abgelehnter Import darf nichts anlegen: %d -> %d", vorher, nachher)
			}
		})
	}
}

// TestTakeOverEInvoiceIgnoresClientSuppliedInvoiceData ist der Kerntest
// dieser Subtask: der Client darf die Rechnungsdaten NICHT beeinflussen.
// Er schickt hier absichtlich abweichende Werte mit - sie müssen
// wirkungslos bleiben, weil die Daten aus der Datei stammen.
func TestTakeOverEInvoiceIgnoresClientSuppliedInvoiceData(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "einvoice-takeover-trust@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "einvoice-takeover-trust@example.com", "Secret123!")
	ctx := context.Background()

	if _, err := env.PG.Exec(ctx, `
        INSERT INTO contacts (id, typ, rolle, name, company_id)
        VALUES ('c-uebernahme-trust','org','supplier','Vertrauen GmbH','default')
        ON CONFLICT (id) DO NOTHING
    `); err != nil {
		t.Fatalf("seed supplier: %v", err)
	}

	rec := uploadEInvoiceTakeover(t, handler, accessToken, []byte(inboundCIIForUpload), map[string]string{
		"lieferant_id": "c-uebernahme-trust",
		// Untergeschobene Werte - dürfen alle wirkungslos sein.
		"rechnungsnummer": "MANIPULIERT-1",
		"waehrung":        "USD",
		"menge":           "9999",
		"preis":           "0.01",
		"rechnungsdatum":  "2020-01-01T00:00:00Z",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("erwartet 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var got takeoverResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Antwort dekodieren: %v", err)
	}

	if got.Rechnung.Rechnungsnummer != "EIN-2026-0042" {
		t.Errorf("der Client konnte die Rechnungsnummer beeinflussen: %q", got.Rechnung.Rechnungsnummer)
	}
	if got.Rechnung.Waehrung != "EUR" {
		t.Errorf("der Client konnte die Währung beeinflussen: %q", got.Rechnung.Waehrung)
	}
	if !strings.HasPrefix(got.Rechnung.Rechnungsdatum, "2026-06-10") {
		t.Errorf("der Client konnte das Rechnungsdatum beeinflussen: %q", got.Rechnung.Rechnungsdatum)
	}
	if len(got.Positionen) != 1 || got.Positionen[0].Menge != 50 || got.Positionen[0].Preis != 4 {
		t.Errorf("der Client konnte die Positionen beeinflussen: %+v", got.Positionen)
	}
}
