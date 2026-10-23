package apihttp

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"nalaerp3/internal/testutil"
)

// Mit echtem Postgres (Backlog 0.47): der Fehler kommt so, wie pgx ihn
// liefert - Postgres wiederholt den Wert in der Meldung ("invalid input
// syntax for type uuid: \"...\""). Er geht durch beide Fehlerwege; weder
// Antwort noch Fehlertext im Log enthalten den Wert.
func TestRealPostgresErrorReachesNeitherCallerNorLog(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	var dummy string
	dbErr := env.PG.QueryRow(context.Background(), `SELECT $1::uuid::text`, "Mueller4711").Scan(&dummy)
	if dbErr == nil || pgErrorOf(dbErr) == nil || !strings.Contains(dbErr.Error(), "Mueller4711") {
		t.Fatalf("erwartet Postgres-Fehler mit dem Wert im Text, bekommen %v", dbErr)
	}

	for name, write := range map[string]func(http.ResponseWriter, *http.Request){
		"writeDomainError": func(w http.ResponseWriter, req *http.Request) { writeDomainError(w, req, dbErr) },
		"writeHTTPError": func(w http.ResponseWriter, req *http.Request) {
			writeHTTPError(w, req, http.StatusBadRequest, dbErr.Error(), dbErr)
		},
	} {
		t.Run(name, func(t *testing.T) {
			logBuf := captureLog(t)
			rec := serve(write)
			body := rec.Body.String()
			line := logBuf.String()
			for _, leak := range []string{"Mueller4711", "invalid input syntax"} {
				if strings.Contains(body, leak) || strings.Contains(line, leak) {
					t.Fatalf("%q in Antwort oder Log: %s | %s", leak, body, line)
				}
			}
			if !strings.Contains(line, "code=22P02") || !strings.Contains(line, "request_id="+rec.Header().Get("X-Request-ID")) {
				t.Fatalf("Log ohne SQLSTATE oder Request-ID: %s", line)
			}
		})
	}
}
