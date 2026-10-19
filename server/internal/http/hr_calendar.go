package apihttp

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/hr"
)

// HTTP-Anbindung des Feiertagskalenders (ADR 0029, Backlog F.8.2.2).
//
// Rechte:
//   - GET /holidays: jeder angemeldete Benutzer des Mandanten. Feiertage
//     und Betriebsruhe sind keine vertraulichen Angaben, und jeder, der
//     Urlaub plant, braucht sie.
//   - /settings/hr-calendar (Bundesland, bayerische Schalter):
//     settings.manage wie die uebrigen Mandanten-Einstellungen.
//   - /holidays/company-days (betriebliche Zusatztage pflegen):
//     leave_requests.decide - wer Urlaub genehmigt, legt auch die
//     Betriebsruhe fest (Personalabteilung, Geschaeftsfuehrung).

// calendarYear liest den Query-Parameter year; ohne Angabe das laufende
// Jahr in der Arbeitszeitzone. Bei false ist die Antwort geschrieben.
func calendarYear(w http.ResponseWriter, req *http.Request) (int, bool) {
	v := req.URL.Query().Get("year")
	if v == "" {
		return hr.WorkDateOf(time.Now()).Year(), true
	}
	year, err := strconv.Atoi(v)
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültiges Jahr")
		return 0, false
	}
	return year, true
}

func mountHRCalendarRoutes(protected chi.Router, svc *hr.CalendarService) {
	decide := requirePermission(permLeaveRequestsDecide)

	protected.With(requirePermission("settings.manage")).Route("/settings/hr-calendar", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.GetSettings(req.Context(), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		// Ersetzt die Einstellung vollstaendig; bundesland "" setzt sie
		// zurueck (dann nur die bundesweiten Feiertage).
		r.Put("/", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				Bundesland             string `json:"bundesland"`
				MariaeHimmelfahrt      bool   `json:"mariae_himmelfahrt"`
				AugsburgerFriedensfest bool   `json:"augsburger_friedensfest"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.SetSettings(req.Context(), hr.CalendarSettings{
				Bundesland: in.Bundesland, MariaeHimmelfahrt: in.MariaeHimmelfahrt, AugsburgerFriedensfest: in.AugsburgerFriedensfest,
			}, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})

	protected.Route("/holidays", func(r chi.Router) {
		// Alle freien Tage eines Jahres: gesetzliche und betriebliche.
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			year, ok := calendarYear(w, req)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Calendar(req.Context(), year, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(decide).Get("/company-days", func(w http.ResponseWriter, req *http.Request) {
			year, ok := calendarYear(w, req)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ListCompanyDays(req.Context(), year, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(decide).Post("/company-days", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				Datum string `json:"datum"`
				Name  string `json:"name"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			datum, ok := parseOptionalDate(w, req, "datum", in.Datum)
			if !ok {
				return
			}
			var d time.Time
			if datum != nil {
				d = *datum // fehlt es, meldet der Service "Datum erforderlich"
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.CreateCompanyDay(req.Context(), d, in.Name, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(decide).Delete("/company-days/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige ID")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			if err := svc.DeleteCompanyDay(req.Context(), id, companyID); err != nil {
				writeDomainError(w, req, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})
}
