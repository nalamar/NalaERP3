package apihttp

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/hr"
)

// HTTP-Anbindung der Zeiterfassung (ADR 0024, Backlog F.1.5).
//
// Die Routen tragen bewusst KEIN requirePermission: wer was darf, haengt
// nicht nur vom Recht ab, sondern davon, WESSEN Zeiten betroffen sind.
// Die Pruefung liegt deshalb in resolveTimeEntryEmployee und wird von
// jedem Handler aufgerufen, bevor er den Service anfasst.
//
//   - time_entries.write / time_entries.read: Zeiten aller Mitarbeiter
//     des Mandanten schreiben bzw. lesen.
//   - time_entries.self: nur die Zeiten des Mitarbeiters, der ueber
//     hr_employees.user_id mit dem angemeldeten Benutzer verknuepft ist.

const (
	permTimeEntriesSelf  = "time_entries.self"
	permTimeEntriesRead  = "time_entries.read"
	permTimeEntriesWrite = "time_entries.write"
)

func hasPermission(req *http.Request, permission string) bool {
	permissions, _ := authPermissionsFromContext(req.Context())
	for _, p := range permissions {
		if p == permission || p == adminSuperuserPermission {
			return true
		}
	}
	return false
}

// employeeForUser loest einen Login zu seinem Mitarbeiter auf
// (hr_employees.user_id). hr.TimeEntryService erfuellt das.
type employeeForUser interface {
	EmployeeIDForUser(ctx context.Context, userID, companyID string) (uuid.UUID, bool, error)
}

// resolveOwnOrPermittedEmployee bestimmt, fuer welchen Mitarbeiter ein
// Aufruf gilt, und prueft zugleich, ob der angemeldete Benutzer das darf.
// Das Muster "eigene Daten mit self-Recht, fremde mit einem breiten Recht"
// gilt fuer Zeiteintraege und Urlaubsantraege gleichermassen.
//
// requested ist der im Aufruf genannte Mitarbeiter (nil = keiner genannt,
// gemeint ist der eigene). broadPermission erlaubt den Zugriff auf fremde
// Daten, selfPermission den auf die eigenen.
//
// Bei false ist die Antwort bereits geschrieben.
func resolveOwnOrPermittedEmployee(w http.ResponseWriter, req *http.Request, resolver employeeForUser, requested *uuid.UUID, broadPermission, selfPermission string) (uuid.UUID, bool) {
	broad := hasPermission(req, broadPermission)
	if broad && requested != nil {
		return *requested, true
	}
	if !broad && !hasPermission(req, selfPermission) {
		writeAPIError(w, req, http.StatusForbidden, "forbidden", "Keine Berechtigung")
		return uuid.Nil, false
	}

	companyID, _ := companyIDFromContext(req.Context())
	own, linked, err := resolver.EmployeeIDForUser(req.Context(), actorUserIDFromContext(req.Context()), companyID)
	if err != nil {
		writeDomainError(w, req, err)
		return uuid.Nil, false
	}
	if !linked {
		if broad {
			// Darf fuer alle handeln, hat aber selbst keinen
			// Mitarbeiterdatensatz: dann muss er sagen, fuer wen.
			writeAPIError(w, req, http.StatusBadRequest, "validation_error", "employee_id erforderlich")
			return uuid.Nil, false
		}
		writeAPIError(w, req, http.StatusForbidden, "forbidden", "Kein Mitarbeiter mit diesem Benutzer verknüpft")
		return uuid.Nil, false
	}
	if requested != nil && *requested != own {
		writeAPIError(w, req, http.StatusForbidden, "forbidden", "Keine Berechtigung")
		return uuid.Nil, false
	}
	return own, true
}

// resolveTimeEntryEmployee ist resolveOwnOrPermittedEmployee mit den
// Rechten der Zeiterfassung. broadPermission ist time_entries.read oder
// time_entries.write.
func resolveTimeEntryEmployee(w http.ResponseWriter, req *http.Request, svc *hr.TimeEntryService, requested *uuid.UUID, broadPermission string) (uuid.UUID, bool) {
	return resolveOwnOrPermittedEmployee(w, req, svc, requested, broadPermission, permTimeEntriesSelf)
}

// loadAuthorizedTimeEntry laedt einen Eintrag und prueft, ob der
// angemeldete Benutzer auf die Zeiten dieses Mitarbeiters zugreifen darf.
// Bei false ist die Antwort bereits geschrieben.
func loadAuthorizedTimeEntry(w http.ResponseWriter, req *http.Request, svc *hr.TimeEntryService, broadPermission string) (*hr.TimeEntry, bool) {
	id, err := uuid.Parse(chi.URLParam(req, "id"))
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Zeiteintrag-ID")
		return nil, false
	}
	companyID, _ := companyIDFromContext(req.Context())
	entry, err := svc.Get(req.Context(), id, companyID)
	if err != nil {
		writeDomainError(w, req, err)
		return nil, false
	}
	if _, ok := resolveTimeEntryEmployee(w, req, svc, &entry.EmployeeID, broadPermission); !ok {
		return nil, false
	}
	return entry, true
}

