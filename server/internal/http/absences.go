package apihttp

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/hr"
)

// HTTP-Anbindung der Krankmeldungen (ADR 0029, Backlog F.8.4.2).
//
// Rechte:
//   - leave_requests.self: sich selbst krank melden (ohne Attest, wird
//     "neu" bis zur Bestaetigung) und die eigenen Abwesenheiten lesen.
//   - leave_requests.read: Abwesenheiten aller lesen, Liste "neue
//     Krankmeldungen".
//   - leave_requests.decide: Krankmeldungen fuer jeden erfassen (gilt als
//     bestaetigt), Attest festhalten, bestaetigen, stornieren.
//
// Gespeichert werden nur Zeitraum und Attest ja/nein - der Body kennt
// keine weiteren Felder; was der Aufrufer mitschickt, wird nicht gelesen.

func parseAbsenceID(w http.ResponseWriter, req *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(req, "id"))
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Abwesenheits-ID")
		return uuid.Nil, false
	}
	return id, true
}

func mountAbsenceRoutes(protected chi.Router, svc *hr.AbsenceService, resolver employeeForUser) {
	decide := requirePermission(permLeaveRequestsDecide)

	protected.Route("/absences", func(r chi.Router) {
		// Krankmeldung. Ohne employee_id fuer den eigenen Mitarbeiter.
		r.Post("/sick", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				EmployeeID *uuid.UUID `json:"employee_id"`
				StartDate  string     `json:"start_date"`
				EndDate    string     `json:"end_date"`
				Attest     bool       `json:"attest"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			employeeID, ok := resolveOwnOrPermittedEmployee(w, req, resolver, in.EmployeeID, permLeaveRequestsDecide, permLeaveRequestsSelf)
			if !ok {
				return
			}
			start, ok := parseOptionalDate(w, req, "start_date", in.StartDate)
			if !ok {
				return
			}
			end, ok := parseOptionalDate(w, req, "end_date", in.EndDate)
			if !ok {
				return
			}
			report := hr.SickReport{EmployeeID: employeeID, Attest: in.Attest}
			if start != nil {
				report.StartDate = *start
			}
			if end != nil {
				report.EndDate = *end
			}
			companyID, _ := companyIDFromContext(req.Context())
			bySelf := !hasPermission(req, permLeaveRequestsDecide)
			out, err := svc.ReportSick(req.Context(), report, bySelf, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		// Uebersicht "wer fehlt wann" (F.8.4.3). Mit leave_requests.read
		// alle Mitarbeiter (optional employee_id, team_id); sonst nur die
		// eigenen Abwesenheiten (leave_requests.self). Kollegen sehen sich
		// gegenseitig nicht (ADR 0029, Entscheidung des Nutzers).
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			q := req.URL.Query()
			requested, ok := optionalEmployeeID(w, req)
			if !ok {
				return
			}
			var teamID *uuid.UUID
			if v := q.Get("team_id"); v != "" {
				id, err := uuid.Parse(v)
				if err != nil {
					writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige team_id")
					return
				}
				teamID = &id
			}
			from, ok := parseOptionalDate(w, req, "from", q.Get("from"))
			if !ok {
				return
			}
			to, ok := parseOptionalDate(w, req, "to", q.Get("to"))
			if !ok {
				return
			}
			filter := hr.AbsenceFilter{EmployeeID: requested, TeamID: teamID, IncludeStorniert: q.Get("include_storniert") == "true"}
			if from != nil {
				filter.From = *from
			}
			if to != nil {
				filter.To = *to
			}
			// Mit Leserecht bleibt der Filter wie angefragt (auch keiner).
			if !hasPermission(req, permLeaveRequestsRead) {
				employeeID, ok := resolveOwnOrPermittedEmployee(w, req, resolver, requested, permLeaveRequestsRead, permLeaveRequestsSelf)
				if !ok {
					return
				}
				filter.EmployeeID = &employeeID
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.List(req.Context(), filter, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(requirePermission(permLeaveRequestsRead)).Get("/sick/new", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ListNewSick(req.Context(), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Einzelne Abwesenheit: eigene mit self, sonst read.
		r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseAbsenceID(w, req)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Get(req.Context(), id, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			if _, ok := resolveOwnOrPermittedEmployee(w, req, resolver, &out.EmployeeID, permLeaveRequestsRead, permLeaveRequestsSelf); !ok {
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(decide).Post("/{id}/attest", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseAbsenceID(w, req)
			if !ok {
				return
			}
			var in struct {
				Attest *bool `json:"attest"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			if in.Attest == nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "attest erforderlich")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.SetAttest(req.Context(), id, *in.Attest, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(decide).Post("/{id}/confirm", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseAbsenceID(w, req)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Confirm(req.Context(), id, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(decide).Post("/{id}/storno", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseAbsenceID(w, req)
			if !ok {
				return
			}
			var in struct {
				Reason string `json:"reason"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Storno(req.Context(), id, in.Reason, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
