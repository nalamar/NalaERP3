package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/production"
)

// HTTP-Anbindung der Fertigungsauftraege (ADR 0034, Backlog H.1.3). Lesen
// mit production.read, anlegen, aendern, freigeben, Status und Storno mit
// production.write.

const (
	permProductionRead  = "production.read"
	permProductionWrite = "production.write"
)

var productionStatuses = []string{production.StatusEntwurf, production.StatusFreigegeben, production.StatusInArbeit,
	production.StatusFertig, production.StatusStorniert}

// productionOrderFilterFromQuery liest project_id, elevation_id, status,
// include_storniert. Bei false ist die Antwort bereits geschrieben.
func productionOrderFilterFromQuery(w http.ResponseWriter, req *http.Request) (production.OrderFilter, bool) {
	q := req.URL.Query()
	projectID, ok := optionalQueryUUID(w, req, "project_id")
	if !ok {
		return production.OrderFilter{}, false
	}
	elevationID, ok := optionalQueryUUID(w, req, "elevation_id")
	if !ok {
		return production.OrderFilter{}, false
	}
	status := strings.ToLower(strings.TrimSpace(q.Get("status")))
	if status != "" && !slices.Contains(productionStatuses, status) {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültiger Status")
		return production.OrderFilter{}, false
	}
	return production.OrderFilter{ProjectID: projectID, ElevationID: elevationID, Status: status,
		IncludeStorniert: q.Get("include_storniert") == "true"}, true
}

func mountProductionOrderRoutes(protected chi.Router, svc *production.Service) {
	read := requirePermission(permProductionRead)
	write := requirePermission(permProductionWrite)

	// action: ein Schritt ohne Eingabe (beginnen, fertigmelden).
	action := func(do func(ctx context.Context, id uuid.UUID, actorUserID, companyID string) (*production.Order, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := do(req.Context(), id, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		}
	}

	protected.Route("/production/orders", func(r chi.Router) {
		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				ProjectID          uuid.UUID `json:"project_id"`
				ElevationID        uuid.UUID `json:"elevation_id"`
				SingleElevationID  uuid.UUID `json:"single_elevation_id"`
				Stueckzahl         int       `json:"stueckzahl"`
				Liefertermin       string    `json:"liefertermin"`
				Ausfuehrungsklasse *string   `json:"ausfuehrungsklasse"`
				Notiz              string    `json:"notiz"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			liefertermin, ok := parseOptionalDate(w, req, "liefertermin", in.Liefertermin)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Create(req.Context(), production.OrderInput{
				ProjectID: in.ProjectID, ElevationID: in.ElevationID, SingleElevationID: in.SingleElevationID,
				Stueckzahl: in.Stueckzahl, Liefertermin: liefertermin, Ausfuehrungsklasse: in.Ausfuehrungsklasse, Notiz: in.Notiz,
			}, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		// Liste nach Liefertermin. Filter: project_id, elevation_id, status,
		// include_storniert=true.
		r.With(read).Get("/", func(w http.ResponseWriter, req *http.Request) {
			f, ok := productionOrderFilterFromQuery(w, req)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.List(req.Context(), companyID, f)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(read).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
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

		// Entwurf aendern: alle aenderbaren Angaben auf einmal.
		r.With(write).Put("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
			if !ok {
				return
			}
			var in struct {
				Stueckzahl         int     `json:"stueckzahl"`
				Liefertermin       string  `json:"liefertermin"`
				Ausfuehrungsklasse *string `json:"ausfuehrungsklasse"`
				Notiz              string  `json:"notiz"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			liefertermin, ok := parseOptionalDate(w, req, "liefertermin", in.Liefertermin)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.UpdateDraft(req.Context(), id, production.DraftUpdate{Stueckzahl: in.Stueckzahl, Liefertermin: liefertermin,
				Ausfuehrungsklasse: in.Ausfuehrungsklasse, Notiz: in.Notiz}, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Freigeben; optional {"lager_id": "..."} - dann wird aus diesem Lager
		// reserviert (H.1.5). Ohne Body: ohne Reservierung.
		r.With(write).Post("/{id}/freigeben", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
			if !ok {
				return
			}
			var in struct {
				LagerID string `json:"lager_id"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil && !errors.Is(err, io.EOF) {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Release(req.Context(), id, in.LagerID, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(write).Post("/{id}/beginnen", action(svc.Start))
		r.With(write).Post("/{id}/fertigmelden", action(svc.Finish))

		r.With(write).Post("/{id}/storno", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
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
			out, err := svc.Cancel(req.Context(), id, in.Reason, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