func mountTimeEntryRoutes(protected chi.Router, svc *hr.TimeEntryService) {
	protected.Route("/time-entries", func(r chi.Router) {
		// Anlegen. Ohne employee_id fuer den eigenen Mitarbeiter. Ohne
		// started_at und ended_at wird gestempelt, und zwar mit der Uhr
		// des Servers: beim Stempeln ist "jetzt" gemeint, und die Uhr des
		// Clients ist dafuer keine verlaessliche Quelle.
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				EmployeeID   *uuid.UUID `json:"employee_id"`
				StartedAt    *time.Time `json:"started_at"`
				EndedAt      *time.Time `json:"ended_at"`
				BreakMinutes int        `json:"break_minutes"`
				ProjectID    *uuid.UUID `json:"project_id"`
				Note         string     `json:"note"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			employeeID, ok := resolveTimeEntryEmployee(w, req, svc, in.EmployeeID, permTimeEntriesWrite)
			if !ok {
				return
			}
			entry := hr.TimeEntryInput{EmployeeID: employeeID, EndedAt: in.EndedAt, BreakMinutes: in.BreakMinutes, ProjectID: in.ProjectID, Note: in.Note}
			switch {
			case in.StartedAt != nil:
				entry.StartedAt = *in.StartedAt
			case in.EndedAt == nil:
				entry.StartedAt = time.Now()
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Create(req.Context(), entry, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		// Beenden. Ohne ended_at mit der Uhr des Servers.
		r.Post("/{id}/stop", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				EndedAt      *time.Time `json:"ended_at"`
				BreakMinutes int        `json:"break_minutes"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			entry, ok := loadAuthorizedTimeEntry(w, req, svc, permTimeEntriesWrite)
			if !ok {
				return
			}
			endedAt := time.Now()
			if in.EndedAt != nil {
				endedAt = *in.EndedAt
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Stop(req.Context(), entry.ID, endedAt, in.BreakMinutes, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			entry, ok := loadAuthorizedTimeEntry(w, req, svc, permTimeEntriesRead)
			if !ok {
				return
			}
			writeJSON(w, http.StatusOK, entry)
		})

		// Ersatzlos stornieren (z. B. Fehlstempelung).
		r.Post("/{id}/storno", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				Reason string `json:"reason"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			entry, ok := loadAuthorizedTimeEntry(w, req, svc, permTimeEntriesWrite)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Storno(req.Context(), entry.ID, in.Reason, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Korrigieren: storniert den Eintrag und legt den Ersatz an.
		// Geliefert wird der NEUE Eintrag (201). Anders als beim Stempeln
		// gibt es hier keine Serverzeit als Vorgabe - eine Korrektur muss
		// ihre Zeiten nennen.
		r.Post("/{id}/correct", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				StartedAt    *time.Time `json:"started_at"`
				EndedAt      *time.Time `json:"ended_at"`
				BreakMinutes int        `json:"break_minutes"`
				ProjectID    *uuid.UUID `json:"project_id"`
				Note         string     `json:"note"`
				Reason       string     `json:"reason"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			entry, ok := loadAuthorizedTimeEntry(w, req, svc, permTimeEntriesWrite)
			if !ok {
				return
			}
			replacement := hr.TimeEntryInput{EndedAt: in.EndedAt, BreakMinutes: in.BreakMinutes, ProjectID: in.ProjectID, Note: in.Note}
			if in.StartedAt != nil {
				replacement.StartedAt = *in.StartedAt
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Correct(req.Context(), entry.ID, replacement, in.Reason, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		// Liste samt Tagesauswertung. from/to als JJJJ-MM-TT, beide
		// einschliesslich. Ohne employee_id der eigene Mitarbeiter.
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			q := req.URL.Query()
			var requested *uuid.UUID
			if v := q.Get("employee_id"); v != "" {
				id, err := uuid.Parse(v)
				if err != nil {
					writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige employee_id")
					return
				}
				requested = &id
			}
			var from, to time.Time
			for _, p := range []struct {
				name string
				dst  *time.Time
			}{{"from", &from}, {"to", &to}} {
				v := q.Get(p.name)
				if v == "" {
					continue // fehlende Grenze meldet der Service
				}
				d, err := time.Parse("2006-01-02", v)
				if err != nil {
					writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültiges Datum in "+p.name+" (erwartet JJJJ-MM-TT)")
					return
				}
				*p.dst = d
			}
			employeeID, ok := resolveTimeEntryEmployee(w, req, svc, requested, permTimeEntriesRead)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.List(req.Context(), employeeID, from, to, q.Get("include_storniert") == "true", companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
