package apihttp

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/assets"
	"nalaerp3/internal/hr"
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

		// Pruefstand eines Assets heute (Europe/Berlin). Vorwarnzeit ueber
		// warn_tage, Standard assets.DefaultInspectionWarnDays.
		r.With(read).Get("/{id}/inspection-standing", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseAssetID(w, req)
			if !ok {
				return
			}
			warnDays, ok := parseWarnDays(w, req, assets.DefaultInspectionWarnDays)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.InspectionStandingOf(req.Context(), id, hr.WorkDateOf(time.Now()), warnDays, companyID)
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

func parseAssignmentID(w http.ResponseWriter, req *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(req, "id"))
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Ausgabe-ID")
		return uuid.Nil, false
	}
	return id, true
}

// optionalQueryUUID liest einen optionalen UUID-Parameter. Bei false ist
// die Antwort bereits geschrieben.
func optionalQueryUUID(w http.ResponseWriter, req *http.Request, name string) (*uuid.UUID, bool) {
	v := req.URL.Query().Get(name)
	if v == "" {
		return nil, true
	}
	id, err := uuid.Parse(v)
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige "+name)
		return nil, false
	}
	return &id, true
}

// mountAssetAssignmentRoutes haengt die Ausgaben ein (Backlog F.3.6.2).
//
// Lesen: mit assets.read alle Ausgaben des Mandanten; mit assets.self nur
// die eigenen ("was habe ich bekommen?"). Schreiben (ausgeben,
// zuruecknehmen, stornieren) nur mit assets.write - eine Ausgabe ist eine
// Aussage des Betriebs, keine Selbstauskunft.
func mountAssetAssignmentRoutes(protected chi.Router, svc *assets.Service, resolver employeeForUser) {
	write := requirePermission(permAssetsWrite)

	protected.Route("/asset-assignments", func(r chi.Router) {
		// Ausgeben. Die Antwort enthaelt "hinweise" (Pruefung ueberfaellig,
		// nie geprueft, Maengel); ein gesperrtes Stueck wird abgelehnt.
		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				AssetID      uuid.UUID `json:"asset_id"`
				EmployeeID   uuid.UUID `json:"employee_id"`
				AusgegebenAm string    `json:"ausgegeben_am"`
				Notiz        string    `json:"notiz"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			datum, ok := parseOptionalDate(w, req, "ausgegeben_am", in.AusgegebenAm)
			if !ok {
				return
			}
			input := assets.IssueInput{AssetID: in.AssetID, EmployeeID: in.EmployeeID, Notiz: in.Notiz}
			if datum != nil {
				input.AusgegebenAm = *datum // fehlt es, meldet der Service "Ausgabedatum erforderlich"
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Issue(req.Context(), input, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		// Liste, die neuesten zuerst. Filter: asset_id, employee_id,
		// nur_offen=true, include_storniert=true. Ohne assets.read nur die
		// eigenen - eine fremde employee_id ist dann verboten.
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			assetID, ok := optionalQueryUUID(w, req, "asset_id")
			if !ok {
				return
			}
			employeeID, ok := optionalEmployeeID(w, req)
			if !ok {
				return
			}
			if !hasPermission(req, permAssetsRead) {
				own, ok := resolveOwnOrPermittedEmployee(w, req, resolver, employeeID, permAssetsRead, permAssetsSelf)
				if !ok {
					return
				}
				employeeID = &own
			}
			q := req.URL.Query()
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ListAssignments(req.Context(), companyID, assets.AssignmentFilter{
				AssetID: assetID, EmployeeID: employeeID,
				NurOffen: q.Get("nur_offen") == "true", IncludeStorniert: q.Get("include_storniert") == "true",
			})
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseAssignmentID(w, req)
			if !ok {
				return
			}
			if !hasPermission(req, permAssetsRead) && !hasPermission(req, permAssetsSelf) {
				writeAPIError(w, req, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.GetAssignment(req.Context(), id, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			if _, ok := resolveOwnOrPermittedEmployee(w, req, resolver, &out.EmployeeID, permAssetsRead, permAssetsSelf); !ok {
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Zuruecknehmen: einmalig, mit Datum und optionalem Zustand.
		r.With(write).Post("/{id}/return", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseAssignmentID(w, req)
			if !ok {
				return
			}
			var in struct {
				Datum   string `json:"datum"`
				Zustand string `json:"zustand"`
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
				d = *datum // fehlt es, meldet der Service "Rückgabedatum erforderlich"
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Return(req.Context(), id, d, in.Zustand, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(write).Post("/{id}/storno", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseAssignmentID(w, req)
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

// mountAssetInspectionRoutes haengt die Pruefungen und die
// Faelligkeitsliste ein (Backlog F.3.6.3). Nur assets.read bzw.
// assets.write: eine Pruefung betrifft das Stueck, nicht die Person -
// assets.self gibt hier nichts.
func mountAssetInspectionRoutes(protected chi.Router, svc *assets.Service) {
	read := requirePermission(permAssetsRead)
	write := requirePermission(permAssetsWrite)

	protected.Route("/asset-inspections", func(r chi.Router) {
		// Pruefung erfassen. Ohne naechste_pruefung_am wird sie aus dem
		// Pruefintervall des Assets berechnet (bei nicht_bestanden: keine).
		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				AssetID            uuid.UUID  `json:"asset_id"`
				GeprueftAm         string     `json:"geprueft_am"`
				Ergebnis           string     `json:"ergebnis"`
				NaechstePruefungAm string     `json:"naechste_pruefung_am"`
				PrueferName        string     `json:"pruefer_name"`
				PrueferEmployeeID  *uuid.UUID `json:"pruefer_employee_id"`
				Notiz              string     `json:"notiz"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			geprueft, ok := parseOptionalDate(w, req, "geprueft_am", in.GeprueftAm)
			if !ok {
				return
			}
			naechste, ok := parseOptionalDate(w, req, "naechste_pruefung_am", in.NaechstePruefungAm)
			if !ok {
				return
			}
			input := assets.InspectionInput{AssetID: in.AssetID, Ergebnis: in.Ergebnis, NaechstePruefungAm: naechste,
				PrueferName: in.PrueferName, PrueferEmployeeID: in.PrueferEmployeeID, Notiz: in.Notiz}
			if geprueft != nil {
				input.GeprueftAm = *geprueft // fehlt es, meldet der Service "Prüfdatum erforderlich"
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.RecordInspection(req.Context(), input, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		// Pruefungen EINES Assets, die juengste zuerst; asset_id ist
		// Pflicht. include_storniert=true zeigt auch stornierte.
		r.With(read).Get("/", func(w http.ResponseWriter, req *http.Request) {
			assetID, ok := optionalQueryUUID(w, req, "asset_id")
			if !ok {
				return
			}
			if assetID == nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "asset_id erforderlich")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ListInspections(req.Context(), *assetID, req.URL.Query().Get("include_storniert") == "true", companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Betriebsweite Liste: gesperrt, ueberfaellig, nie geprueft, bald
		// faellig - Stichtag heute (Europe/Berlin), Vorwarnzeit warn_tage.
		r.With(read).Get("/due", func(w http.ResponseWriter, req *http.Request) {
			warnDays, ok := parseWarnDays(w, req, assets.DefaultInspectionWarnDays)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.DueList(req.Context(), hr.WorkDateOf(time.Now()), warnDays, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(read).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseInspectionID(w, req)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.GetInspection(req.Context(), id, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(write).Post("/{id}/storno", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseInspectionID(w, req)
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
			out, err := svc.StornoInspection(req.Context(), id, in.Reason, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}

func parseInspectionID(w http.ResponseWriter, req *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(req, "id"))
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Prüfungs-ID")
		return uuid.Nil, false
	}
	return id, true
}
