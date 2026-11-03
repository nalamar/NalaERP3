package apihttp

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"nalaerp3/internal/purchasing"
)

// HTTP-Anbindung der Zertifikate am Lieferanten (ADR 0035 §1/§6, Backlog
// H.4.2b). Lesen mit purchase_orders.read, anlegen, stornieren und Dateien
// hochladen mit purchase_orders.write. Eigener Pfad statt
// /contacts/{id}/...: die Kontaktrouten haengen an contacts.*, die
// Nachweise am Einkauf. Dateien bewusst nicht ueber /documents/{id}.

const supplierCertificatesPath = "/supplier-certificates"

func mountSupplierCertificateRoutes(protected chi.Router, svc *purchasing.CertificateService) {
	read := requirePermission("purchase_orders.read")
	write := requirePermission("purchase_orders.write")

	protected.Route(supplierCertificatesPath, func(r chi.Router) {
		// Liste je Lieferant: lieferant_id Pflicht, include_storniert=true
		// zeigt auch stornierte.
		r.With(read).Get("/", func(w http.ResponseWriter, req *http.Request) {
			q := req.URL.Query()
			supplierID := strings.TrimSpace(q.Get("lieferant_id"))
			if supplierID == "" {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Lieferant erforderlich (lieferant_id)")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.List(req.Context(), supplierID, q.Get("include_storniert") == "true", companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				SupplierID string  `json:"lieferant_id"`
				Art        string  `json:"art"`
				Nummer     string  `json:"nummer"`
				Stelle     string  `json:"stelle"`
				ExcBis     *string `json:"exc_bis"`
				GueltigVon string  `json:"gueltig_von"`
				GueltigBis string  `json:"gueltig_bis"`
				Notiz      string  `json:"notiz"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			if strings.TrimSpace(in.SupplierID) == "" {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Lieferant erforderlich (lieferant_id)")
				return
			}
			von, ok := parseOptionalDate(w, req, "gueltig_von", strings.TrimSpace(in.GueltigVon))
			if !ok {
				return
			}
			bis, ok := parseOptionalDate(w, req, "gueltig_bis", strings.TrimSpace(in.GueltigBis))
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Create(req.Context(), in.SupplierID, purchasing.CertificateInput{Art: in.Art, Nummer: in.Nummer, Stelle: in.Stelle,
				ExcBis: in.ExcBis, GueltigVon: von, GueltigBis: bis, Notiz: in.Notiz}, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		r.With(read).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Zertifikat-ID")
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

		r.With(write).Post("/{id}/storno", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Zertifikat-ID")
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
			out, err := svc.Storno(req.Context(), id, in.Reason, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(write).Post("/{id}/documents", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Zertifikat-ID")
			if !ok {
				return
			}
			// Etwas Luft fuer die Multipart-Huelle; die Datei selbst prueft
			// der Service auf 10 MB.
			req.Body = http.MaxBytesReader(w, req.Body, purchasing.MaxEvidenceDocumentBytes+(1<<20))
			if err := req.ParseMultipartForm(1 << 20); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültiges Upload-Formular oder Datei zu groß")
				return
			}
			file, header, err := req.FormFile("file")
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Datei fehlt (Feld 'file')")
				return
			}
			defer file.Close()
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.UploadDocument(req.Context(), id, file, header.Filename, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		r.With(read).Get("/{id}/documents", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Zertifikat-ID")
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.ListDocuments(req.Context(), id, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(read).Get("/{id}/documents/{docID}", func(w http.ResponseWriter, req *http.Request) {
			id, ok := parseUUIDParam(w, req, "id", "Ungültige Zertifikat-ID")
			if !ok {
				return
			}
			docID, ok := parseUUIDParam(w, req, "docID", "Ungültige Dokument-ID")
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			rc, d, err := svc.OpenDocument(req.Context(), id, docID, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			defer rc.Close()
			w.Header().Set("Content-Type", d.ContentType)
			w.Header().Set("Content-Disposition", "attachment; filename=\""+sanitizeFilename(d.Filename)+"\"")
			w.Header().Set("Content-Length", strconv.FormatInt(d.Length, 10))
			_, _ = io.Copy(w, rc)
		})
	})
}
