package apihttp

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/hr"
)

// Stellen als Vorlagen fuer erforderliche Qualifikationen (ADR 0031,
// Backlog F.11.2). Rechte wie beim Qualifikationskatalog: lesen mit
// einem der Qualifikationsrechte, pflegen mit qualifications.write.

// positionUpdateFields: PUT ersetzt die Stelle vollstaendig, wie bei den
// Qualifikationsarten. Ein vergessenes "type_ids" leerte sonst die Stelle,
// ein vergessenes "aktiv" legte sie still.
var positionUpdateFields = []string{"name", "aktiv", "hinweis", "type_ids"}

func parsePositionID(w http.ResponseWriter, req *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(req, "id"))
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Stellen-ID")
		return uuid.Nil, false
	}
	return id, true
}

func mountPositionRoutes(protected chi.Router, svc *hr.PositionService) {
	readAny := requireAnyPermission(permQualificationsSelf, permQualificationsRead, permQualificationsWrite)
	write := requirePermission(permQualificationsWrite)

	protected.Route("/positions", func(r chi.Router) {
		r.With(readAny).Get("/", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.List(req.Context(), companyID, req.URL.Query().Get("include_inactive") == "true")
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in hr.PositionInput
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

		// Vorschlagskatalog uebernehmen (F.11.4): legt nur an, was fehlt,
		// und liefert die NEU angelegten Stellen samt fehlender Arten.
		r.With(write).Post("/adopt-defaults", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.AdoptDefaults(req.Context(), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(readAny).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parsePositionID(w, req)
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

		r.With(write).Put("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parsePositionID(w, req)
			if !ok {
				return
			}
			var in hr.PositionUpdate
			if !decodeCompleteReplacement(w, req, positionUpdateFields, "Der Code einer Stelle ist nicht änderbar", &in) {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Update(req.Context(), id, in, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
