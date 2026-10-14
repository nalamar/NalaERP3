package apihttp

import (
	"encoding/json"
	"net/http"

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
