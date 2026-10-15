package apihttp

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"nalaerp3/internal/hr"
)

// HTTP-Anbindung der Qualifikationen (ADR 0027, Backlog F.2.5).
//
// Rechte:
//   - qualifications.self: eigene Nachweise und eigenen Stand lesen.
//   - qualifications.read: Nachweise und Stand aller Mitarbeiter lesen.
//   - qualifications.write: Nachweise erfassen und stornieren,
//     Anforderungen und Katalog pflegen.
//
// Den Katalog (die Namen der Arten) darf lesen, wer eines der drei Rechte
// hat: wer seine eigenen Nachweise sieht, muss wissen, wofuer sie stehen.

const (
	permQualificationsSelf  = "qualifications.self"
	permQualificationsRead  = "qualifications.read"
	permQualificationsWrite = "qualifications.write"
)

// requireAnyPermission laesst durch, wer mindestens eines der Rechte hat.
func requireAnyPermission(permissions ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			for _, p := range permissions {
				if hasPermission(req, p) {
					next.ServeHTTP(w, req)
					return
				}
			}
			writeAPIError(w, req, http.StatusForbidden, "forbidden", "Keine Berechtigung")
		})
	}
}

func parseQualificationTypeID(w http.ResponseWriter, req *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(req, "id"))
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Qualifikationsart-ID")
		return uuid.Nil, false
	}
	return id, true
}

// qualificationTypeUpdateFields sind die Felder, die PUT verlangt - ALLE.
// PUT ersetzt die Art vollstaendig (siehe hr.QualificationTypeUpdate). Ein
// fehlendes Feld waere sonst nicht von einem absichtlich leeren zu
// unterscheiden: ein vergessenes "aktiv" legte die Art still, ein
// vergessenes "gueltigkeit_monate" machte sie unbefristet - beides ohne
// jede Rueckmeldung.
var qualificationTypeUpdateFields = []string{"name", "kategorie", "gueltigkeit_monate", "pflicht_fuer_alle", "aktiv", "hinweis"}

