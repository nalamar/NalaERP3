package apihttp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"nalaerp3/internal/production"
)

// HTTP-Anbindung der Rueckmeldung je Arbeitsgang (ADR 0038, Backlog
// H.3.5d), eingehaengt unter /production/orders. Soll/Ist lesen mit
// production.read, melden und zuruecknehmen mit production.write.

func mountProductionFeedbackRoutes(r chi.Router, svc *production.Service) {
	read := requirePermission(permProductionRead)
	write := requirePermission(permProductionWrite)

	r.With(read).Get("/{id}/soll-ist", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.OrderSollIst(req.Context(), id, companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})

	// Body optional: {"ist_minuten": 95.5, "notiz": "..."}; ohne Body
	// erledigt ohne Ist-Zeit.
	r.With(write).Post("/{id}/operations/{opId}/erledigt", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		opID, ok := parseUUIDParam(w, req, "opId", "Ungültige Arbeitsgang-ID")
		if !ok {
			return
		}
		var in production.FeedbackInput
		if err := json.NewDecoder(req.Body).Decode(&in); err != nil && !errors.Is(err, io.EOF) {
			writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.ReportOperationDone(req.Context(), id, opID, in, actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})

	r.With(write).Post("/{id}/operations/{opId}/zuruecknehmen", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		opID, ok := parseUUIDParam(w, req, "opId", "Ungültige Arbeitsgang-ID")
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
		out, err := svc.RevertOperationDone(req.Context(), id, opID, in.Reason, actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
}
