package apihttp

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/hr"
)

// HTTP-Anbindung des Mitarbeiterstamms (ADR 0025, Backlog F.4.4).
//
// Anders als bei der Zeiterfassung gibt es hier kein "self"-Recht: der
// Mitarbeiterstamm enthaelt personenbezogene Daten und ist der
// Personalabteilung vorbehalten (employees.read / employees.write). Die
// Routen sind deshalb schlicht ueber requirePermission geschuetzt.

// maxEmployeePageSize begrenzt eine Listenabfrage. Ohne Grenze liesse sich
// der gesamte Stamm mit einem Aufruf abziehen.
const maxEmployeePageSize = 500

func parseEmployeeID(w http.ResponseWriter, req *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(req, "id"))
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Mitarbeiter-ID")
		return uuid.Nil, false
	}
	return id, true
}

func mountEmployeeRoutes(protected chi.Router, svc *hr.EmployeeService, quals *hr.QualificationService) {
	protected.Route("/employees", func(r chi.Router) {
		r.With(requirePermission("employees.read")).Get("/", func(w http.ResponseWriter, req *http.Request) {
			q := req.URL.Query()
			limit, _ := strconv.Atoi(q.Get("limit"))
			offset, _ := strconv.Atoi(q.Get("offset"))
			if limit > maxEmployeePageSize {
				limit = maxEmployeePageSize
			}
			if offset < 0 {
				offset = 0
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.List(req.Context(), limit, offset, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(requirePermission("employees.read")).Get("/teams", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ListTeams(req.Context(), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Teams pflegen (F.8.5): employees.write wie die Mitarbeiter. Teams
		// haben vorerst keine Wirkung auf Rechte (ADR 0029).
		r.With(requirePermission("employees.write")).Post("/teams", func(w http.ResponseWriter, req *http.Request) {
			var in hr.TeamInput
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.CreateTeam(req.Context(), in, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("employees.write")).Put("/teams/{teamID}", func(w http.ResponseWriter, req *http.Request) {
			id, err := uuid.Parse(chi.URLParam(req, "teamID"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Team-ID")
				return
			}
			var in hr.TeamInput
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.UpdateTeam(req.Context(), id, in, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("employees.write")).Delete("/teams/{teamID}", func(w http.ResponseWriter, req *http.Request) {
			id, err := uuid.Parse(chi.URLParam(req, "teamID"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Team-ID")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			if err := svc.DeleteTeam(req.Context(), id, companyID); err != nil {
				writeDomainError(w, req, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		// Logins des Mandanten zur Auswahl beim Verknuepfen (Backlog
		// F.7.2.1). employees.write wie das Verknuepfen selbst; die volle
		// Benutzerliste unter /users verlangt users.manage, das die
		// Personalabteilung nicht hat und nicht braucht.
		r.With(requirePermission("employees.write")).Get("/login-users", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ListLoginUsers(req.Context(), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(requirePermission("employees.write")).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in hr.EmployeeInput
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Create(req.Context(), in, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		// Anlage mit Anforderungen und vorhandenen Nachweisen in einem
		// Aufruf (F.10.5.1): ganz oder gar nicht. Verlangt beide
		// Schreibrechte - die Anforderungen und Nachweise allein brauchen
		// qualifications.write.
		r.With(requirePermission("employees.write"), requirePermission(permQualificationsWrite)).Post("/with-qualifications", func(w http.ResponseWriter, req *http.Request) {
			var body struct {
				Employee        hr.EmployeeInput `json:"employee"`
				RequiredTypeIDs []uuid.UUID      `json:"required_type_ids"`
				Qualifications  []struct {
					TypeID     uuid.UUID `json:"type_id"`
					ErworbenAm string    `json:"erworben_am"`
					GueltigBis string    `json:"gueltig_bis"`
					Aussteller string    `json:"aussteller"`
					NachweisNr string    `json:"nachweis_nr"`
					Note       string    `json:"note"`
				} `json:"qualifications"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			in := hr.EmployeeWithQualificationsInput{Employee: body.Employee, RequiredTypeIDs: body.RequiredTypeIDs}
			for _, q := range body.Qualifications {
				erworben, ok := parseOptionalDate(w, req, "erworben_am", q.ErworbenAm)
				if !ok {
					return
				}
				gueltigBis, ok := parseOptionalDate(w, req, "gueltig_bis", q.GueltigBis)
				if !ok {
					return
				}
				iq := hr.InitialQualification{TypeID: q.TypeID, GueltigBis: gueltigBis, Aussteller: q.Aussteller, NachweisNr: q.NachweisNr, Note: q.Note}
				if erworben != nil {
					iq.ErworbenAm = *erworben // fehlt es, meldet der Service "Erwerbsdatum erforderlich"
				}
				in.Qualifications = append(in.Qualifications, iq)
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := quals.CreateEmployeeWithQualifications(req.Context(), in, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		r.With(requirePermission("employees.read")).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseEmployeeID(w, req)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Get(req.Context(), id, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Aendern ueber einen Patch: nur die genannten Felder. Welche
		// Felder zulaessig sind und welchen Typ sie tragen, prueft der
		// Service. Die Benutzer-Verknuepfung gehoert NICHT dazu, sie hat
		// ihre eigene Route.
		r.With(requirePermission("employees.write")).Patch("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseEmployeeID(w, req)
			if !ok {
				return
			}
			var patch map[string]any
			if err := json.NewDecoder(req.Body).Decode(&patch); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			if err := svc.Update(req.Context(), id, patch, companyID); err != nil {
				writeDomainError(w, req, err)
				return
			}
			out, err := svc.Get(req.Context(), id, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Mitarbeiter mit einem Login verknuepfen ("user_id": "...") oder
		// die Verknuepfung loesen ("user_id": null). Die Verknuepfung
		// entscheidet, wessen Zeiten ein Login mit time_entries.self bucht.
		r.With(requirePermission("employees.write")).Put("/{id}/user", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseEmployeeID(w, req)
			if !ok {
				return
			}
			var in struct {
				UserID *string `json:"user_id"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			userID := ""
			if in.UserID != nil {
				userID = *in.UserID
			}
			companyID, _ := companyIDFromContext(req.Context())
			if err := svc.SetUser(req.Context(), id, userID, companyID); err != nil {
				writeDomainError(w, req, err)
				return
			}
			out, err := svc.Get(req.Context(), id, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
