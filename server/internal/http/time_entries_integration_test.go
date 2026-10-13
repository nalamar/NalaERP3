package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/hr"
	"nalaerp3/internal/testutil"
)

// Es gibt noch keinen HTTP-Weg, einen Mitarbeiter anzulegen oder mit einem
// Benutzer zu verknuepfen (Backlog F.4). Die Tests legen den Mitarbeiter
// deshalb ueber den Service an und setzen hr_employees.user_id per SQL.

type timeEntryTestUser struct {
	Token      string
	UserID     string
	EmployeeID uuid.UUID
}

// seedTimeEntryUser legt einen Benutzer mit der Rolle an und meldet ihn an.
// Mit linked=true bekommt er einen eigenen Mitarbeiterdatensatz.
func seedTimeEntryUser(t *testing.T, env *testutil.IntegrationEnv, handler http.Handler, email, role string, linked bool) timeEntryTestUser {
	t.Helper()
	ctx := context.Background()
	userID := testutil.SeedAuthUser(t, env, email, "Secret123!", role)
	// Wiederholte Laeufe gegen dieselbe DB: eine alte Verknuepfung loesen,
	// sonst verletzt die neue den UNIQUE-Index auf user_id.
	if _, err := env.PG.Exec(ctx, `UPDATE hr_employees SET user_id=NULL WHERE user_id=$1`, userID); err != nil {
		t.Fatalf("alte Verknuepfung loesen: %v", err)
	}
	u := timeEntryTestUser{UserID: userID}
	if linked {
		u.EmployeeID = seedTimeEntryEmployee(t, env, email)
		if _, err := env.PG.Exec(ctx, `UPDATE hr_employees SET user_id=$1 WHERE id=$2`, userID, u.EmployeeID); err != nil {
			t.Fatalf("Benutzer mit Mitarbeiter verknuepfen: %v", err)
		}
	}
	u.Token = loginIntegrationUser(t, handler, email, "Secret123!")
	return u
}

func seedTimeEntryEmployee(t *testing.T, env *testutil.IntegrationEnv, lastName string) uuid.UUID {
	t.Helper()
	emp, err := hr.NewEmployeeService(env.PG).Create(context.Background(), hr.EmployeeInput{FirstName: "Zeit", LastName: lastName}, "default")
	if err != nil {
		t.Fatalf("Mitarbeiter anlegen: %v", err)
	}
	return emp.ID
}

type timeEntryResponse struct {
	ID              uuid.UUID  `json:"id"`
	EmployeeID      uuid.UUID  `json:"employee_id"`
	WorkDate        time.Time  `json:"work_date"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at"`
	BreakMinutes    int        `json:"break_minutes"`
	Status          string     `json:"status"`
	CreatedByUserID *string    `json:"created_by_user_id"`

	StorniertByUserID *string `json:"storniert_by_user_id"`
}

func decodeTimeEntry(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) timeEntryResponse {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("erwartet Status %d, bekommen %d mit Body %s", wantStatus, rec.Code, rec.Body.String())
	}
	var out timeEntryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

func wantAPIError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode, wantMessage string) {
	t.Helper()
	var out struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != wantStatus || out.Error.Code != wantCode || out.Error.Message != wantMessage {
		t.Fatalf("erwartet %d/%s/%q, bekommen %d mit Body %s", wantStatus, wantCode, wantMessage, rec.Code, rec.Body.String())
	}
}

func closedTimeEntryBody(employeeID *uuid.UUID, start, end string, breakMinutes int) string {
	emp := ""
	if employeeID != nil {
		emp = fmt.Sprintf(`"employee_id":%q,`, employeeID.String())
	}
	return fmt.Sprintf(`{%s"started_at":%q,"ended_at":%q,"break_minutes":%d}`, emp, start, end, breakMinutes)
}

