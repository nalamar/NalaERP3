package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"nalaerp3/internal/llm"
	"nalaerp3/internal/matching"
)

// HTTP-Anbindung der Zuordnung am GAEB-Import (ADR 0045, Backlog I.5.3)
// unter /quotes/imports/{id}: Lesen mit quotes.read, alles andere mit
// quotes.write wie die uebrige Pruefung.

// backgroundMatchRunner fuehrt einen Lauf in einer Goroutine mit eigenem
// Context aus: er ueberdauert die Anfrage; Zeitlimits je Aufruf setzt der
// Client des Sprachmodells. Das Ergebnis steht am Importlauf; im Log nur
// die Meldung, keine Daten aus dem LV.
func backgroundMatchRunner(run func(context.Context) error) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Zuordnung abgebrochen (Panic): %v", r)
			}
		}()
		if err := run(context.Background()); err != nil {
			log.Printf("Zuordnung mit Fehler beendet: %v", err)
		}
	}()
}

func mountQuoteImportMatchRoutes(r chi.Router, svc *matching.LaufService, runner func(func(context.Context) error)) {
	// Lauf starten: 202 mit dem Status; der Client fragt den Status ab.
	// Leerer Body = Standardprofil (ohne Sprachmodell: Wortabgleich).
	r.With(requirePermission("quotes.write")).Post("/imports/{id}/match", func(w http.ResponseWriter, req *http.Request) {
		var in matching.LaufEingabe
		if req.ContentLength != 0 {
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
		}
		companyID, _ := companyIDFromContext(req.Context())
		st, run, err := svc.Starten(req.Context(), chi.URLParam(req, "id"), in, actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeMatchError(w, req, err)
			return
		}
		runner(run)
		writeJSON(w, http.StatusAccepted, st)
	})
	r.With(requirePermission("quotes.read")).Get("/imports/{id}/match", func(w http.ResponseWriter, req *http.Request) {
		companyID, _ := companyIDFromContext(req.Context())
		st, err := svc.Status(req.Context(), chi.URLParam(req, "id"), companyID)
		if err != nil {
			writeMatchError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, st)
	})
	r.With(requirePermission("quotes.read")).Get("/imports/{id}/matches", func(w http.ResponseWriter, req *http.Request) {
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.Pruefliste(req.Context(), chi.URLParam(req, "id"), companyID)
		if err != nil {
			writeMatchError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	// Sammelbestaetigung: alle offenen Vorschlaege ueber der Schwelle.
	r.With(requirePermission("quotes.write")).Post("/imports/{id}/matches/bestaetigen", func(w http.ResponseWriter, req *http.Request) {
		companyID, _ := companyIDFromContext(req.Context())
		n, err := svc.SammelBestaetigen(req.Context(), chi.URLParam(req, "id"), actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeMatchError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"bestaetigt": n})
	})
	// Entscheidung je Position: {"aktion": "bestaetigen"|"aendern"|"verwerfen", "leistung_id": nur beim Aendern}.
	r.With(requirePermission("quotes.write")).Put("/imports/{id}/matches/{itemID}", func(w http.ResponseWriter, req *http.Request) {
		var in matching.EntscheidungEingabe
		if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
			writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.Entscheiden(req.Context(), chi.URLParam(req, "id"), chi.URLParam(req, "itemID"), in,
			actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeMatchError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
}

// writeMatchError: ein ausgeschaltetes Profil ist ein Konflikt (409), der
// Rest wie ueblich ueber die Texte der Meldungen ("läuft bereits" = 409).
func writeMatchError(w http.ResponseWriter, req *http.Request, err error) {
	if errors.Is(err, llm.ErrNotEnabled) {
		writeAPIError(w, req, http.StatusConflict, "conflict", err.Error())
		return
	}
	writeDomainError(w, req, err)
}
