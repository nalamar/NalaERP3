package apihttp

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/fleet"
	"nalaerp3/internal/hr"
)

// HTTP-Anbindung der Fuhrpark-Termine (ADR 0032 §3, Backlog G.2.3):
// Termine je Fahrzeug, Erledigungen, betriebsweite Faelligkeitsliste.
// Rechte wie der Fahrzeugstamm: lesen mit fleet.read, schreiben mit
// fleet.write. Datumsangaben als JJJJ-MM-TT; "heute" ist der Kalendertag in
// Europe/Berlin.

// scheduleUpdateFields: PUT ersetzt die Planung vollstaendig - ein
// vergessenes "aktiv" legte den Termin sonst still, ein vergessenes
// "intervall_monate" nahm ihm den Vorschlag.
var scheduleUpdateFields = []string{"starttermin", "start_km", "intervall_monate", "intervall_km", "notiz", "aktiv"}

func parseUUIDParam(w http.ResponseWriter, req *http.Request, name, message string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(req, name))
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", message)
		return uuid.Nil, false
	}
	return id, true
}

// optionalDate liest ein optionales Datum aus einem Body-Feld (nil oder
// leer = keines). Bei false ist die Antwort bereits geschrieben.
func optionalDate(w http.ResponseWriter, req *http.Request, name string, v *string) (*time.Time, bool) {
	if v == nil {
		return nil, true
	}
	return parseOptionalDate(w, req, name, *v)
}

func mountFleetScheduleRoutes(protected chi.Router, svc *fleet.ScheduleService) {
	read := requirePermission(permFleetRead)
	write := requirePermission(permFleetWrite)

	// Termine eines Fahrzeugs mit Stand; include_inactive=true zeigt
	// stillgelegte, warn_tage die Vorwarnzeit (Standard 30).
	protected.With(read).Get("/fleet/vehicles/{id}/schedules", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseVehicleID(w, req)
		if !ok {
			return
		}
		warn, ok := parseWarnDays(w, req, fleet.DefaultScheduleWarnDays)
		if !ok {
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.ListSchedules(req.Context(), id, req.URL.Query().Get("include_inactive") == "true", hr.WorkDateOf(time.Now()), warn, companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})

	protected.Route("/fleet/schedules", func(r chi.Router) {
		// Betriebsweite Faelligkeitsliste: gesperrt, ueberfaellig, faellig
		// bald - das Dringendste zuerst.
		r.With(read).Get("/due", func(w http.ResponseWriter, req *http.Request) {
			warn, ok := parseWarnDays(w, req, fleet.DefaultScheduleWarnDays)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.DueList(req.Context(), hr.WorkDateOf(time.Now()), warn, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var b struct {
				VehicleID       uuid.UUID `json:"vehicle_id"`
				Art             string    `json:"art"`
				Bezeichnung     string    `json:"bezeichnung"`
				Starttermin     *string   `json:"starttermin"`
				StartKm         *int      `json:"start_km"`
				IntervallMonate *int      `json:"intervall_monate"`
				IntervallKm     *int      `json:"intervall_km"`
				Notiz           string    `json:"notiz"`
			}
			if err := json.NewDecoder(req.Body).Decode(&b); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			start, ok := optionalDate(w, req, "starttermin", b.Starttermin)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.CreateSchedule(req.Context(), fleet.ScheduleInput{
				VehicleID: b.VehicleID, Art: b.Art, Bezeichnung: b.Bezeichnung, Starttermin: start, StartKm: b.StartKm,
				IntervallMonate: b.IntervallMonate, IntervallKm: b.IntervallKm, Notiz: b.Notiz,
			}, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		r.With(write).Put("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Termin-ID")
			if !ok {
				return
			}
			var b struct {
				Starttermin     *string `json:"starttermin"`
				StartKm         *int    `json:"start_km"`
				IntervallMonate *int    `json:"intervall_monate"`
				IntervallKm     *int    `json:"intervall_km"`
				Notiz           string  `json:"notiz"`
				Aktiv           bool    `json:"aktiv"`
			}
			if !decodeCompleteReplacement(w, req, scheduleUpdateFields, "unbekanntes Feld: code", &b) {
				return
			}
			start, ok := optionalDate(w, req, "starttermin", b.Starttermin)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.UpdateSchedule(req.Context(), id, fleet.ScheduleUpdate{
				Starttermin: start, StartKm: b.StartKm, IntervallMonate: b.IntervallMonate, IntervallKm: b.IntervallKm, Notiz: b.Notiz, Aktiv: b.Aktiv,
			}, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Erledigungen eines Termins, die juengste zuerst;
		// include_storniert=true zeigt stornierte.
		r.With(read).Get("/{id}/completions", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Termin-ID")
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ListCompletions(req.Context(), id, req.URL.Query().Get("include_storniert") == "true", companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})

	protected.Route("/fleet/completions", func(r chi.Router) {
		// Erledigung erfassen. Ohne naechster_termin schlaegt der Server
		// den Folgetermin aus dem Intervall vor (nicht nach HU/UVV mit
		// Maengeln oder nicht bestanden).
		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var b struct {
				ScheduleID      uuid.UUID `json:"schedule_id"`
				ErledigtAm      string    `json:"erledigt_am"`
				Ergebnis        string    `json:"ergebnis"`
				KmStand         *int      `json:"km_stand"`
				NaechsterTermin *string   `json:"naechster_termin"`
				NaechsterKm     *int      `json:"naechster_km"`
				Pruefer         string    `json:"pruefer"`
				Notiz           string    `json:"notiz"`
			}
			if err := json.NewDecoder(req.Body).Decode(&b); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			erledigt, ok := parseOptionalDate(w, req, "erledigt_am", b.ErledigtAm)
			if !ok {
				return
			}
			naechster, ok := optionalDate(w, req, "naechster_termin", b.NaechsterTermin)
			if !ok {
				return
			}
			in := fleet.CompletionInput{
				ScheduleID: b.ScheduleID, Ergebnis: b.Ergebnis, KmStand: b.KmStand, NaechsterTermin: naechster,
				NaechsterKm: b.NaechsterKm, Pruefer: b.Pruefer, Notiz: b.Notiz,
			}
			if erledigt != nil {
				in.ErledigtAm = *erledigt // fehlt es, meldet der Service "Datum erforderlich"
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.RecordCompletion(req.Context(), in, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		r.With(write).Post("/{id}/storno", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Erledigungs-ID")
			if !ok {
				return
			}
			var b struct {
				Reason string `json:"reason"`
			}
			if err := json.NewDecoder(req.Body).Decode(&b); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.StornoCompletion(req.Context(), id, b.Reason, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
