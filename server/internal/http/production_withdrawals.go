package apihttp

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"nalaerp3/internal/production"
)

// HTTP-Anbindung der Entnahmen am Fertigungsauftrag (ADR 0038, Backlog
// H.3.2c), eingehaengt unter /production/orders. Lesen mit
// production.read, entnehmen und stornieren mit production.write.

func mountProductionWithdrawalRoutes(r chi.Router, svc *production.Service) {
	read := requirePermission(permProductionRead)
	write := requirePermission(permProductionWrite)

	// Neueste zuerst, mit Stornos.
	r.With(read).Get("/{id}/withdrawals", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.ListWithdrawals(req.Context(), id, companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})

	// Body: item_id oder material_id, warehouse_id, optional location_id,
	// menge, notiz.
	r.With(write).Post("/{id}/withdrawals", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		var in production.WithdrawalInput
		if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
			writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.Withdraw(req.Context(), id, in, actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	})

	r.With(write).Post("/{id}/withdrawals/{wid}/storno", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		wid, ok := parseUUIDParam(w, req, "wid", "Ungültige Entnahme-ID")
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
		out, err := svc.CancelWithdrawal(req.Context(), id, wid, in.Reason, actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
}
