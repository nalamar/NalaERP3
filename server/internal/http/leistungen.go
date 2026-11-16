package apihttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"nalaerp3/internal/leistungen"
)

// HTTP-Anbindung des Leistungskatalogs (ADR 0044, Backlog I.4.4b). Lesen mit
// quotes.read, Anlegen, Aendern und Import mit leistungen.write (Migration
// 121). Abschalten statt Loeschen: PUT mit "aktiv": false.

// maxLeistungenImportBytes begrenzt eine Importdatei (JSON oder CSV).
const maxLeistungenImportBytes = 5 << 20

func mountLeistungenRoutes(protected chi.Router, svc *leistungen.Service) {
	protected.Route("/leistungen", func(r chi.Router) {
		// Liste: q (Code oder Kurztext), aktiv=1 (nur aktive), limit
		// (Standard 200, hoechstens 1000), offset.
		r.With(requirePermission("quotes.read")).Get("/", func(w http.ResponseWriter, req *http.Request) {
			q := req.URL.Query()
			f := leistungen.Filter{Q: q.Get("q"), NurAktive: q.Get("aktiv") == "1" || q.Get("aktiv") == "true"}
			for name, ziel := range map[string]*int{"limit": &f.Limit, "offset": &f.Offset} {
				if v := q.Get(name); v != "" {
					n, err := strconv.Atoi(v)
					if err != nil || n < 0 {
						writeAPIError(w, req, http.StatusBadRequest, "validation_error", name+" muss eine Zahl ab 0 sein")
						return
					}
					*ziel = n
				}
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Liste(req.Context(), f, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(requirePermission("quotes.read")).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Lesen(req.Context(), chi.URLParam(req, "id"), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(requirePermission("leistungen.write")).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in leistungen.Eingabe
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Anlegen(req.Context(), in, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		r.With(requirePermission("leistungen.write")).Put("/{id}", func(w http.ResponseWriter, req *http.Request) {
			var in leistungen.Eingabe
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Aendern(req.Context(), chi.URLParam(req, "id"), in, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Import: multipart/form-data mit CSV-Datei im Feld "file" (Kopfzeile
		// mit den Feldnamen, Semikolon; der Router laesst nur JSON und
		// Multipart zu), sonst JSON {"leistungen": [...]}. Ganz oder gar
		// nicht; Meldungen nennen die Zeile (CSV: Zeile der Datei).
		r.With(requirePermission("leistungen.write")).Post("/import", func(w http.ResponseWriter, req *http.Request) {
			req.Body = http.MaxBytesReader(w, req.Body, maxLeistungenImportBytes)
			var (
				zeilen  []leistungen.Eingabe
				nummern []int
				err     error
			)
			if strings.HasPrefix(strings.ToLower(req.Header.Get("Content-Type")), "multipart/form-data") {
				if err = req.ParseMultipartForm(1 << 20); err == nil {
					file, _, ferr := req.FormFile("file")
					if ferr != nil {
						writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Datei fehlt (Feld 'file')")
						return
					}
					defer file.Close()
					zeilen, nummern, err = leistungen.LeseCSV(file)
				}
			} else {
				var body struct {
					Leistungen []leistungen.Eingabe `json:"leistungen"`
				}
				dec := json.NewDecoder(req.Body)
				dec.DisallowUnknownFields()
				if err = dec.Decode(&body); err != nil {
					err = errors.New("Ungültige Eingabe: " + err.Error())
				}
				zeilen = body.Leistungen
			}
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) || (err != nil && strings.Contains(err.Error(), "request body too large")) {
				writeAPIError(w, req, http.StatusRequestEntityTooLarge, "validation_error", "Importdatei ist größer als 5 MB")
				return
			}
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", err.Error())
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Importieren(req.Context(), zeilen, nummern, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
