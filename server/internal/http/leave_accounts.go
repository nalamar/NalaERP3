package apihttp

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/hr"
)

// HTTP-Anbindung des Urlaubskontos (ADR 0029, Backlog F.8.3.1).
//
// Rechte wie bei den Urlaubsantraegen:
//   - leave_requests.self: das eigene Konto lesen.
//   - leave_requests.read: das Konto jedes Mitarbeiters lesen.
//   - leave_requests.decide: das Konto eines Jahres festlegen (Anspruch,
//     Uebertrag, Hinweis auf Verfall) - Personalabteilung,
//     Geschaeftsfuehrung.

func mountLeaveAccountRoutes(protected chi.Router, svc *hr.LeaveAccountService, resolver employeeForUser) {
	protected.Route("/leave-accounts", func(r chi.Router) {
		// Konto eines Jahres; ohne employee_id das eigene, ohne year das
		// laufende Jahr.
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			requested, ok := optionalEmployeeID(w, req)
			if !ok {
				return
			}
			year, ok := calendarYear(w, req)
			if !ok {
				return
			}
			employeeID, ok := resolveOwnOrPermittedEmployee(w, req, resolver, requested, permLeaveRequestsRead, permLeaveRequestsSelf)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Get(req.Context(), employeeID, year, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Konto eines Jahres festlegen - vollstaendiger Ersatz; anspruch_tage
		// ist Pflicht, damit ein vergessenes Feld nicht still 0 Tage ergibt.
		r.With(requirePermission(permLeaveRequestsDecide)).Put("/{employee_id}/{year}", func(w http.ResponseWriter, req *http.Request) {
			employeeID, err := uuid.Parse(chi.URLParam(req, "employee_id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige employee_id")
				return
			}
			year, err := strconv.Atoi(chi.URLParam(req, "year"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültiges Jahr")
				return
			}
			var in struct {
				AnspruchTage     *float64 `json:"anspruch_tage"`
				UebertragTage    float64  `json:"uebertrag_tage"`
				HinweisErteiltAm string   `json:"hinweis_erteilt_am"`
				Notiz            string   `json:"notiz"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			if in.AnspruchTage == nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "anspruch_tage erforderlich")
				return
			}
			hint, ok := parseOptionalDate(w, req, "hinweis_erteilt_am", in.HinweisErteiltAm)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Set(req.Context(), employeeID, year, hr.LeaveAccountInput{
				AnspruchTage: *in.AnspruchTage, UebertragTage: in.UebertragTage, HinweisErteiltAm: hint, Notiz: in.Notiz,
			}, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
