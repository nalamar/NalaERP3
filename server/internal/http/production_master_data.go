package apihttp

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/production"
)

// HTTP-Anbindung der Stammdaten der Grobplanung (ADR 0037, Backlog H.2.2):
// Arbeitsplaetze und Arbeitsgangvorlagen. Lesen mit production.read,
// anlegen und aendern (auch deaktivieren) mit production.write. Loeschen
// gibt es nicht.

func mountProductionMasterDataRoutes(protected chi.Router, svc *production.Service) {
	read := requirePermission(permProductionRead)
	write := requirePermission(permProductionWrite)

	type workCenterBody struct {
		Name         string  `json:"name"`
		StundenJeTag float64 `json:"stunden_je_tag"`
		Aktiv        *bool   `json:"aktiv"`
	}
	protected.Route("/production/work-centers", func(r chi.Router) {
		// include_inactive=true zeigt auch deaktivierte.
		r.With(read).Get("/", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ListWorkCenters(req.Context(), companyID, req.URL.Query().Get("include_inactive") == "true")
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in workCenterBody
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.CreateWorkCenter(req.Context(), production.WorkCenterInput{Name: in.Name, StundenJeTag: in.StundenJeTag}, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		// Alle Angaben auf einmal; aktiv weglassen laesst den Zustand.
		r.With(write).Put("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Arbeitsplatz-ID")
			if !ok {
				return
			}
			var in workCenterBody
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.UpdateWorkCenter(req.Context(), id, production.WorkCenterInput{Name: in.Name, StundenJeTag: in.StundenJeTag}, in.Aktiv, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})

	type templateBody struct {
		Name            string    `json:"name"`
		WorkCenterID    uuid.UUID `json:"work_center_id"`
		Reihenfolge     int       `json:"reihenfolge"`
		RuestzeitMin    float64   `json:"ruestzeit_min"`
		MinutenJeStueck float64   `json:"minuten_je_stueck"`
		Aktiv           *bool     `json:"aktiv"`
	}
	toInput := func(in templateBody) production.TemplateInput {
		return production.TemplateInput{Name: in.Name, WorkCenterID: in.WorkCenterID, Reihenfolge: in.Reihenfolge,
			RuestzeitMin: in.RuestzeitMin, MinutenJeStueck: in.MinutenJeStueck}
	}
	protected.Route("/production/operation-templates", func(r chi.Router) {
		// In der Reihenfolge des Vorschlags; include_inactive=true zeigt
		// auch deaktivierte.
		r.With(read).Get("/", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ListTemplates(req.Context(), companyID, req.URL.Query().Get("include_inactive") == "true")
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in templateBody
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.CreateTemplate(req.Context(), toInput(in), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(write).Put("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Vorlagen-ID")
			if !ok {
				return
			}
			var in templateBody
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.UpdateTemplate(req.Context(), id, toInput(in), in.Aktiv, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
