package apihttp

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Das Request-Log darf keine Werte von Query-Parametern enthalten (ADR
// 0026, Backlog 0.46): der Suchbegriff bei Kontakten ist ein Name,
// employee_id zeigt auf einen Mitarbeiter.

func TestLoggableQueryKeys(t *testing.T) {
	cases := []struct {
		name      string
		rawQuery  string
		wantKeys  string
		wantOther int
	}{
		{"leer", "", "", 0},
		{"bekannte Namen, sortiert", "to=2026-03-06&employee_id=4f1c&from=2026-03-02", "employee_id,from,to", 0},
		{"Suchbegriff: nur der Name", "q=M%C3%BCller&limit=20", "limit,q", 0},
		{"doppelter Parameter nur einmal", "status=offen&status=bezahlt", "status", 0},
		{"unbekannter Name wird gezaehlt, nicht genannt", "geheim=1&limit=5", "limit", 1},
		// "?Mueller" ist ein Parameter NAMENS Mueller ohne Wert. Darum eine
		// feste Liste und nicht "alle Namen loggen".
		{"Wert an Namensstelle", "Mueller", "", 1},
		{"mehrere unbekannte", "Erika+Musterfrau&a=b", "", 2},
		{"bekannter Name ohne Wert", "include_storniert", "include_storniert", 0},
		// Fehlerhafte Kodierung: was lesbar ist, wird ausgewertet, der Rest
		// zaehlt als ein unbekannter Parameter.
		{"fehlerhaft kodiert", "limit=5&q=%zz", "limit", 1},
		{"nur fehlerhaft", "%zz", "", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keys, other := loggableQueryKeys(tc.rawQuery)
			if keys != tc.wantKeys || other != tc.wantOther {
				t.Fatalf("erwartet (%q, %d), bekommen (%q, %d)", tc.wantKeys, tc.wantOther, keys, other)
			}
		})
	}
}

// Ende-zu-Ende durch die Middleware: was im Query-String steht, darf in
// der geschriebenen Logzeile nicht wieder auftauchen.
func TestRequestLogContainsNoQueryValues(t *testing.T) {
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})

	handler := requestLoggerMiddleware(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Der Handler selbst sieht den Query-String weiterhin vollstaendig.
		if req.URL.Query().Get("q") != "Müller" {
			t.Errorf("Handler bekommt den Suchbegriff nicht mehr: %q", req.URL.RawQuery)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	const employeeID = "4f1c2d3e-0000-4000-8000-123456789abc"
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/contacts/?q=M%C3%BCller&employee_id="+employeeID+"&from=2026-03-02&to=2026-03-06&Erika=Musterfrau", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	line := buf.String()
	if !strings.Contains(line, "http_request method=GET path=/api/v1/contacts/ ") {
		t.Fatalf("Logzeile fehlt oder hat ein unerwartetes Format: %q", line)
	}
	if !strings.Contains(line, `query_keys="employee_id,from,q,to" query_other=1 status=204`) {
		t.Fatalf("erwartet Parameternamen und Anzahl unbekannter Parameter, bekommen %q", line)
	}
	for _, secret := range []string{"Müller", "M%C3%BCller", "ller", employeeID, "4f1c2d3e", "2026-03-02", "2026-03-06", "Erika", "Musterfrau"} {
		if strings.Contains(line, secret) {
			t.Fatalf("Logzeile enthaelt den Wert %q: %q", secret, line)
		}
	}
}

func TestRequestLogWithoutQuery(t *testing.T) {
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})

	handler := requestLoggerMiddleware(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/employees/", nil))

	if line := buf.String(); !strings.Contains(line, `path=/api/v1/employees/ query_keys="" query_other=0 status=200`) {
		t.Fatalf("unerwartete Logzeile ohne Query: %q", line)
	}
}
