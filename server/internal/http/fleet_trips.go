package apihttp

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/fleet"
)

// HTTP-Anbindung des Fahrtenbuchs (ADR 0032 §6, Backlog G.3c.3), nach dem
// Muster der Fahrerzuordnung. Fahrer erfassen und lesen ihre eigenen
// Fahrten mit fleet.self; fleet.write erfasst fuer jeden, fleet.read liest
// alle. Stornieren nur mit fleet.write.

func mountFleetTripRoutes(protected chi.Router, svc *fleet.TripService, resolver employeeForUser) {
	write := requirePermission(permFleetWrite)

	protected.Route("/fleet/trips", func(r chi.Router) {
		// Erfassen. Ohne employee_id gilt die Fahrt dem eigenen Mitarbeiter;
		// eine fremde employee_id verlangt fleet.write.
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			if !hasPermission(req, permFleetWrite) && !hasPermission(req, permFleetSelf) {
				writeAPIError(w, req, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				return
			}
			var in struct {
				VehicleID         uuid.UUID `json:"vehicle_id"`
				EmployeeID        uuid.UUID `json:"employee_id"`
				Datum             string    `json:"datum"`
				KmBeginn          int       `json:"km_beginn"`
				KmEnde            int       `json:"km_ende"`
				Fahrtart          string    `json:"fahrtart"`
				Ziel              string    `json:"ziel"`
				Zweck             string    `json:"zweck"`
				Geschaeftspartner string    `json:"geschaeftspartner"`
				Notiz             string    `json:"notiz"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			datum, ok := parseOptionalDate(w, req, "datum", in.Datum)
			if !ok {
				return
			}
			var requested *uuid.UUID
			if in.EmployeeID != uuid.Nil {
				requested = &in.EmployeeID
			}
			employeeID, ok := resolveOwnOrPermittedEmployee(w, req, resolver, requested, permFleetWrite, permFleetSelf)
			if !ok {
				return
			}
			input := fleet.TripInput{
				VehicleID: in.VehicleID, EmployeeID: employeeID, KmBeginn: in.KmBeginn, KmEnde: in.KmEnde,
				Fahrtart: in.Fahrtart, Ziel: in.Ziel, Zweck: in.Zweck, Geschaeftspartner: in.Geschaeftspartner, Notiz: in.Notiz,
			}
			if datum != nil {
				input.Datum = *datum // fehlt es, meldet der Service "Datum erforderlich"
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.RecordTrip(req.Context(), input, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		// Liste, die juengsten zuerst. Filter: vehicle_id, employee_id,
		// von, bis (JJJJ-MM-TT, einschliesslich), include_storniert=true,
		// nur_eigene=true. Ohne fleet.read nur die eigenen - eine fremde employee_id ist
		// dann verboten.
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			vehicleID, ok := optionalQueryUUID(w, req, "vehicle_id")
			if !ok {
				return
			}
			employeeID, ok := optionalEmployeeID(w, req)
			if !ok {
				return
			}
			q := req.URL.Query()
			von, ok := parseOptionalDate(w, req, "von", q.Get("von"))
			if !ok {
				return
			}
			bis, ok := parseOptionalDate(w, req, "bis", q.Get("bis"))
			if !ok {
				return
			}
			if employeeID, ok = ownFleetEmployee(w, req, resolver, employeeID); !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ListTrips(req.Context(), companyID, fleet.TripFilter{
				VehicleID: vehicleID, EmployeeID: employeeID, Von: von, Bis: bis,
				IncludeStorniert: q.Get("include_storniert") == "true",
			})
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Kilometerstand und Luecken eines Fahrzeugs - der Fahrer braucht
		// den Stand fuer die naechste Fahrt. Enthaelt keine Personendaten.
		r.Get("/chain", func(w http.ResponseWriter, req *http.Request) {
			if !hasPermission(req, permFleetRead) && !hasPermission(req, permFleetSelf) {
				writeAPIError(w, req, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				return
			}
			vehicleID, ok := optionalQueryUUID(w, req, "vehicle_id")
			if !ok {
				return
			}
			if vehicleID == nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "vehicle_id erforderlich")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ChainState(req.Context(), *vehicleID, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Fahrt-ID")
			if !ok {
				return
			}
			if !hasPermission(req, permFleetRead) && !hasPermission(req, permFleetSelf) {
				writeAPIError(w, req, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.GetTrip(req.Context(), id, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			if _, ok := resolveOwnOrPermittedEmployee(w, req, resolver, &out.EmployeeID, permFleetRead, permFleetSelf); !ok {
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(write).Post("/{id}/storno", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Fahrt-ID")
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
			out, err := svc.StornoTrip(req.Context(), id, in.Reason, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