// decodeQualificationTypeUpdate liest den Body eines PUT. Bei false ist die
// Antwort bereits geschrieben.
func decodeQualificationTypeUpdate(w http.ResponseWriter, req *http.Request) (hr.QualificationTypeUpdate, bool) {
	var out hr.QualificationTypeUpdate
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(req.Body).Decode(&raw); err != nil || raw == nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
		return out, false
	}
	if _, ok := raw["code"]; ok {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Der Code einer Qualifikationsart ist nicht änderbar")
		return out, false
	}
	allowed := make(map[string]bool, len(qualificationTypeUpdateFields))
	for _, f := range qualificationTypeUpdateFields {
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

func mountQualificationTypeRoutes(protected chi.Router, svc *hr.QualificationTypeService) {
	readAny := requireAnyPermission(permQualificationsSelf, permQualificationsRead, permQualificationsWrite)
	write := requirePermission(permQualificationsWrite)

	protected.Route("/qualification-types", func(r chi.Router) {
		r.With(readAny).Get("/", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.List(req.Context(), companyID, req.URL.Query().Get("include_inactive") == "true")
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in hr.QualificationTypeInput
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Create(req.Context(), in, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		// Den Vorschlagskatalog in den eigenen Katalog uebernehmen. Legt nur
		// an, was fehlt, und ueberschreibt nichts; liefert die NEU
		// angelegten Arten (bei einem zweiten Aufruf eine leere Liste).
		r.With(write).Post("/adopt-defaults", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.AdoptDefaults(req.Context(), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(readAny).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseQualificationTypeID(w, req)
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
			id, ok := parseQualificationTypeID(w, req)
			if !ok {
				return
			}
			in, ok := decodeQualificationTypeUpdate(w, req)
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
	})
}

// parseOptionalDate liest ein Datum im Format JJJJ-MM-TT. Ein leerer Wert
// ergibt nil. Bei false ist die Antwort bereits geschrieben.
func parseOptionalDate(w http.ResponseWriter, req *http.Request, name, value string) (*time.Time, bool) {
	if value == "" {
		return nil, true
	}
	d, err := time.Parse("2006-01-02", value)
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültiges Datum in "+name+" (erwartet JJJJ-MM-TT)")
		return nil, false
	}
	return &d, true
}

// optionalEmployeeID liest den Query-Parameter employee_id. Bei false ist
// die Antwort bereits geschrieben.
func optionalEmployeeID(w http.ResponseWriter, req *http.Request) (*uuid.UUID, bool) {
	v := req.URL.Query().Get("employee_id")
	if v == "" {
		return nil, true
	}
	id, err := uuid.Parse(v)
	if err != nil {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige employee_id")
		return nil, false
	}
	return &id, true
}

// mountQualificationRoutes haengt Nachweise und Anforderungen ein.
//
// Nachweise erfasst und storniert nur, wer qualifications.write hat - es
// gibt keinen Selbsteintrag (Vorgabe des Nutzers vom 2026-10-13: "Die
// Personalabteilung traegt ein, welche Qualifikationen erreicht sind").
// Lesen nach dem Muster der Zeiterfassung: eigene mit self, fremde mit
// read (resolveOwnOrPermittedEmployee).
func mountQualificationRoutes(protected chi.Router, svc *hr.QualificationService, resolver employeeForUser) {
	write := requirePermission(permQualificationsWrite)

	protected.Route("/qualifications", func(r chi.Router) {
		// Einen Nachweis fuer einen oder mehrere Mitarbeiter erfassen -
		// ganz oder gar nicht. Liefert je Mitarbeiter den erfassten Nachweis.
		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				EmployeeIDs []uuid.UUID `json:"employee_ids"`
				TypeID      uuid.UUID   `json:"type_id"`
				ErworbenAm  string      `json:"erworben_am"`
				GueltigBis  string      `json:"gueltig_bis"`
				Aussteller  string      `json:"aussteller"`
				NachweisNr  string      `json:"nachweis_nr"`
				Note        string      `json:"note"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			erworben, ok := parseOptionalDate(w, req, "erworben_am", in.ErworbenAm)
			if !ok {
				return
			}
			gueltigBis, ok := parseOptionalDate(w, req, "gueltig_bis", in.GueltigBis)
			if !ok {
				return
			}
			input := hr.QualificationInput{EmployeeIDs: in.EmployeeIDs, TypeID: in.TypeID, GueltigBis: gueltigBis, Aussteller: in.Aussteller, NachweisNr: in.NachweisNr, Note: in.Note}
			if erworben != nil {
				input.ErworbenAm = *erworben // fehlt es, meldet der Service "Erwerbsdatum erforderlich"
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Create(req.Context(), input, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		// Nachweise eines Mitarbeiters; ohne employee_id die eigenen.
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			requested, ok := optionalEmployeeID(w, req)
			if !ok {
				return
			}
			employeeID, ok := resolveOwnOrPermittedEmployee(w, req, resolver, requested, permQualificationsRead, permQualificationsSelf)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ListForEmployee(req.Context(), employeeID, req.URL.Query().Get("include_storniert") == "true", companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Anforderungen eines Mitarbeiters: was seine Stelle verlangt.
		// Lesen wie die Nachweise; ohne employee_id die eigenen.
		r.Get("/requirements", func(w http.ResponseWriter, req *http.Request) {
			requested, ok := optionalEmployeeID(w, req)
			if !ok {
				return
			}
			employeeID, ok := resolveOwnOrPermittedEmployee(w, req, resolver, requested, permQualificationsRead, permQualificationsSelf)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ListRequirements(req.Context(), employeeID, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Anforderungen setzen: ersetzt die Menge vollstaendig. Die
		// employee_id ist hier Pflicht - geaendert wird nie "die eigene"
		// Stelle, sondern immer ausdruecklich die eines Mitarbeiters.
		r.With(write).Put("/requirements", func(w http.ResponseWriter, req *http.Request) {
			requested, ok := optionalEmployeeID(w, req)
			if !ok {
				return
			}
			if requested == nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "employee_id erforderlich")
				return
			}
			var in struct {
				TypeIDs *[]uuid.UUID `json:"type_ids"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			// Eine fehlende Liste ist nicht dasselbe wie eine leere: ein
			// vergessenes Feld wuerde sonst alle Anforderungen entfernen.
			if in.TypeIDs == nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Feld erforderlich: type_ids")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.SetRequirements(req.Context(), *requested, *in.TypeIDs, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Stand eines Mitarbeiters - ohne employee_id der eigene: was fehlt,
		// abgelaufen ist, ablaeuft oder gilt, das Dringendste zuerst.
		r.Get("/standing", func(w http.ResponseWriter, req *http.Request) {
			requested, ok := optionalEmployeeID(w, req)
			if !ok {
				return
			}
			warnDays, ok := parseWarnDays(w, req, hr.DefaultQualificationWarnDays)
			if !ok {
				return
			}
			employeeID, ok := resolveOwnOrPermittedEmployee(w, req, resolver, requested, permQualificationsRead, permQualificationsSelf)
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Standing(req.Context(), employeeID, hr.WorkDateOf(time.Now()), warnDays, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Betriebsweite Liste: alles, was bei aktiven Mitarbeitern fehlt,
		// abgelaufen ist oder ablaeuft - die Arbeitsliste der
		// Personalabteilung zum Anmahnen (Vorgabe des Nutzers vom
		// 2026-10-13). Enthaelt die Namen aller Mitarbeiter und braucht
		// deshalb qualifications.read; self genuegt nicht.
		r.With(requirePermission(permQualificationsRead)).Get("/due", func(w http.ResponseWriter, req *http.Request) {
			warnDays, ok := parseWarnDays(w, req, hr.DefaultQualificationWarnDays)
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

		r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Nachweis-ID")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Get(req.Context(), id, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			if _, ok := resolveOwnOrPermittedEmployee(w, req, resolver, &out.EmployeeID, permQualificationsRead, permQualificationsSelf); !ok {
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(write).Post("/{id}/storno", func(w http.ResponseWriter, req *http.Request) {
			id, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Nachweis-ID")
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
			out, err := svc.Storno(req.Context(), id, in.Reason, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}

// parseWarnDays liest die Vorwarnzeit in Tagen (warn_tage); ohne Angabe
// defaultDays - der Standard des jeweiligen Bereichs (Qualifikationen 60,
// Pruefungen der Betriebsmittel 30). Bei false ist die Antwort bereits
// geschrieben.
func parseWarnDays(w http.ResponseWriter, req *http.Request, defaultDays int) (int, bool) {
	v := req.URL.Query().Get("warn_tage")
	if v == "" {
		return defaultDays, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Vorwarnzeit (warn_tage: ganze Zahl ab 0)")
		return 0, false
	}
	return n, true
}
