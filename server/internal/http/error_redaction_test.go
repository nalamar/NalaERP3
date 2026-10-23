package apihttp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgconn"
)

// Rohe Datenbankfehler erreichen weder Log noch Aufrufer (Backlog 0.47,
// ADR 0030). Ohne Datenbank: der Postgres-Fehler wird gebaut, wie pgx ihn
// liefert.

// uniqueViolation: Message und Detail tragen den Wert, wie bei Postgres.
func uniqueViolation() *pgconn.PgError {
	return &pgconn.PgError{
		Severity:       "ERROR",
		Code:           "23505",
		Message:        `duplicate key value violates unique constraint "idx_hr_employees_company_personalnummer"`,
		Detail:         "Key (company_id, personalnummer)=(default, PN-4711-Mueller) already exists.",
		TableName:      "hr_employees",
		ConstraintName: "idx_hr_employees_company_personalnummer",
	}
}

// Merkmale, die nicht erscheinen duerfen: der Wert und der Postgres-Text.
var leakMarkers = []string{"4711", "Mueller", "Key (", "duplicate key", "already exists"}

func assertNoLeak(t *testing.T, where, text string) {
	t.Helper()
	for _, m := range leakMarkers {
		if strings.Contains(text, m) {
			t.Fatalf("%s enthaelt %q: %s", where, m, text)
		}
	}
}

// captureLog leitet log fuer die Dauer des Tests in einen Puffer.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return &buf
}

// serve ruft h mit Request-ID wie im Router (middleware.RequestID +
// requestContextMiddleware).
func serve(h http.HandlerFunc) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	middleware.RequestID(requestContextMiddleware(h)).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/employees/", nil))
	return rec
}

func errorBody(t *testing.T, rec *httptest.ResponseRecorder) (string, string) {
	t.Helper()
	var out struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out.Error.Code, out.Error.Message
}

func TestDescribeErrorKeepsOnlySchemaNames(t *testing.T) {
	want := "postgres code=23505 constraint=idx_hr_employees_company_personalnummer table=hr_employees column="
	for name, err := range map[string]error{
		"direkt":      uniqueViolation(),
		"umwickelt":   fmt.Errorf("Mitarbeiter speichern: %w", uniqueViolation()),
		"doppelt um.": fmt.Errorf("a: %w", fmt.Errorf("b: %w", uniqueViolation())),
	} {
		if got := describeError(err); got != want {
			t.Fatalf("%s: %q", name, got)
		}
	}
	if got := describeError(errors.New("Team nicht gefunden")); got != "Team nicht gefunden" {
		t.Fatalf("fachlicher Fehler veraendert: %q", got)
	}
	if got := describeError(nil); got != "" {
		t.Fatalf("nil: %q", got)
	}
}

// Ein Postgres-Fehler ist nie fachlich - auch nicht, wenn ein Datenwert in
// ihm wie eine fachliche Meldung klingt.
func TestClassifyDomainErrorTreatsPostgresErrorsAsInternal(t *testing.T) {
	pgErr := uniqueViolation()
	pgErr.Detail = "Key (name)=(Team nicht gefunden) already exists."
	pgErr.Message = "Feld erforderlich"
	if status, code := classifyDomainError(fmt.Errorf("x: %w", pgErr)); status != http.StatusInternalServerError || code != "internal_error" {
		t.Fatalf("erwartet 500/internal_error, bekommen %d/%s", status, code)
	}
	if status, _ := classifyDomainError(errors.New("Team nicht gefunden")); status != http.StatusNotFound {
		t.Fatalf("fachlicher Fehler: %d", status)
	}
}

func TestWriteDomainErrorRedactsPostgresErrors(t *testing.T) {
	logBuf := captureLog(t)
	rec := serve(func(w http.ResponseWriter, req *http.Request) {
		writeDomainError(w, req, fmt.Errorf("Mitarbeiter speichern: %w", uniqueViolation()))
	})
	reqID := rec.Header().Get("X-Request-ID")
	if rec.Code != http.StatusInternalServerError || reqID == "" {
		t.Fatalf("Status %d, Request-ID %q", rec.Code, reqID)
	}
	code, msg := errorBody(t, rec)
	if code != "internal_error" || msg != "Interner Fehler (Request-ID "+reqID+")" {
		t.Fatalf("Antwort: %s / %q", code, msg)
	}
	assertNoLeak(t, "Antwort", rec.Body.String())
	line := logBuf.String()
	assertNoLeak(t, "Log", line)
	for _, want := range []string{"status=500", "code=23505", "constraint=idx_hr_employees_company_personalnummer", "request_id=" + reqID} {
		if !strings.Contains(line, want) {
			t.Fatalf("Log ohne %q: %s", want, line)
		}
	}
}

