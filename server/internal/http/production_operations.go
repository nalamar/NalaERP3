package apihttp

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/production"
)

// HTTP-Anbindung der Arbeitsgaenge am Fertigungsauftrag (ADR 0037, Backlog
// H.2.3b), eingehaengt unter /production/orders. Lesen mit
// production.read, vorschlagen, anlegen, aendern, entfernen mit
// production.write.

type operationBody struct {
	Name            string    `json:"name"`
	WorkCenterID    uuid.UUID `json:"work_center_id"`
	RuestzeitMin    float64   `json:"ruestzeit_min"`
	MinutenJeStueck float64   `json:"minuten_je_stueck"`
	// Ein beliebiger Tag der Woche (JJJJ-MM-TT); leer = ungeplant.
	GeplanteWoche string `json:"geplante_woche"`
}

// decodeOperation liest den Body. Bei false ist die Antwort geschrieben.
func decodeOperation(w http.ResponseWriter, req *http.Request) (production.OperationInput, bool) {
	var in operationBody
	if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
		return production.OperationInput{}, false
	}
	woche, ok := parseOptionalDate(w, req, "geplante_woche", in.GeplanteWoche)
	if !ok {
		return production.OperationInput{}, false
	}
	return production.OperationInput{Name: in.Name, WorkCenterID: in.WorkCenterID, RuestzeitMin: in.RuestzeitMin,
		MinutenJeStueck: in.MinutenJeStueck, GeplanteWoche: woche}, true
}

func mountProductionOperationRoutes(r chi.Router, svc *production.Service) {
	read := requirePermission(permProductionRead)
	write := requirePermission(permProductionWrite)

	r.With(read).Get("/{id}/operations", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.ListOperations(req.Context(), id, companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})

	// Vorschlag aus den aktiven Vorlagen, nur fuer einen Auftrag ohne
	// Arbeitsgaenge.
	r.With(write).Post("/{id}/operations/vorschlagen", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.SuggestOperations(req.Context(), id, actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	})

	r.With(write).Post("/{id}/operations", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		in, ok := decodeOperation(w, req)
		if !ok {
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.AddOperation(req.Context(), id, in, actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	})

	// Alle Angaben auf einmal.
	r.With(write).Put("/{id}/operations/{opId}", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		opID, ok := parseUUIDParam(w, req, "opId", "Ungültige Arbeitsgang-ID")
		if !ok {
			return
		}
		in, ok := decodeOperation(w, req)
		if !ok {
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.UpdateOperation(req.Context(), id, opID, in, actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})

	r.With(write).Delete("/{id}/operations/{opId}", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		opID, ok := parseUUIDParam(w, req, "opId", "Ungültige Arbeitsgang-ID")
		if !ok {
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		if err := svc.RemoveOperation(req.Context(), id, opID, actorUserIDFromContext(req.Context()), companyID); err != nil {
			writeDomainError(w, req, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
