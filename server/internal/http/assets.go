package apihttp

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/assets"
)

// HTTP-Anbindung der Betriebsmittel (ADR 0028, Backlog F.3.6).
//
// Nicht verwechseln: /projects/{id}/assets sind Projekt-DATEIEN; /assets
// sind Werkzeug, Leitern, IT, PSA.
//
// Rechte:
//   - assets.self: die eigenen Ausgaben lesen (F.3.6.2).
//   - assets.read: Bestand, Ausgaben, Pruefungen und Faelligkeiten lesen.
//   - assets.write: Bestand pflegen, ausmustern, ausgeben, zuruecknehmen,
//     Pruefungen erfassen, stornieren.
//
// read/write haben vorerst nur Personalabteilung und Administrator
// (Entscheidung des Nutzers vom 2026-10-15).

const (
	permAssetsSelf  = "assets.self"
	permAssetsRead  = "assets.read"
	permAssetsWrite = "assets.write"
)

func parseAssetID(w http.ResponseWriter, req *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(req, "id"))
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Asset-ID")
		return uuid.Nil, false
	}
	return id, true
}

// assetBody ist der Body von POST und PUT. Datumsangaben als JJJJ-MM-TT.
type assetBody struct {
	Inventarnummer       string   `json:"inventarnummer"`
	Bezeichnung          string   `json:"bezeichnung"`
	Kategorie            string   `json:"kategorie"`
	Seriennummer         string   `json:"seriennummer"`
	Hersteller           string   `json:"hersteller"`
	Modell               string   `json:"modell"`
	Standort             string   `json:"standort"`
	Anschaffungsdatum    *string  `json:"anschaffungsdatum"`
	Anschaffungspreis    *float64 `json:"anschaffungspreis"`
	KostenstelleID       *string  `json:"kostenstelle_id"`
	PruefintervallMonate *int     `json:"pruefintervall_monate"`
	Notiz                string   `json:"notiz"`
}

// assetUpdateFields sind die Felder, die PUT verlangt - ALLE. PUT ersetzt
// die Stammdaten vollstaendig (siehe assets.AssetInput); ein fehlendes Feld
// waere sonst nicht von einem absichtlich leeren zu unterscheiden - ein
// vergessenes "pruefintervall_monate" machte ein Elektrowerkzeug still
// pruefungsfrei. Die Ausmusterung ist kein Feld, sie hat eine eigene Route.
var assetUpdateFields = []string{"inventarnummer", "bezeichnung", "kategorie", "seriennummer", "hersteller", "modell", "standort",
	"anschaffungsdatum", "anschaffungspreis", "kostenstelle_id", "pruefintervall_monate", "notiz"}

// toInput wandelt den Body in die Eingabe des Service. Bei false ist die
// Antwort bereits geschrieben.
func (b assetBody) toInput(w http.ResponseWriter, req *http.Request) (assets.AssetInput, bool) {
	in := assets.AssetInput{
		Inventarnummer: b.Inventarnummer, Bezeichnung: b.Bezeichnung, Kategorie: b.Kategorie,
		Seriennummer: b.Seriennummer, Hersteller: b.Hersteller, Modell: b.Modell, Standort: b.Standort,
		Anschaffungspreis: b.Anschaffungspreis, KostenstelleID: b.KostenstelleID, PruefintervallMonate: b.PruefintervallMonate, Notiz: b.Notiz,
	}
	if b.Anschaffungsdatum != nil {
		d, ok := parseOptionalDate(w, req, "anschaffungsdatum", *b.Anschaffungsdatum)
		if !ok {
			return in, false
		}
		in.Anschaffungsdatum = d
	}
	return in, true
}

// decodeAssetUpdate liest den Body eines PUT. Bei false ist die Antwort
// bereits geschrieben.
func decodeAssetUpdate(w http.ResponseWriter, req *http.Request) (assetBody, bool) {
	var out assetBody
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(req.Body).Decode(&raw); err != nil || raw == nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
		return out, false
	}
	allowed := make(map[string]bool, len(assetUpdateFields))
	for _, f := range assetUpdateFields {
		allowed[f] = true
		if _, ok := raw[f]; !ok {
			writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Feld erforderlich: "+f)
			return out, false
		}
	}
	for k := range raw {
		if !allowed[k] {
			writeAPIError(w, req, http.StatusBadRequest, "validation_error", "unbekanntes Feld: "+k)
			return out, false
		}
	}
	body, err := json.Marshal(raw)
	if err == nil {
		err = json.Unmarshal(body, &out)
	}
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
		return out, false
	}
	return out, true
}

func mountAssetRoutes(protected chi.Router, svc *assets.Service) {
	read := requirePermission(permAssetsRead)
	write := requirePermission(permAssetsWrite)

	protected.Route("/assets", func(r chi.Router) {
		// Bestand des Mandanten, nach Inventarnummer. Filter: kategorie;
		// include_ausgemustert=true zeigt auch ausgemusterte.
		r.With(read).Get("/", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			q := req.URL.Query()
			out, err := svc.List(req.Context(), companyID, assets.ListFilter{
				Kategorie:           q.Get("kategorie"),
				IncludeAusgemustert: q.Get("include_ausgemustert") == "true",
			})
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var b assetBody
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
			id, ok := parseAssetID(w, req)
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
			id, ok := parseAssetID(w, req)
			if !ok {
				return
			}
			b, ok := decodeAssetUpdate(w, req)
			if !ok {
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

		// Ausmustern: endgueltig, mit Datum und Grund. Abgelehnt, solange
		// das Stueck ausgegeben ist.
		r.With(write).Post("/{id}/retire", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseAssetID(w, req)
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
				d = *datum // fehlt es, meldet der Service "Ausmusterungsdatum erforderlich"
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Retire(req.Context(), id, d, in.Grund, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
