package apihttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/testutil"
)

// time_entries.read und time_entries.write sind getrennte Rechte (Backlog
// F.9). Die Systemrollen vergeben beide immer zusammen; die Tests aus
// F.1.5 konnten deshalb nicht bemerken, wenn eine Route das falsche der
// beiden verlangt. Hier zwei Rollen, die jeweils genau eines haben - und
// weder das andere noch time_entries.self.
//
// Helfer aus time_entries_integration_test.go und
// leave_requests_integration_test.go (seedRoleWithPermissions).

func TestTimeEntriesReadAndWriteAreSeparatePermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-te-readonly", "time_entries.read")
	seedRoleWithPermissions(t, env, "itest-te-writeonly", "time_entries.write")
	reader := seedTimeEntryUser(t, env, handler, "te-perm-reader@example.com", "itest-te-readonly", false)
	writer := seedTimeEntryUser(t, env, handler, "te-perm-writer@example.com", "itest-te-writeonly", false)
	worker := seedTimeEntryUser(t, env, handler, "te-perm-worker@example.com", "production", true)

	as := func(u timeEntryTestUser) func(method, path, body string) *httptest.ResponseRecorder {
		return func(method, path, body string) *httptest.ResponseRecorder {
			return doAuthedRequest(t, handler, u.Token, method, path, body)
		}
	}
	asReader, asWriter, asWorker := as(reader), as(writer), as(worker)

	const base = "/api/v1/time-entries/"
	emp := worker.EmployeeID.String()
	list := base + "?employee_id=" + emp + "&from=2026-03-02&to=2026-03-06"
	correction := `{"started_at":"2026-03-02T08:00:00+01:00","ended_at":"2026-03-02T16:00:00+01:00","break_minutes":30,"reason":"Zeiten berichtigt"}`

	// Ausgangslage, vom Mitarbeiter selbst angelegt: ein abgeschlossener
	// Eintrag am Montag, ein laufender am Dienstag.
	closed := decodeTimeEntry(t, asWorker(http.MethodPost, base, closedTimeEntryBody(nil, "2026-03-02T07:00:00+01:00", "2026-03-02T15:30:00+01:00", 30)), http.StatusCreated)
	running := decodeTimeEntry(t, asWorker(http.MethodPost, base, `{"started_at":"2026-03-03T07:00:00+01:00"}`), http.StatusCreated)

	t.Run("nur read: lesen ja, schreiben nein", func(t *testing.T) {
		got := decodeTimeEntry(t, asReader(http.MethodGet, base+closed.ID.String(), ""), http.StatusOK)
		if got.ID != closed.ID {
			t.Fatalf("Einzelabruf liefert anderen Eintrag: %+v", got)
		}
		if sheet := decodeTimeSheet(t, asReader(http.MethodGet, list, "")); len(sheet.Entries) != 2 {
			t.Fatalf("Liste: erwartet beide Eintraege des Mitarbeiters, bekommen %d", len(sheet.Entries))
		}

		for name, rec := range map[string]*httptest.ResponseRecorder{
			"anlegen":     asReader(http.MethodPost, base, closedTimeEntryBody(&worker.EmployeeID, "2026-03-04T07:00:00+01:00", "2026-03-04T15:30:00+01:00", 30)),
			"beenden":     asReader(http.MethodPost, base+running.ID.String()+"/stop", `{"ended_at":"2026-03-03T15:30:00+01:00","break_minutes":30}`),
			"stornieren":  asReader(http.MethodPost, base+closed.ID.String()+"/storno", `{"reason":"Nur Leserecht"}`),
			"korrigieren": asReader(http.MethodPost, base+closed.ID.String()+"/correct", correction),
		} {
			t.Run(name, func(t *testing.T) {
				wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
			})
		}

		// Nichts davon hat etwas veraendert oder hinzugefuegt.
		sheet := decodeTimeSheet(t, asWorker(http.MethodGet, base+"?from=2026-03-02&to=2026-03-06&include_storniert=true", ""))
		if len(sheet.Entries) != 2 {
			t.Fatalf("abgelehnte Schreibzugriffe haben die Eintraege veraendert: %d statt 2", len(sheet.Entries))
		}
		for _, e := range sheet.Entries {
			if e.Status != "active" {
				t.Fatalf("abgelehnter Storno/Korrektur hat einen Eintrag storniert: %+v", e)
			}
		}
		if still := decodeTimeEntry(t, asWorker(http.MethodGet, base+running.ID.String(), ""), http.StatusOK); still.EndedAt != nil {
			t.Fatalf("abgelehntes Beenden hat den Eintrag beendet: %+v", still)
		}
	})

	t.Run("nur write: schreiben ja, lesen nein", func(t *testing.T) {
		// Lesen - Einzelabruf und Liste - ist ohne read nicht moeglich.
		wantAPIError(t, asWriter(http.MethodGet, base+closed.ID.String(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
		wantAPIError(t, asWriter(http.MethodGet, list, ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
		// Auch nicht "die eigenen": ohne self und ohne read gibt es keine.
		wantAPIError(t, asWriter(http.MethodGet, base+"?from=2026-03-02&to=2026-03-06", ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")

		// Schreiben geht - alle vier Schreibrouten.
		stopped := decodeTimeEntry(t, asWriter(http.MethodPost, base+running.ID.String()+"/stop", `{"ended_at":"2026-03-03T15:30:00+01:00","break_minutes":30}`), http.StatusOK)
		if stopped.EndedAt == nil {
			t.Fatalf("Beenden mit write: erwartet beendeten Eintrag, bekommen %+v", stopped)
		}
		created := decodeTimeEntry(t, asWriter(http.MethodPost, base, closedTimeEntryBody(&worker.EmployeeID, "2026-03-04T07:00:00+01:00", "2026-03-04T15:30:00+01:00", 30)), http.StatusCreated)
		if created.EmployeeID != worker.EmployeeID {
			t.Fatalf("Anlegen mit write: erwartet Eintrag fuer %s, bekommen %s", worker.EmployeeID, created.EmployeeID)
		}
		replacement := decodeTimeEntry(t, asWriter(http.MethodPost, base+closed.ID.String()+"/correct", correction), http.StatusCreated)
		if replacement.ID == closed.ID || replacement.EmployeeID != worker.EmployeeID {
			t.Fatalf("Korrigieren mit write: erwartet neuen Eintrag desselben Mitarbeiters, bekommen %+v", replacement)
		}
		storniert := decodeTimeEntry(t, asWriter(http.MethodPost, base+created.ID.String()+"/storno", `{"reason":"Doppelt erfasst"}`), http.StatusOK)
		if storniert.Status != "storniert" {
			t.Fatalf("Stornieren mit write: erwartet stornierten Eintrag, bekommen %+v", storniert)
		}

		// Beobachtung, bewusst als Test festgehalten: jede Schreibroute
		// antwortet mit dem Eintrag, den sie gerade geschrieben hat. Wer
		// nur write hat, sieht also, was er selbst schreibt - das ist
		// gewollt (er muss das Ergebnis pruefen koennen). Ein Lesepfad
		// OHNE Schreibwirkung ergibt sich daraus nicht: der eben
		// korrigierte Eintrag ist fuer ihn danach nicht abrufbar.
		wantAPIError(t, asWriter(http.MethodGet, base+replacement.ID.String(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	})
}
