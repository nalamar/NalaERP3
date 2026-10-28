package apihttp

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/fleet"
)

// HTTP-Anbindung des Fuhrparks (ADR 0032, Backlog G.1.3): Fahrzeugstamm.
//
// Rechte:
//   - fleet.read: Fahrzeuge lesen.
//   - fleet.write: Fahrzeuge pflegen und abmelden.
//
// read/write haben nur Fuhrpark und Administrator, nicht die
// Personalabteilung (Entscheidung des Nutzers vom 2026-10-25). fleet.self
// (alle Systemrollen) zeigt die eigenen Zuordnungen (G.3a.2), spaeter die
// eigenen Fahrten (G.3c).

const (
	permFleetSelf  = "fleet.self"
	permFleetRead  = "fleet.read"
	permFleetWrite = "fleet.write"
)

func parseVehicleID(w http.ResponseWriter, req *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(req, "id"))
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Fahrzeug-ID")
		return uuid.Nil, false
	}
	return id, true
}

// vehicleBody ist der Body von POST und PUT. Datumsangaben als JJJJ-MM-TT.
type vehicleBody struct {
	Kennzeichen             string  `json:"kennzeichen"`
	Art                     string  `json:"art"`
	Bezeichnung             string  `json:"bezeichnung"`
	Hersteller              string  `json:"hersteller"`
	Modell                  string  `json:"modell"`
	FIN                     *string `json:"fin"`
	Erstzulassung           *string `json:"erstzulassung"`
	ZulaessigeGesamtmasseKg *int    `json:"zulaessige_gesamtmasse_kg"`
	Standort                string  `json:"standort"`
	KostenstelleID          *string `json:"kostenstelle_id"`
	Fahrtenbuch             bool    `json:"fahrtenbuch"`
	Notiz                   string  `json:"notiz"`
}

// vehicleUpdateFields: PUT ersetzt die Stammdaten vollstaendig und
// verlangt ALLE Felder - ein vergessenes "fahrtenbuch" schaltete sonst das
// Fahrtenbuch still ab, eine vergessene "kostenstelle_id" entfernte die
// Zuordnung. Die Abmeldung hat eine eigene Route.
var vehicleUpdateFields = []string{"kennzeichen", "art", "bezeichnung", "hersteller", "modell", "fin", "erstzulassung",
	"zulaessige_gesamtmasse_kg", "standort", "kostenstelle_id", "fahrtenbuch", "notiz"}

// toInput wandelt den Body in die Eingabe des Service. Bei false ist die
// Antwort bereits geschrieben.
func (b vehicleBody) toInput(w http.ResponseWriter, req *http.Request) (fleet.VehicleInput, bool) {
	in := fleet.VehicleInput{
		Kennzeichen: b.Kennzeichen, Art: b.Art, Bezeichnung: b.Bezeichnung, Hersteller: b.Hersteller, Modell: b.Modell,
		FIN: b.FIN, ZulaessigeGesamtmasseKg: b.ZulaessigeGesamtmasseKg, Standort: b.Standort,
		KostenstelleID: b.KostenstelleID, Fahrtenbuch: b.Fahrtenbuch, Notiz: b.Notiz,
	}
	if b.Erstzulassung != nil {
		d, ok := parseOptionalDate(w, req, "erstzulassung", *b.Erstzulassung)
		if !ok {
			return in, false
		}
		in.Erstzulassung = d
	}
	return in, true
}

func mountFleetRoutes(protected chi.Router, svc *fleet.VehicleService) {
	read := requirePermission(permFleetRead)
	write := requirePermission(permFleetWrite)

	protected.Route("/fleet/vehicles", func(r chi.Router) {
		// Fahrzeuge des Mandanten nach Kennzeichen. Filter: art;
		// include_abgemeldete=true zeigt auch abgemeldete.
		r.With(read).Get("/", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			q := req.URL.Query()
			out, err := svc.List(req.Context(), companyID, fleet.ListFilter{
				Art:                q.Get("art"),
				IncludeAbgemeldete: q.Get("include_abgemeldete") == "true",
			})
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var b vehicleBody
			if err := json.NewDecoder(req.Body).Decode(&b); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			in, ok := b.toInput(w, req)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Create(req.Context(), in, companyID, actorUserIDFromContext(req.Context()))
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		r.With(read).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseVehicleID(w, req)
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

		r.With(write).Put("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseVehicleID(w, req)
			if !ok {
				return
			}
			var b vehicleBody
			if !decodeCompleteReplacement(w, req, vehicleUpdateFields, "unbekanntes Feld: code", &b) {
				return
			}
			in, ok := b.toInput(w, req)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Update(req.Context(), id, in, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Abmelden: einmalig, mit Datum (JJJJ-MM-TT) und Grund.
		r.With(write).Post("/{id}/deregister", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseVehicleID(w, req)
			if !ok {
				return
			}
			var in struct {
				Datum string `json:"datum"`
				Grund string `json:"grund"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			datum, ok := parseOptionalDate(w, req, "datum", in.Datum)
			if !ok {
				return
			}
			var d time.Time
			if datum != nil {
				d = *datum // fehlt es, meldet der Service "Abmeldedatum erforderlich"
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Deregister(req.Context(), id, d, in.Grund, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
