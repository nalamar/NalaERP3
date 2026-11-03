package apihttp

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"nalaerp3/internal/purchasing"
)

// HTTP-Anbindung der Lieferpapiere an der Bestellung (ADR 0035 §4/§6,
// Backlog H.4.3b). Lesen mit purchase_orders.read, hochladen und
// stornieren mit purchase_orders.write. Dateien bewusst nicht ueber
// /documents/{id}.

func mountPurchaseOrderPaperRoutes(protected chi.Router, svc *purchasing.PaperService) {
	read := requirePermission("purchase_orders.read")
	write := requirePermission("purchase_orders.write")

	protected.Route("/purchase-orders/{id}/papers", func(r chi.Router) {
		// include_storniert=true zeigt auch stornierte.
		r.With(read).Get("/", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.List(req.Context(), chi.URLParam(req, "id"), req.URL.Query().Get("include_storniert") == "true", companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Multipart: Datei im Feld "file", dazu art, position_id, charge,
		// notiz als Formularfelder.
		r.With(write).Post("/", func(w http.ResponseWriter, req *http.Request) {
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
			var itemID *string
			if v := req.FormValue("position_id"); v != "" {
				itemID = &v
			}
			in := purchasing.DeliveryPaperInput{Art: req.FormValue("art"), ItemID: itemID, Charge: req.FormValue("charge"), Notiz: req.FormValue("notiz")}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Upload(req.Context(), chi.URLParam(req, "id"), in, file, header.Filename, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		r.With(write).Post("/{paperID}/storno", func(w http.ResponseWriter, req *http.Request) {
			paperID, ok := parseUUIDParam(w, req, "paperID", "Ungültige Lieferpapier-ID")
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
			out, err := svc.Storno(req.Context(), chi.URLParam(req, "id"), paperID, in.Reason, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(read).Get("/{paperID}", func(w http.ResponseWriter, req *http.Request) {
			paperID, ok := parseUUIDParam(w, req, "paperID", "Ungültige Lieferpapier-ID")
			if !ok {
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			rc, p, err := svc.Open(req.Context(), chi.URLParam(req, "id"), paperID, companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			defer rc.Close()
			w.Header().Set("Content-Type", p.ContentType)
			w.Header().Set("Content-Disposition", "attachment; filename=\""+sanitizeFilename(p.Filename)+"\"")
			w.Header().Set("Content-Length", strconv.FormatInt(p.Length, 10))
			_, _ = io.Copy(w, rc)
		})
	})
}