// Der Normalfall eines Werkstattmitarbeiters: nur time_entries.self, kein
// employee_id im Aufruf, keine Zeitangabe - gestempelt wird mit der Uhr
// des Servers fuer den eigenen Mitarbeiter.
func TestTimeEntriesSelfStampInAndOut(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	worker := seedTimeEntryUser(t, env, handler, "zeit-self-stamp@example.com", "production", true)

	before := time.Now().Add(-2 * time.Second)
	started := decodeTimeEntry(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, "/api/v1/time-entries/", `{}`), http.StatusCreated)
	after := time.Now().Add(2 * time.Second)

	if started.EmployeeID != worker.EmployeeID {
		t.Fatalf("erwartet Eintrag fuer den eigenen Mitarbeiter %s, bekommen %s", worker.EmployeeID, started.EmployeeID)
	}
	if started.EndedAt != nil || started.Status != "active" {
		t.Fatalf("erwartet laufenden, aktiven Eintrag, bekommen %+v", started)
	}
	if started.StartedAt.Before(before) || started.StartedAt.After(after) {
		t.Fatalf("erwartet Beginn = Serverzeit (zwischen %s und %s), bekommen %s", before, after, started.StartedAt)
	}
	if started.CreatedByUserID == nil || *started.CreatedByUserID != worker.UserID {
		t.Fatalf("erwartet created_by_user_id=%s, bekommen %v", worker.UserID, started.CreatedByUserID)
	}

	// Zweites Stempeln: 409, nicht 400 und nicht 500.
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, "/api/v1/time-entries/", `{}`),
		http.StatusConflict, "conflict", "Es läuft bereits ein Zeiteintrag")

	got := decodeTimeEntry(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, "/api/v1/time-entries/"+started.ID.String(), ""), http.StatusOK)
	if got.ID != started.ID {
		t.Fatalf("Einzelabruf liefert anderen Eintrag: %+v", got)
	}

	// Der Eintrag laeuft erst seit Sekunden: eine Pause von 30 Minuten
	// kann nicht stimmen. Der Eintrag bleibt offen.
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, "/api/v1/time-entries/"+started.ID.String()+"/stop", `{"break_minutes":30}`),
		http.StatusBadRequest, "validation_error", "Pause muss kürzer sein als die Anwesenheit")

	time.Sleep(20 * time.Millisecond)
	stopped := decodeTimeEntry(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, "/api/v1/time-entries/"+started.ID.String()+"/stop", `{}`), http.StatusOK)
	if stopped.EndedAt == nil || !stopped.EndedAt.After(stopped.StartedAt) || stopped.EndedAt.After(time.Now().Add(2*time.Second)) {
		t.Fatalf("erwartet Ende = Serverzeit nach dem Beginn, bekommen %+v", stopped)
	}

	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, "/api/v1/time-entries/"+started.ID.String()+"/stop", `{}`),
		http.StatusConflict, "conflict", "Zeiteintrag ist bereits beendet")
}

