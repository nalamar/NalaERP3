package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nalaerp3/internal/testutil"
)

// TestXRechnungEndpointEndToEnd deckt E.4.3.3 ab: das HTTP-Wiring von
// GET /invoices-out/{id}/xrechnung und den in derselben Subtask
// nachgezogenen Schreibpfad PATCH /invoices-out/{id}/buyer-reference.
//
// Die Serialisierung selbst ist in E.4.3.1 (einvoice_cii_test.go, DB-los)
// und das Mapping in E.4.3.2 (einvoice_query_integration_test.go,
// Direktaufruf) bewiesen - dieser Test beweist das Wiring: Routen,
// Statuscodes, Response-Header und die Verkettung beider Endpunkte.
func TestXRechnungEndpointEndToEnd(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "xrechnung@example.com", "Secret123!", "admin")
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "xrechnung@example.com", "Secret123!")
	ctx := context.Background()

	// Verkaeuferstammdaten: der Seed aus 025_company_profile.sql enthaelt
	// nur Name und Land, ohne Anschrift/USt-IdNr. waere keine
	// EN-16931-konforme Rechnung moeglich.
	if _, err := env.PG.Exec(ctx, `
        UPDATE company_profiles
           SET street='Werkstrasse 4', postal_code='40213', city='Duesseldorf', country='DE',
               vat_id='DE123456789', iban='DE02120300000000202051', bic='BYLADEM1001'
         WHERE id='default'
    `); err != nil {
		t.Fatalf("seed seller profile: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `
        INSERT INTO contacts (id, typ, rolle, name, email)
        VALUES ('c-xr-http','org','customer','Bauamt XRechnung','rechnung@bauamt.example')
        ON CONFLICT (id) DO NOTHING
    `); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `
        INSERT INTO contact_addresses (id, contact_id, art, zeile1, plz, ort, land, is_primary)
        VALUES ('a-xr-http','c-xr-http','billing','Rathausplatz 1','50667','Koeln','DE',true)
        ON CONFLICT (id) DO NOTHING
    `); err != nil {
		t.Fatalf("seed address: %v", err)
	}

	// Rechnung ueber die echte API anlegen.
	createBody := `{"contact_id":"c-xr-http","invoice_date":"2026-06-15T00:00:00Z","currency":"EUR",
        "items":[{"description":"Fensterelement RC2","qty":2,"unit_price":1250,"tax_code":"DE19","account_code":"8000"}]}`
	createRec := doAuthedRequest(t, handler, accessToken, http.MethodPost, "/api/v1/invoices-out/", createBody)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create invoice: expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created invoice: %v", err)
	}

	// Negativfall 1: ein draft darf nicht als E-Rechnung exportiert werden.
	draftRec := doAuthedRequest(t, handler, accessToken, http.MethodGet, "/api/v1/invoices-out/"+created.ID+"/xrechnung", "")
	if draftRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for draft invoice, got %d: %s", draftRec.Code, draftRec.Body.String())
	}

	// Buchen.
	bookRec := doAuthedRequest(t, handler, accessToken, http.MethodPost, "/api/v1/invoices-out/"+created.ID+"/book", "")
	if bookRec.Code != http.StatusOK {
		t.Fatalf("book invoice: expected 200, got %d: %s", bookRec.Code, bookRec.Body.String())
	}

	// Negativfall 2: gebucht, aber ohne Kaeuferreferenz -> 400.
	noRefRec := doAuthedRequest(t, handler, accessToken, http.MethodGet, "/api/v1/invoices-out/"+created.ID+"/xrechnung", "")
	if noRefRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without buyer reference, got %d: %s", noRefRec.Code, noRefRec.Body.String())
	}
	if !strings.Contains(noRefRec.Body.String(), "ferreferenz") {
		t.Errorf("Fehlermeldung nennt die Ursache nicht: %s", noRefRec.Body.String())
	}

	// Negativfall 3: leere Kaeuferreferenz wird abgelehnt.
	emptyRefRec := doAuthedRequest(t, handler, accessToken, http.MethodPatch,
		"/api/v1/invoices-out/"+created.ID+"/buyer-reference", `{"buyer_reference":"   "}`)
	if emptyRefRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty buyer reference, got %d: %s", emptyRefRec.Code, emptyRefRec.Body.String())
	}

	// Schreibpfad: Kaeuferreferenz NACH dem Buchen setzen (E.4.3.3).
	setRefRec := doAuthedRequest(t, handler, accessToken, http.MethodPatch,
		"/api/v1/invoices-out/"+created.ID+"/buyer-reference", `{"buyer_reference":"04011000-12345-03"}`)
	if setRefRec.Code != http.StatusOK {
		t.Fatalf("set buyer reference: expected 200, got %d: %s", setRefRec.Code, setRefRec.Body.String())
	}

	// Die Aenderung muss im Aenderungsprotokoll stehen (GoBD, Epic 0.3).
	var auditCount int
	if err := env.PG.QueryRow(ctx, `
        SELECT count(*) FROM entity_change_log
         WHERE entity_type='invoice_out' AND entity_id=$1 AND action='kaeuferreferenz_geaendert'
    `, created.ID).Scan(&auditCount); err != nil {
		t.Fatalf("query change log: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("erwartet genau einen Protokolleintrag zur Käuferreferenz, gefunden %d", auditCount)
	}

	// Happy Path: XRechnung abrufen.
	rec := doAuthedRequest(t, handler, accessToken, http.MethodGet, "/api/v1/invoices-out/"+created.ID+"/xrechnung", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("xrechnung: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/xml") {
		t.Errorf("Content-Type = %q, erwartet application/xml", ct)
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment; filename=\"xrechnung_") || !strings.HasSuffix(cd, ".xml\"") {
		t.Errorf("Content-Disposition unerwartet: %q", cd)
	}

	body := rec.Body.String()
	for _, want := range []string{
		"<rsm:CrossIndustryInvoice",
		"urn:cen.eu:en16931:2017#compliant#urn:xeinkauf.de:kosit:xrechnung_3.0",
		"<ram:BuyerReference>04011000-12345-03</ram:BuyerReference>",
		"<ram:TypeCode>380</ram:TypeCode>",
		"<ram:Name>Bauamt XRechnung</ram:Name>",
		"<ram:LineOne>Rathausplatz 1</ram:LineOne>",
		"<ram:GrandTotalAmount>2975.00</ram:GrandTotalAmount>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("XRechnung enthält %q nicht.\nBody:\n%s", want, body)
		}
	}

	// Negativfall 4: unbekannte Rechnungs-ID -> 404.
	notFoundRec := doAuthedRequest(t, handler, accessToken, http.MethodGet,
		"/api/v1/invoices-out/00000000-0000-0000-0000-00000000beef/xrechnung", "")
	if notFoundRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown invoice, got %d: %s", notFoundRec.Code, notFoundRec.Body.String())
	}

	// Negativfall 5: ungültige Rechnungs-ID -> 400.
	badIDRec := doAuthedRequest(t, handler, accessToken, http.MethodGet, "/api/v1/invoices-out/keine-uuid/xrechnung", "")
	if badIDRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed id, got %d: %s", badIDRec.Code, badIDRec.Body.String())
	}

	// Negativfall 6: nach Storno ist weder Export noch Referenzänderung
	// möglich.
	stornoRec := doAuthedRequest(t, handler, accessToken, http.MethodPost,
		"/api/v1/invoices-out/"+created.ID+"/storno", `{"reason":"Testfall"}`)
	if stornoRec.Code != http.StatusOK {
		t.Fatalf("storno: expected 200, got %d: %s", stornoRec.Code, stornoRec.Body.String())
	}
	stornoExportRec := doAuthedRequest(t, handler, accessToken, http.MethodGet, "/api/v1/invoices-out/"+created.ID+"/xrechnung", "")
	if stornoExportRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for cancelled invoice, got %d: %s", stornoExportRec.Code, stornoExportRec.Body.String())
	}
	stornoRefRec := doAuthedRequest(t, handler, accessToken, http.MethodPatch,
		"/api/v1/invoices-out/"+created.ID+"/buyer-reference", `{"buyer_reference":"NEU-123"}`)
	if stornoRefRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when changing buyer reference of cancelled invoice, got %d: %s", stornoRefRec.Code, stornoRefRec.Body.String())
	}
}

// doAuthedRequest schickt einen authentifizierten Request und liefert den
// Recorder. Body darf leer sein.
func doAuthedRequest(t *testing.T, handler http.Handler, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
