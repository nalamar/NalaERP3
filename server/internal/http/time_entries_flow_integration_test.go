package apihttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Stornieren, Korrigieren und Liste ueber HTTP (Backlog F.1.5.2). Helfer
// aus time_entries_integration_test.go.

type timeSheetResponse struct {
	Entries []struct {
		ID              uuid.UUID  `json:"id"`
		Status          string     `json:"status"`
		StornoReason    string     `json:"storno_reason"`
		ReplacesEntryID *uuid.UUID `json:"replaces_entry_id"`
	} `json:"entries"`
	Days []struct {
		WorkDate     string   `json:"work_date"`
		NetMinutes   int      `json:"net_minutes"`
		BreakMinutes int      `json:"break_minutes"`
		Open         bool     `json:"open"`
		Hinweise     []string `json:"hinweise"`
	} `json:"days"`
}

func decodeTimeSheet(t *testing.T, rec *httptest.ResponseRecorder) timeSheetResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("erwartet 200, bekommen %d mit Body %s", rec.Code, rec.Body.String())
	}
	var out timeSheetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

// Der ganze Ablauf aus Sicht eines Mitarbeiters, nur mit
// time_entries.self und ohne je eine employee_id zu nennen:
// stempeln, beenden, feststellen, dass die Zeit falsch war, korrigieren,
// die Woche ansehen.
func TestTimeEntriesEndToEndFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	worker := seedTimeEntryUser(t, env, handler, "zeit-flow@example.com", "production", true)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, worker.Token, method, path, body)
	}

	// Sonntag: Spaetschicht bis 22:00 (liegt vor dem abgefragten Zeitraum).
	decodeTimeEntry(t, do(http.MethodPost, "/api/v1/time-entries/", closedTimeEntryBody(nil, "2026-03-01T15:00:00+01:00", "2026-03-01T22:00:00+01:00", 30)), http.StatusCreated)

	// Montag: gestempelt um 07:00, beendet um 18:00 ohne Pause.
	monday := decodeTimeEntry(t, do(http.MethodPost, "/api/v1/time-entries/", `{"started_at":"2026-03-02T07:00:00+01:00"}`), http.StatusCreated)
	decodeTimeEntry(t, do(http.MethodPost, "/api/v1/time-entries/"+monday.ID.String()+"/stop", `{"ended_at":"2026-03-02T18:00:00+01:00"}`), http.StatusOK)

	// Die Auswertung zeigt den Verstoss - gespeichert wurde er trotzdem.
	sheet := decodeTimeSheet(t, do(http.MethodGet, "/api/v1/time-entries/?from=2026-03-02&to=2026-03-06", ""))
	if len(sheet.Entries) != 1 || len(sheet.Days) != 1 {
		t.Fatalf("erwartet einen Eintrag und einen Tag, bekommen %+v", sheet)
	}
	wantHints := []string{"ueber_8h", "ueber_10h", "pause_zu_kurz", "ruhezeit_unterschritten"}
	if sheet.Days[0].WorkDate != "2026-03-02" || sheet.Days[0].NetMinutes != 660 || !reflect.DeepEqual(sheet.Days[0].Hinweise, wantHints) {
		t.Fatalf("Montag vor Korrektur: erwartet 660 min mit %v, bekommen %+v", wantHints, sheet.Days[0])
	}

	// Tatsaechlich war es 09:00 bis 17:45 mit 45 Minuten Pause.
	// Korrektur ohne Begruendung: abgelehnt, nichts passiert.
	wantAPIError(t, do(http.MethodPost, "/api/v1/time-entries/"+monday.ID.String()+"/correct",
		`{"started_at":"2026-03-02T09:00:00+01:00","ended_at":"2026-03-02T17:45:00+01:00","break_minutes":45}`),
		http.StatusBadRequest, "validation_error", "Storno-Begründung erforderlich")

	corrected := decodeTimeEntry(t, do(http.MethodPost, "/api/v1/time-entries/"+monday.ID.String()+"/correct",
		`{"started_at":"2026-03-02T09:00:00+01:00","ended_at":"2026-03-02T17:45:00+01:00","break_minutes":45,"reason":"Falsche Uhrzeiten eingetragen"}`), http.StatusCreated)
	if corrected.ID == monday.ID || corrected.EmployeeID != worker.EmployeeID || corrected.BreakMinutes != 45 {
		t.Fatalf("Korrektur muss einen neuen Eintrag fuer denselben Mitarbeiter liefern: %+v", corrected)
	}

	// Standardansicht: nur der gueltige Eintrag, Auswertung jetzt unauffaellig
	// (11 h Ruhezeit seit Sonntag 22:00, 8 h Arbeit, 45 min Pause).
	sheet = decodeTimeSheet(t, do(http.MethodGet, "/api/v1/time-entries/?from=2026-03-02&to=2026-03-06", ""))
	if len(sheet.Entries) != 1 || sheet.Entries[0].ID != corrected.ID {
		t.Fatalf("Standardansicht: erwartet nur den korrigierten Eintrag, bekommen %+v", sheet.Entries)
	}
	if len(sheet.Days) != 1 || sheet.Days[0].NetMinutes != 480 || sheet.Days[0].BreakMinutes != 45 || len(sheet.Days[0].Hinweise) != 0 {
		t.Fatalf("Montag nach Korrektur: erwartet 480 min, 45 min Pause, keine Hinweise; bekommen %+v", sheet.Days)
	}

	// Mit include_storniert ist die Korrekturkette nachlesbar.
	full := decodeTimeSheet(t, do(http.MethodGet, "/api/v1/time-entries/?from=2026-03-02&to=2026-03-06&include_storniert=true", ""))
	if len(full.Entries) != 2 {
		t.Fatalf("erwartet Original und Korrektur, bekommen %d Eintraege", len(full.Entries))
	}
	var chainOK int
	for _, e := range full.Entries {
		if e.ID == monday.ID && e.Status == "storniert" && e.StornoReason == "Falsche Uhrzeiten eingetragen" {
			chainOK++
		}
		if e.ID == corrected.ID && e.Status == "active" && e.ReplacesEntryID != nil && *e.ReplacesEntryID == monday.ID {
			chainOK++
		}
	}
	if chainOK != 2 {
		t.Fatalf("Korrekturkette nicht nachvollziehbar: %+v", full.Entries)
	}
	if len(full.Days) != 1 || full.Days[0].NetMinutes != 480 {
		t.Fatalf("Auswertung darf den stornierten Eintrag nicht mitrechnen: %+v", full.Days)
	}

	// Dienstag: versehentlich gestempelt, ersatzlos storniert.
	stray := decodeTimeEntry(t, do(http.MethodPost, "/api/v1/time-entries/", `{"started_at":"2026-03-03T06:00:00+01:00"}`), http.StatusCreated)
	wantAPIError(t, do(http.MethodPost, "/api/v1/time-entries/"+stray.ID.String()+"/storno", `{"reason":"  "}`),
		http.StatusBadRequest, "validation_error", "Storno-Begründung erforderlich")
	storniert := decodeTimeEntry(t, do(http.MethodPost, "/api/v1/time-entries/"+stray.ID.String()+"/storno", `{"reason":"Versehentlich gestempelt"}`), http.StatusOK)
	if storniert.Status != "storniert" || storniert.EndedAt != nil {
		t.Fatalf("erwartet stornierten Eintrag ohne erfundenes Ende, bekommen %+v", storniert)
	}
	if storniert.StorniertByUserID == nil || *storniert.StorniertByUserID != worker.UserID {
		t.Fatalf("erwartet storniert_by_user_id=%s, bekommen %v", worker.UserID, storniert.StorniertByUserID)
	}
	wantAPIError(t, do(http.MethodPost, "/api/v1/time-entries/"+stray.ID.String()+"/storno", `{"reason":"Noch einmal"}`),
		http.StatusConflict, "conflict", "Zeiteintrag ist bereits storniert")
	wantAPIError(t, do(http.MethodPost, "/api/v1/time-entries/"+stray.ID.String()+"/stop", `{}`),
		http.StatusConflict, "conflict", "Zeiteintrag ist storniert")

	// Der stornierte Dienstag erscheint in der Auswertung gar nicht.
	sheet = decodeTimeSheet(t, do(http.MethodGet, "/api/v1/time-entries/?from=2026-03-02&to=2026-03-06", ""))
	if len(sheet.Entries) != 1 || len(sheet.Days) != 1 {
		t.Fatalf("stornierter Tag darf nicht erscheinen, bekommen %+v", sheet)
	}

	// Eine Korrektur, deren Ersatz scheitert, laesst den Eintrag aktiv
	// (Atomaritaet, hier ueber HTTP).
	wantAPIError(t, do(http.MethodPost, "/api/v1/time-entries/"+corrected.ID.String()+"/correct",
		`{"started_at":"2026-03-01T21:00:00+01:00","ended_at":"2026-03-02T05:00:00+01:00","reason":"Verschoben"}`),
		http.StatusConflict, "conflict", "Zeitraum überschneidet sich mit einem bestehenden Zeiteintrag")
	still := decodeTimeEntry(t, do(http.MethodGet, "/api/v1/time-entries/"+corrected.ID.String(), ""), http.StatusOK)
	if still.Status != "active" {
		t.Fatalf("gescheiterte Korrektur hat den Eintrag storniert: %+v", still)
	}
}

func TestTimeEntriesListAndCorrectionPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	hrUser := seedTimeEntryUser(t, env, handler, "zeit-perm-hr@example.com", "hr", false)
	worker := seedTimeEntryUser(t, env, handler, "zeit-perm-a@example.com", "production", true)
	colleague := seedTimeEntryUser(t, env, handler, "zeit-perm-b@example.com", "production", true)

	theirs := decodeTimeEntry(t, doAuthedRequest(t, handler, colleague.Token, http.MethodPost, "/api/v1/time-entries/",
		closedTimeEntryBody(nil, "2026-03-02T07:00:00+01:00", "2026-03-02T15:30:00+01:00", 30)), http.StatusCreated)
	list := "/api/v1/time-entries/?from=2026-03-02&to=2026-03-02"

	// self: fremde Liste, fremder Storno, fremde Korrektur - alles 403.
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, list+"&employee_id="+colleague.EmployeeID.String(), ""),
		http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, "/api/v1/time-entries/"+theirs.ID.String()+"/storno", `{"reason":"Fremdzugriff"}`),
		http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, "/api/v1/time-entries/"+theirs.ID.String()+"/correct",
		`{"started_at":"2026-03-02T08:00:00+01:00","ended_at":"2026-03-02T15:30:00+01:00","reason":"Fremdzugriff"}`),
		http.StatusForbidden, "forbidden", "Keine Berechtigung")
	// Die eigene Liste des Fremden ist leer - er sieht den Eintrag auch so nicht.
	if own := decodeTimeSheet(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, list, "")); len(own.Entries) != 0 {
		t.Fatalf("eigene Liste enthaelt fremde Eintraege: %+v", own.Entries)
	}
	untouched := decodeTimeEntry(t, doAuthedRequest(t, handler, colleague.Token, http.MethodGet, "/api/v1/time-entries/"+theirs.ID.String(), ""), http.StatusOK)
	if untouched.Status != "active" {
		t.Fatalf("abgelehnter Fremdzugriff hat den Eintrag veraendert: %+v", untouched)
	}

	// Personalabteilung: liest die Liste des Mitarbeiters und korrigiert.
	sheet := decodeTimeSheet(t, doAuthedRequest(t, handler, hrUser.Token, http.MethodGet, list+"&employee_id="+colleague.EmployeeID.String(), ""))
	if len(sheet.Entries) != 1 || sheet.Entries[0].ID != theirs.ID {
		t.Fatalf("Personalabteilung: erwartet den Eintrag des Mitarbeiters, bekommen %+v", sheet.Entries)
	}
	fixed := decodeTimeEntry(t, doAuthedRequest(t, handler, hrUser.Token, http.MethodPost, "/api/v1/time-entries/"+theirs.ID.String()+"/correct",
		`{"started_at":"2026-03-02T07:30:00+01:00","ended_at":"2026-03-02T15:30:00+01:00","break_minutes":30,"reason":"Beginn laut Schichtplan"}`), http.StatusCreated)
	if fixed.EmployeeID != colleague.EmployeeID {
		t.Fatalf("Korrektur muss beim Mitarbeiter bleiben, bekommen %s", fixed.EmployeeID)
	}
	if fixed.CreatedByUserID == nil || *fixed.CreatedByUserID != hrUser.UserID {
		t.Fatalf("erwartet die Personalabteilung als Urheber der Korrektur, bekommen %v", fixed.CreatedByUserID)
	}
	// Die Personalabteilung hat selbst keinen Mitarbeiterdatensatz: ohne
	// employee_id ist unklar, wessen Liste gemeint ist.
	wantAPIError(t, doAuthedRequest(t, handler, hrUser.Token, http.MethodGet, list, ""),
		http.StatusBadRequest, "validation_error", "employee_id erforderlich")

	// Fehlerhafte Abfragen.
	base := "/api/v1/time-entries/"
	for path, want := range map[string]string{
		base + "?from=02.03.2026&to=2026-03-02":                 "Ungültiges Datum in from (erwartet JJJJ-MM-TT)",
		base + "?from=2026-03-02&to=morgen":                     "Ungültiges Datum in to (erwartet JJJJ-MM-TT)",
		base + "?from=2026-03-02":                               "Zeitraum (von/bis) erforderlich",
		base + "":                                               "Zeitraum (von/bis) erforderlich",
		base + "?from=2026-03-03&to=2026-03-02":                 "Bis-Datum darf nicht vor dem Von-Datum liegen",
		base + "?from=2026-01-01&to=2027-01-02":                 "Zeitraum darf höchstens 366 Tage umfassen",
		base + "?from=2026-03-02&to=2026-03-02&employee_id=xyz": "Ungültige employee_id",
	} {
		wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, path, ""), http.StatusBadRequest, "validation_error", want)
	}
	wantAPIError(t, doAuthedRequest(t, handler, hrUser.Token, http.MethodGet, list+"&employee_id="+uuid.NewString(), ""),
		http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
}

