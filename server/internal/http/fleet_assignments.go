package apihttp

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/fleet"
)

// HTTP-Anbindung der Fahrerzuordnung (ADR 0032 §4, Backlog G.3a.2), nach
// dem Muster der Betriebsmittel-Ausgabe (/asset-assignments). Schreiben
// mit fleet.write; lesen mit fleet.read alle, mit fleet.self nur die
// eigenen Zuordnungen.

func mountFleetAssignmentRoutes(protected chi.Router, svc *fleet.AssignmentService, resolver employeeForUser) {
	write := requirePermission(permFleetWrite)

	protected.Route("/fleet/assignments", func(r chi.Router) {
		// Zuordnen. Die Antwort enthaelt "hinweise" (Termin ueberfaellig,
		// keine gueltige Fuehrerscheinkontrolle); ein Fahrzeug mit HU/UVV
		// "Maengel"/"nicht bestanden" wird abgelehnt.
		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				VehicleID  uuid.UUID `json:"vehicle_id"`
				EmployeeID uuid.UUID `json:"employee_id"`
				Beginn     string    `json:"beginn"`
				Notiz      string    `json:"notiz"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			beginn, ok := parseOptionalDate(w, req, "beginn", in.Beginn)
			if !ok {
				return
			}
			input := fleet.AssignInput{VehicleID: in.VehicleID, EmployeeID: in.EmployeeID, Notiz: in.Notiz}
			if beginn != nil {
				input.Beginn = *beginn // fehlt es, meldet der Service "Beginn erforderlich"
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Assign(req.Context(), input, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		// Liste, die juengsten zuerst. Filter: vehicle_id, employee_id,
		// nur_offen=true, include_storniert=true. Ohne fleet.read nur die
		// eigenen - eine fremde employee_id ist dann verboten.
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			vehicleID, ok := optionalQueryUUID(w, req, "vehicle_id")
			if !ok {
				return
			}
			employeeID, ok := optionalEmployeeID(w, req)
			if !ok {
				return
			}
			if !hasPermission(req, permFleetRead) {
				own, ok := resolveOwnOrPermittedEmployee(w, req, resolver, employeeID, permFleetRead, permFleetSelf)
				if !ok {
					return
				}
				employeeID = &own
			}
			q := req.URL.Query()
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ListAssignments(req.Context(), companyID, fleet.AssignmentFilter{
				VehicleID: vehicleID, EmployeeID: employeeID,
				NurOffen: q.Get("nur_offen") == "true", IncludeStorniert: q.Get("include_storniert") == "true",
			})
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Zuordnungs-ID")
			if !ok {
				return
			}
			if !hasPermission(req, permFleetRead) && !hasPermission(req, permFleetSelf) {
				writeAPIError(w, req, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.GetAssignment(req.Context(), id, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			if _, ok := resolveOwnOrPermittedEmployee(w, req, resolver, &out.EmployeeID, permFleetRead, permFleetSelf); !ok {
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Beenden: einmalig, mit Datum.
		r.With(write).Post("/{id}/end", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Zuordnungs-ID")
			if !ok {
				return
			}
			var in struct {
				Ende string `json:"ende"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			ende, ok := parseOptionalDate(w, req, "ende", in.Ende)
			if !ok {
				return
			}
			var d time.Time
			if ende != nil {
				d = *ende // fehlt es, meldet der Service "Ende erforderlich"
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.End(req.Context(), id, d, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(write).Post("/{id}/storno", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Zuordnungs-ID")
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
			out, err := svc.StornoAssignment(req.Context(), id, in.Reason, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