// Ein nicht zugeordneter Fehler ohne Postgres: der Aufrufer bekommt den
// Text nicht, das Log behaelt ihn (Fehlersuche; ADR 0030).
func TestWriteDomainErrorHidesUnclassifiedTextFromCaller(t *testing.T) {
	logBuf := captureLog(t)
	rec := serve(func(w http.ResponseWriter, req *http.Request) {
		writeDomainError(w, req, errors.New("strconv: parsing \"Geheim\": invalid syntax"))
	})
	_, msg := errorBody(t, rec)
	if rec.Code != http.StatusInternalServerError || !strings.HasPrefix(msg, "Interner Fehler (Request-ID ") || strings.Contains(rec.Body.String(), "Geheim") {
		t.Fatalf("Antwort %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logBuf.String(), "Geheim") {
		t.Fatalf("Log ohne Fehlertext: %s", logBuf.String())
	}
}

// Fachliche Meldungen bleiben, wie sie sind.
func TestWriteDomainErrorKeepsDomainMessages(t *testing.T) {
	captureLog(t)
	rec := serve(func(w http.ResponseWriter, req *http.Request) {
		writeDomainError(w, req, errors.New("Personalnummer existiert bereits"))
	})
	code, msg := errorBody(t, rec)
	if rec.Code != http.StatusBadRequest || code != "validation_error" || msg != "Personalnummer existiert bereits" {
		t.Fatalf("Antwort %d: %s / %q", rec.Code, code, msg)
	}
}

func TestWriteHTTPErrorRedactsRawErrors(t *testing.T) {
	plain := errors.New("Lager nicht gefunden")
	cases := []struct {
		name   string
		code   int
		msg    string
		err    error
		want   string // "" = Interner Fehler mit Request-ID
		hidden string // darf nicht in der Antwort stehen
	}{
		{"Postgres bei 400", http.StatusBadRequest, uniqueViolation().Error(), uniqueViolation(), "Ungültige Eingabe", "4711"},
		{"Postgres bei 404", http.StatusNotFound, "x: " + uniqueViolation().Error(), fmt.Errorf("x: %w", uniqueViolation()), "Nicht gefunden", "duplicate"},
		{"Postgres bei 500", http.StatusInternalServerError, uniqueViolation().Error(), uniqueViolation(), "", "duplicate"},
		// Auch eine selbst formulierte Meldung weicht, wenn ein
		// Postgres-Fehler dahintersteht: sie koennte ihn enthalten.
		{"Postgres hinter eigener Meldung", http.StatusBadRequest, "Speichern fehlgeschlagen", uniqueViolation(), "Ungültige Eingabe", "Speichern"},
		{"roher Text bei 500", http.StatusInternalServerError, "open /data/Geheim.pdf: no such file", errors.New("open /data/Geheim.pdf: no such file"), "", "Geheim"},
		{"eigene Meldung bei 500 bleibt", http.StatusInternalServerError, "GridFS nicht verfügbar", errors.New("dial tcp: refused"), "GridFS nicht verfügbar", "dial"},
		{"fachlicher Text bei 404 bleibt", http.StatusNotFound, plain.Error(), plain, "Lager nicht gefunden", ""},
		{"ohne Fehler", http.StatusBadRequest, "Ungültige Eingabe", nil, "Ungültige Eingabe", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logBuf := captureLog(t)
			rec := serve(func(w http.ResponseWriter, req *http.Request) {
				writeHTTPError(w, req, tc.code, tc.msg, tc.err)
			})
			want := tc.want
			if want == "" {
				want = "Interner Fehler (Request-ID " + rec.Header().Get("X-Request-ID") + ")"
			}
			body := strings.TrimSpace(rec.Body.String())
			if rec.Code != tc.code || body != want {
				t.Fatalf("erwartet %d %q, bekommen %d %q", tc.code, want, rec.Code, body)
			}
			if tc.hidden != "" && strings.Contains(body, tc.hidden) {
				t.Fatalf("Antwort enthaelt %q: %s", tc.hidden, body)
			}
			assertNoLeak(t, "Log", logBuf.String())
		})
	}
}