// time_entries.self reicht genau bis zum eigenen Mitarbeiter.
func TestTimeEntriesSelfCannotTouchOtherEmployees(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	worker := seedTimeEntryUser(t, env, handler, "zeit-self-a@example.com", "production", true)
	colleague := seedTimeEntryUser(t, env, handler, "zeit-self-b@example.com", "production", true)

	// Fuer den Kollegen buchen: abgelehnt, und es entsteht nichts.
	rec := doAuthedRequest(t, handler, worker.Token, http.MethodPost, "/api/v1/time-entries/",
		closedTimeEntryBody(&colleague.EmployeeID, "2026-03-02T07:00:00+01:00", "2026-03-02T16:00:00+01:00", 45))
	wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
	var count int
	if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM hr_time_entries WHERE employee_id=$1`, colleague.EmployeeID).Scan(&count); err != nil {
		t.Fatalf("Eintraege zaehlen: %v", err)
	}
	if count != 0 {
		t.Fatalf("abgelehnte Buchung hat %d Eintraege hinterlassen", count)
	}

	// Mit ausdruecklich genanntem EIGENEN Mitarbeiter geht es.
	own := decodeTimeEntry(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, "/api/v1/time-entries/",
		closedTimeEntryBody(&worker.EmployeeID, "2026-03-02T07:00:00+01:00", "2026-03-02T16:00:00+01:00", 45)), http.StatusCreated)
	if own.BreakMinutes != 45 || own.WorkDate.Format("2006-01-02") != "2026-03-02" {
		t.Fatalf("Nachtrag nicht wie erfasst: %+v", own)
	}

	// Der Kollege stempelt selbst; der andere darf diesen Eintrag weder
	// lesen noch beenden.
	theirs := decodeTimeEntry(t, doAuthedRequest(t, handler, colleague.Token, http.MethodPost, "/api/v1/time-entries/", `{}`), http.StatusCreated)
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, "/api/v1/time-entries/"+theirs.ID.String(), ""),
		http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, "/api/v1/time-entries/"+theirs.ID.String()+"/stop", `{}`),
		http.StatusForbidden, "forbidden", "Keine Berechtigung")
	still := decodeTimeEntry(t, doAuthedRequest(t, handler, colleague.Token, http.MethodGet, "/api/v1/time-entries/"+theirs.ID.String(), ""), http.StatusOK)
	if still.EndedAt != nil {
		t.Fatalf("fremder Benutzer hat den Eintrag beendet: %+v", still)
	}
}

func TestTimeEntriesRequireLinkedEmployeeOrPermission(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)

	t.Run("self ohne verknuepften Mitarbeiter", func(t *testing.T) {
		unlinked := seedTimeEntryUser(t, env, handler, "zeit-unlinked@example.com", "production", false)
		wantAPIError(t, doAuthedRequest(t, handler, unlinked.Token, http.MethodPost, "/api/v1/time-entries/", `{}`),
			http.StatusForbidden, "forbidden", "Kein Mitarbeiter mit diesem Benutzer verknüpft")
	})

	t.Run("Benutzer ganz ohne Zeiterfassungsrecht", func(t *testing.T) {
		// Eine Rolle dieses Namens gibt es nicht: der Benutzer hat keine Rechte.
		none := seedTimeEntryUser(t, env, handler, "zeit-norole@example.com", "keine-rolle", true)
		wantAPIError(t, doAuthedRequest(t, handler, none.Token, http.MethodPost, "/api/v1/time-entries/", `{}`),
			http.StatusForbidden, "forbidden", "Keine Berechtigung")
	})

	t.Run("ohne Anmeldung", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/time-entries/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("erwartet 401, bekommen %d", rec.Code)
		}
	})

	t.Run("write ohne eigenen Mitarbeiter muss sagen, fuer wen", func(t *testing.T) {
		hrUser := seedTimeEntryUser(t, env, handler, "zeit-hr-unlinked@example.com", "hr", false)
		wantAPIError(t, doAuthedRequest(t, handler, hrUser.Token, http.MethodPost, "/api/v1/time-entries/", `{}`),
			http.StatusBadRequest, "validation_error", "employee_id erforderlich")
	})
}

// time_entries.write/read: die Personalabteilung bucht und liest fuer andere.
func TestTimeEntriesHRBooksForOthers(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	hrUser := seedTimeEntryUser(t, env, handler, "zeit-hr@example.com", "hr", true)
	worker := seedTimeEntryUser(t, env, handler, "zeit-hr-target@example.com", "production", true)

	// Nachtrag fuer den Mitarbeiter: 11 Stunden ohne Pause. Wird
	// angenommen (ADR 0024) - auch ueber HTTP kein Validierungsfehler.
	created := decodeTimeEntry(t, doAuthedRequest(t, handler, hrUser.Token, http.MethodPost, "/api/v1/time-entries/",
		closedTimeEntryBody(&worker.EmployeeID, "2026-03-02T06:00:00+01:00", "2026-03-02T17:00:00+01:00", 0)), http.StatusCreated)
	if created.EmployeeID != worker.EmployeeID {
		t.Fatalf("erwartet Eintrag fuer %s, bekommen %s", worker.EmployeeID, created.EmployeeID)
	}
	// Erfasst hat die Personalabteilung, nicht der Mitarbeiter.
	if created.CreatedByUserID == nil || *created.CreatedByUserID != hrUser.UserID {
		t.Fatalf("erwartet created_by_user_id=%s, bekommen %v", hrUser.UserID, created.CreatedByUserID)
	}

	decodeTimeEntry(t, doAuthedRequest(t, handler, hrUser.Token, http.MethodGet, "/api/v1/time-entries/"+created.ID.String(), ""), http.StatusOK)
	// Der Mitarbeiter selbst sieht den fuer ihn erfassten Eintrag.
	decodeTimeEntry(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, "/api/v1/time-entries/"+created.ID.String(), ""), http.StatusOK)

	// Ohne employee_id bucht auch die Personalabteilung fuer sich selbst.
	own := decodeTimeEntry(t, doAuthedRequest(t, handler, hrUser.Token, http.MethodPost, "/api/v1/time-entries/", `{}`), http.StatusCreated)
	if own.EmployeeID != hrUser.EmployeeID {
		t.Fatalf("erwartet eigenen Mitarbeiter %s, bekommen %s", hrUser.EmployeeID, own.EmployeeID)
	}

	// Die Personalabteilung beendet den laufenden Eintrag eines anderen.
	running := decodeTimeEntry(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, "/api/v1/time-entries/", `{"started_at":"2026-03-03T07:00:00+01:00"}`), http.StatusCreated)
	stopped := decodeTimeEntry(t, doAuthedRequest(t, handler, hrUser.Token, http.MethodPost, "/api/v1/time-entries/"+running.ID.String()+"/stop",
		`{"ended_at":"2026-03-03T15:30:00+01:00","break_minutes":30}`), http.StatusOK)
	if stopped.EndedAt == nil || stopped.BreakMinutes != 30 {
		t.Fatalf("Beenden durch die Personalabteilung nicht wie angegeben: %+v", stopped)
	}
}

func TestTimeEntriesHTTPErrorMapping(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	worker := seedTimeEntryUser(t, env, handler, "zeit-errors@example.com", "production", true)
	post := func(body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, worker.Token, http.MethodPost, "/api/v1/time-entries/", body)
	}

	decodeTimeEntry(t, post(closedTimeEntryBody(nil, "2026-03-02T08:00:00+01:00", "2026-03-02T12:00:00+01:00", 0)), http.StatusCreated)

	wantAPIError(t, post(closedTimeEntryBody(nil, "2026-03-02T11:00:00+01:00", "2026-03-02T13:00:00+01:00", 0)),
		http.StatusConflict, "conflict", "Zeitraum überschneidet sich mit einem bestehenden Zeiteintrag")
	wantAPIError(t, post(closedTimeEntryBody(nil, "2026-03-03T09:00:00+01:00", "2026-03-03T08:00:00+01:00", 0)),
		http.StatusBadRequest, "validation_error", "Ende muss nach dem Beginn liegen")
	wantAPIError(t, post(`{"ended_at":"2026-03-03T08:00:00+01:00"}`),
		http.StatusBadRequest, "validation_error", "Beginn erforderlich")
	wantAPIError(t, post(`{"started_at":"2099-01-01T08:00:00+01:00"}`),
		http.StatusBadRequest, "validation_error", "Beginn darf nicht in der Zukunft liegen")
	wantAPIError(t, post(`{"started_at":"2026-03-03T08:00:00+01:00","break_minutes":15}`),
		http.StatusBadRequest, "validation_error", "Pause kann erst beim Beenden angegeben werden")
	wantAPIError(t, post(`{"started_at":"2026-03-03T08:00:00+01:00","ended_at":"2026-03-03T09:00:00+01:00","project_id":"`+uuid.NewString()+`"}`),
		http.StatusNotFound, "not_found", "Projekt nicht gefunden")
	wantAPIError(t, post(`{"employee_id":"kein-uuid"}`),
		http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, post(`{`),
		http.StatusBadRequest, "validation_error", "Ungültige Eingabe")

	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, "/api/v1/time-entries/"+uuid.NewString(), ""),
		http.StatusNotFound, "not_found", "Zeiteintrag nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, "/api/v1/time-entries/kein-uuid", ""),
		http.StatusBadRequest, "validation_error", "Ungültige Zeiteintrag-ID")
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, "/api/v1/time-entries/"+uuid.NewString()+"/stop", `{}`),
		http.StatusNotFound, "not_found", "Zeiteintrag nicht gefunden")
}

// Jeder Fehlertext der Zeiterfassung muss als 4xx klassifiziert werden.
// Faellt einer durch, liefert die API fuer einen Bedienfehler einen 500
// (bekanntes Muster aus Backlog 0.29/0.40/0.44). DB-los.
func TestClassifyDomainErrorCoversTimeEntryMessages(t *testing.T) {
	cases := map[string]int{
		"Mandant erforderlich":                                          http.StatusBadRequest,
		"employee_id erforderlich":                                      http.StatusBadRequest,
		"Zeiteintrag-ID erforderlich":                                   http.StatusBadRequest,
		"Beginn erforderlich":                                           http.StatusBadRequest,
		"Storno-Begründung erforderlich":                                http.StatusBadRequest,
		"Zeitraum (von/bis) erforderlich":                               http.StatusBadRequest,
		"Beginn darf nicht in der Zukunft liegen":                       http.StatusBadRequest,
		"Ende darf nicht in der Zukunft liegen":                         http.StatusBadRequest,
		"Pause darf nicht negativ sein":                                 http.StatusBadRequest,
		"Bis-Datum darf nicht vor dem Von-Datum liegen":                 http.StatusBadRequest,
		"Zeitraum darf höchstens 366 Tage umfassen":                     http.StatusBadRequest,
		"Ende muss nach dem Beginn liegen":                              http.StatusBadRequest,
		"Pause muss kürzer sein als die Anwesenheit":                    http.StatusBadRequest,
		"Pause kann erst beim Beenden angegeben werden":                 http.StatusBadRequest,
		"Korrektur kann den Mitarbeiter nicht ändern":                   http.StatusBadRequest,
		"Mitarbeiter nicht gefunden":                                    http.StatusNotFound,
		"Projekt nicht gefunden":                                        http.StatusNotFound,
		"Zeiteintrag nicht gefunden":                                    http.StatusNotFound,
		"Es läuft bereits ein Zeiteintrag":                              http.StatusConflict,
		"Zeiteintrag ist bereits beendet":                               http.StatusConflict,
		"Zeiteintrag ist bereits storniert":                             http.StatusConflict,
		"Zeiteintrag ist storniert":                                     http.StatusConflict,
		"Zeitraum überschneidet sich mit einem bestehenden Zeiteintrag": http.StatusConflict,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
