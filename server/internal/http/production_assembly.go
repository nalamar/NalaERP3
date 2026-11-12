package apihttp

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/hr"
	"nalaerp3/internal/production"
)

// HTTP-Anbindung der Montagetermine (ADR 0038, Backlog H.3.6d). Termine
// je Auftrag unter /production/orders, Wochenuebersicht und Auswahl unter
// /production/montage. Lesen mit production.read, anlegen, aendern und
// stornieren mit production.write.

type assemblyBody struct {
	Von       string     `json:"von"`
	Bis       string     `json:"bis"`
	TeamID    *uuid.UUID `json:"team_id"`
	VehicleID *uuid.UUID `json:"vehicle_id"`
	Baustelle string     `json:"baustelle"`
	Notiz     string     `json:"notiz"`
}

// assemblyInput liest den Body; von/bis als JJJJ-MM-TT.
func assemblyInput(w http.ResponseWriter, req *http.Request) (production.AppointmentInput, bool) {
	var in assemblyBody
	if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
		return production.AppointmentInput{}, false
	}
	von, ok := parseOptionalDate(w, req, "von", in.Von)
	if !ok {
		return production.AppointmentInput{}, false
	}
	bis, ok := parseOptionalDate(w, req, "bis", in.Bis)
	if !ok {
		return production.AppointmentInput{}, false
	}
	return production.AppointmentInput{Von: von, Bis: bis, TeamID: in.TeamID, VehicleID: in.VehicleID, Baustelle: in.Baustelle, Notiz: in.Notiz}, true
}

func mountProductionAssemblyRoutes(r chi.Router, svc *production.Service) {
	read := requirePermission(permProductionRead)
	write := requirePermission(permProductionWrite)

	r.With(read).Get("/{id}/montage", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.ListAppointments(req.Context(), id, companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})

	r.With(write).Post("/{id}/montage", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		in, ok := assemblyInput(w, req)
		if !ok {
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.CreateAppointment(req.Context(), id, in, actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	})

	r.With(write).Put("/{id}/montage/{aid}", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		aid, ok := parseUUIDParam(w, req, "aid", "Ungültige Termin-ID")
		if !ok {
			return
		}
		in, ok := assemblyInput(w, req)
		if !ok {
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.UpdateAppointment(req.Context(), id, aid, in, actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})

	r.With(write).Post("/{id}/montage/{aid}/stornieren", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		aid, ok := parseUUIDParam(w, req, "aid", "Ungültige Termin-ID")
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
		out, err := svc.CancelAppointment(req.Context(), id, aid, in.Reason, actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
}

func mountProductionAssemblyOverviewRoutes(protected chi.Router, svc *production.Service) {
	read := requirePermission(permProductionRead)

	// Wochenuebersicht je Team; von/bis wie bei der Auslastung (ohne
	// Angabe ab der aktuellen Woche acht Wochen).
	protected.With(read).Get("/production/montage", func(w http.ResponseWriter, req *http.Request) {
		q := req.URL.Query()
		von, ok := parseOptionalDate(w, req, "von", q.Get("von"))
		if !ok {
			return
		}
		bis, ok := parseOptionalDate(w, req, "bis", q.Get("bis"))
		if !ok {
			return
		}
		if von == nil {
			today := hr.WorkDateOf(time.Now())
			von = &today
		}
		if bis == nil {
			end := von.AddDate(0, 0, 7*7)
			bis = &end
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.AssemblyWeeks(req.Context(), *von, *bis, companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})

	// Teams und Fahrzeuge zur Auswahl, ohne Personendaten.
	protected.With(read).Get("/production/montage/auswahl", func(w http.ResponseWriter, req *http.Request) {
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.AssemblyChoices(req.Context(), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
}