// Ein Administrator eines ANDEREN Mandanten hat jedes Recht - und kommt
// trotzdem an nichts heran. Die Mandantentrennung liegt unter der
// Rechtepruefung, nicht in ihr.
func TestTimeEntriesAreIsolatedBetweenCompaniesOverHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	worker := seedTimeEntryUser(t, env, handler, "zeit-iso-worker@example.com", "production", true)
	entry := decodeTimeEntry(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, "/api/v1/time-entries/",
		closedTimeEntryBody(nil, "2026-03-02T07:00:00+01:00", "2026-03-02T15:30:00+01:00", 30)), http.StatusCreated)

	if _, err := env.PG.Exec(ctx, `INSERT INTO company_profiles (id, name) VALUES ('other-company','Andere Firma GmbH') ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatalf("zweiten Mandanten anlegen: %v", err)
	}
	foreignID := testutil.SeedAuthUser(t, env, "zeit-iso-admin@other.example.com", "Secret123!", "admin")
	if _, err := env.PG.Exec(ctx, `UPDATE users SET company_id='other-company' WHERE id=$1`, foreignID); err != nil {
		t.Fatalf("Benutzer dem zweiten Mandanten zuordnen: %v", err)
	}
	foreign := loginIntegrationUser(t, handler, "zeit-iso-admin@other.example.com", "Secret123!")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, foreign, method, path, body)
	}
	id := entry.ID.String()

	wantAPIError(t, do(http.MethodGet, "/api/v1/time-entries/"+id, ""), http.StatusNotFound, "not_found", "Zeiteintrag nicht gefunden")
	wantAPIError(t, do(http.MethodPost, "/api/v1/time-entries/"+id+"/stop", `{}`), http.StatusNotFound, "not_found", "Zeiteintrag nicht gefunden")
	wantAPIError(t, do(http.MethodPost, "/api/v1/time-entries/"+id+"/storno", `{"reason":"Fremdmandant"}`), http.StatusNotFound, "not_found", "Zeiteintrag nicht gefunden")
	wantAPIError(t, do(http.MethodPost, "/api/v1/time-entries/"+id+"/correct",
		`{"started_at":"2026-03-02T08:00:00+01:00","ended_at":"2026-03-02T15:30:00+01:00","reason":"Fremdmandant"}`), http.StatusNotFound, "not_found", "Zeiteintrag nicht gefunden")
	wantAPIError(t, do(http.MethodGet, "/api/v1/time-entries/?from=2026-03-02&to=2026-03-02&employee_id="+worker.EmployeeID.String(), ""),
		http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	wantAPIError(t, do(http.MethodPost, "/api/v1/time-entries/",
		closedTimeEntryBody(&worker.EmployeeID, "2026-03-03T07:00:00+01:00", "2026-03-03T15:30:00+01:00", 30)), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")

	// Nichts davon hat etwas veraendert oder hinterlassen.
	still := decodeTimeEntry(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, "/api/v1/time-entries/"+id, ""), http.StatusOK)
	if still.Status != "active" {
		t.Fatalf("Zugriff aus fremdem Mandanten hat den Eintrag veraendert: %+v", still)
	}
	var count int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM hr_time_entries WHERE employee_id=$1`, worker.EmployeeID).Scan(&count); err != nil {
		t.Fatalf("Eintraege zaehlen: %v", err)
	}
	if count != 1 {
		t.Fatalf("erwartet weiterhin genau einen Eintrag, bekommen %d", count)
	}
}
