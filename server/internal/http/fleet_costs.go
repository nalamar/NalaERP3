package apihttp

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/fleet"
)

// HTTP-Anbindung der Fahrzeugkosten (ADR 0032 §7, Backlog G.3d.2). Lesen
// mit fleet.read, erfassen und stornieren mit fleet.write - Kosten sind
// keine personenbezogenen Bewegungsdaten, fleet.self hat hier nichts.

// costFilterFromQuery liest vehicle_id, kostenart, von, bis. Bei false
// ist die Antwort bereits geschrieben.
func costFilterFromQuery(w http.ResponseWriter, req *http.Request) (fleet.CostFilter, bool) {
	q := req.URL.Query()
	vehicleID, ok := optionalQueryUUID(w, req, "vehicle_id")
	if !ok {
		return fleet.CostFilter{}, false
	}
	von, ok := parseOptionalDate(w, req, "von", q.Get("von"))
	if !ok {
		return fleet.CostFilter{}, false
	}
	bis, ok := parseOptionalDate(w, req, "bis", q.Get("bis"))
	if !ok {
		return fleet.CostFilter{}, false
	}
	kostenart := strings.ToLower(strings.TrimSpace(q.Get("kostenart")))
	if kostenart != "" && !slices.Contains(fleet.Kostenarten, kostenart) {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Kostenart")
		return fleet.CostFilter{}, false
	}
	return fleet.CostFilter{
		VehicleID: vehicleID, Kostenart: kostenart, Von: von, Bis: bis,
		IncludeStorniert: q.Get("include_storniert") == "true",
	}, true
}

func mountFleetCostRoutes(protected chi.Router, svc *fleet.CostService) {
	read := requirePermission(permFleetRead)
	write := requirePermission(permFleetWrite)

	protected.Route("/fleet/costs", func(r chi.Router) {
		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				VehicleID      uuid.UUID `json:"vehicle_id"`
				Datum          string    `json:"datum"`
				Kostenart      string    `json:"kostenart"`
				Bezeichnung    string    `json:"bezeichnung"`
				BetragNetto    float64   `json:"betrag_netto"`
				Lieferant      string    `json:"lieferant"`
				Belegnummer    string    `json:"belegnummer"`
				KmStand        *int      `json:"km_stand"`
				Liter          *float64  `json:"liter"`
				KostenstelleID *string   `json:"kostenstelle_id"`
				Notiz          string    `json:"notiz"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			datum, ok := parseOptionalDate(w, req, "datum", in.Datum)
			if !ok {
				return
			}
			input := fleet.CostInput{
				VehicleID: in.VehicleID, Kostenart: in.Kostenart, Bezeichnung: in.Bezeichnung, BetragNetto: in.BetragNetto,
				Lieferant: in.Lieferant, Belegnummer: in.Belegnummer, KmStand: in.KmStand, Liter: in.Liter,
				KostenstelleID: in.KostenstelleID, Notiz: in.Notiz,
			}
			if datum != nil {
				input.Datum = *datum // fehlt es, meldet der Service "Datum erforderlich"
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.RecordCost(req.Context(), input, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		// Liste, die juengsten zuerst. Filter: vehicle_id, kostenart, von,
		// bis (JJJJ-MM-TT, einschliesslich), include_storniert=true.
		r.With(read).Get("/", func(w http.ResponseWriter, req *http.Request) {
			f, ok := costFilterFromQuery(w, req)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ListCosts(req.Context(), companyID, f)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Summen je Kostenart und gesamt; dieselben Filter, stornierte
		// zaehlen nie.
		r.With(read).Get("/summary", func(w http.ResponseWriter, req *http.Request) {
			f, ok := costFilterFromQuery(w, req)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.SummarizeCosts(req.Context(), companyID, f)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(read).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Kosten-ID")
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.GetCost(req.Context(), id, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(write).Post("/{id}/storno", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Kosten-ID")
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
			out, err := svc.StornoCost(req.Context(), id, in.Reason, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
