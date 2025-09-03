package apihttp

import (
    "encoding/json"
    "net/http"
    "io"
    "fmt"
    "strconv"
    "strings"
    "os"
    "log"
    "os/exec"
    "path/filepath"

    "github.com/go-chi/chi/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/gridfs"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/bson/primitive"
    "archive/zip"
    "bytes"
    "nalaerp3/internal/config"
    "nalaerp3/internal/materials"
    "nalaerp3/internal/contacts"
    "nalaerp3/internal/purchasing"
    "nalaerp3/internal/settings"
    "nalaerp3/internal/pdfgen"
    "nalaerp3/internal/projects"
    "time"
)

func NewV1Router(pg *pgxpool.Pool, mg *mongo.Client, rd *redis.Client, cfg *config.Config) http.Handler {
    r := chi.NewRouter()

    matSvc := materials.NewService(pg, mg, cfg.MongoDB)
    conSvc := contacts.NewService(pg)
    poSvc := purchasing.NewService(pg)
    numSvc := settings.NewNumberingService(pg)
    pdfSvc := settings.NewPDFService(pg)
    unitSvc := settings.NewUnitService(pg)
    projSvc := projects.NewService(pg)

    r.Route("/materials", func(r chi.Router) {
        r.Get("/types", func(w http.ResponseWriter, req *http.Request) {
            list, err := matSvc.ListTypes(req.Context())
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            writeJSON(w, http.StatusOK, list)
        })
        r.Get("/categories", func(w http.ResponseWriter, req *http.Request) {
            list, err := matSvc.ListCategories(req.Context())
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            writeJSON(w, http.StatusOK, list)
        })
        r.Post("/", func(w http.ResponseWriter, req *http.Request) {
            var in materials.MaterialCreate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
                http.Error(w, "Ungültige Eingabe", http.StatusBadRequest)
                return
            }
            out, err := matSvc.Create(req.Context(), in)
            if err != nil {
                http.Error(w, err.Error(), http.StatusBadRequest)
                return
            }
            writeJSON(w, http.StatusCreated, out)
        })

        r.Get("/", func(w http.ResponseWriter, req *http.Request) {
            q := req.URL.Query()
            lim := 0; off := 0
            if v := q.Get("limit"); v != "" { if n, err := strconv.Atoi(v); err == nil { lim = n } }
            if v := q.Get("offset"); v != "" { if n, err := strconv.Atoi(v); err == nil { off = n } }
            filter := materials.MaterialFilter{
                Q: q.Get("q"),
                Typ: q.Get("typ"),
                Kategorie: q.Get("kategorie"),
                Limit: lim,
                Offset: off,
            }
            list, err := matSvc.List(req.Context(), filter)
            if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            writeJSON(w, http.StatusOK, list)
        })

        r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            m, err := matSvc.Get(req.Context(), id)
            if err != nil {
                http.Error(w, err.Error(), http.StatusNotFound)
                return
            }
            writeJSON(w, http.StatusOK, m)
        })

        r.Patch("/{id}", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            var in materials.MaterialUpdate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
                http.Error(w, "Ungültige Eingabe", http.StatusBadRequest)
                return
            }
            out, err := matSvc.Update(req.Context(), id, in)
            if err != nil {
                http.Error(w, err.Error(), http.StatusBadRequest)
                return
            }
            writeJSON(w, http.StatusOK, out)
        })

        r.Delete("/{id}", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            if err := matSvc.DeleteSoft(req.Context(), id); err != nil {
                http.Error(w, err.Error(), http.StatusBadRequest)
                return
            }
            w.WriteHeader(http.StatusNoContent)
        })

        r.Get("/{id}/stock", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            res, err := matSvc.StockByMaterial(req.Context(), id)
            if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            writeJSON(w, http.StatusOK, res)
        })

        // Upload Dokument zu Material
        r.Post("/{id}/documents", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            // bis zu 32MB Formdaten im Speicher
            if err := req.ParseMultipartForm(32 << 20); err != nil {
                http.Error(w, "Ungültiges Upload-Formular", http.StatusBadRequest)
                return
            }
            file, header, err := req.FormFile("file")
            if err != nil {
                http.Error(w, "Datei fehlt (Feld 'file')", http.StatusBadRequest)
                return
            }
            defer file.Close()

            contentType := header.Header.Get("Content-Type")
            doc, err := matSvc.UploadMaterialDocument(req.Context(), id, file, header.Filename, contentType)
            if err != nil {
                http.Error(w, err.Error(), http.StatusBadRequest)
                return
            }
            writeJSON(w, http.StatusCreated, doc)
        })

        // Liste Dokumente eines Materials
        r.Get("/{id}/documents", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            docs, err := matSvc.ListMaterialDocuments(req.Context(), id)
            if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            writeJSON(w, http.StatusOK, docs)
        })
    })

    // Kontakte (CRM)
    r.Route("/contacts", func(r chi.Router) {
        r.Get("/", func(w http.ResponseWriter, req *http.Request) {
            q := req.URL.Query()
            lim := 0; off := 0
            if v := q.Get("limit"); v != "" { if n, err := strconv.Atoi(v); err == nil { lim = n } }
            if v := q.Get("offset"); v != "" { if n, err := strconv.Atoi(v); err == nil { off = n } }
            filter := contacts.ContactFilter{ Q: q.Get("q"), Rolle: q.Get("rolle"), Typ: q.Get("typ"), Limit: lim, Offset: off }
            list, err := conSvc.List(req.Context(), filter)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            writeJSON(w, http.StatusOK, list)
        })
        r.Get("/roles", func(w http.ResponseWriter, req *http.Request) { writeJSON(w, http.StatusOK, contacts.Roles()) })
        r.Get("/types", func(w http.ResponseWriter, req *http.Request) { writeJSON(w, http.StatusOK, contacts.Types()) })
        r.Post("/", func(w http.ResponseWriter, req *http.Request) {
            var in contacts.ContactCreate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            out, err := conSvc.Create(req.Context(), in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusCreated, out)
        })
        r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            c, err := conSvc.Get(req.Context(), id)
            if err != nil { http.Error(w, err.Error(), http.StatusNotFound); return }
            writeJSON(w, http.StatusOK, c)
        })
        r.Patch("/{id}", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            var in contacts.ContactUpdate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            out, err := conSvc.Update(req.Context(), id, in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusOK, out)
        })
        r.Delete("/{id}", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            if err := conSvc.DeleteSoft(req.Context(), id); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            w.WriteHeader(http.StatusNoContent)
        })

        // Addresses
        r.Get("/{id}/addresses", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            out, err := conSvc.ListAddresses(req.Context(), id)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            writeJSON(w, http.StatusOK, out)
        })
        r.Post("/{id}/addresses", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            var in contacts.AddressCreate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            out, err := conSvc.CreateAddress(req.Context(), id, in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusCreated, out)
        })
        r.Patch("/{id}/addresses/{addrID}", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            addrID := chi.URLParam(req, "addrID")
            var in contacts.AddressUpdate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            out, err := conSvc.UpdateAddress(req.Context(), id, addrID, in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusOK, out)
        })
        r.Delete("/{id}/addresses/{addrID}", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            addrID := chi.URLParam(req, "addrID")
            if err := conSvc.DeleteAddress(req.Context(), id, addrID); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            w.WriteHeader(http.StatusNoContent)
        })

        // Persons
        r.Get("/{id}/persons", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            out, err := conSvc.ListPersons(req.Context(), id)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            writeJSON(w, http.StatusOK, out)
        })
        r.Post("/{id}/persons", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            var in contacts.PersonCreate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            out, err := conSvc.CreatePerson(req.Context(), id, in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusCreated, out)
        })
        r.Patch("/{id}/persons/{pid}", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            pid := chi.URLParam(req, "pid")
            var in contacts.PersonUpdate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            out, err := conSvc.UpdatePerson(req.Context(), id, pid, in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusOK, out)
        })
        r.Delete("/{id}/persons/{pid}", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            pid := chi.URLParam(req, "pid")
            if err := conSvc.DeletePerson(req.Context(), id, pid); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            w.WriteHeader(http.StatusNoContent)
        })
    })

    // Bestellungen (Purchase Orders)
    r.Route("/purchase-orders", func(r chi.Router) {
        r.Get("/", func(w http.ResponseWriter, req *http.Request) {
            qv := req.URL.Query()
            lim, off := 0, 0
            if v := qv.Get("limit"); v != "" { if n, err := strconv.Atoi(v); err == nil { lim = n } }
            if v := qv.Get("offset"); v != "" { if n, err := strconv.Atoi(v); err == nil { off = n } }
            f := purchasing.PurchaseOrderFilter{ Q: qv.Get("q"), SupplierID: qv.Get("supplier_id"), Status: qv.Get("status"), Limit: lim, Offset: off }
            list, err := poSvc.List(req.Context(), f)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            writeJSON(w, http.StatusOK, list)
        })
        r.Get("/statuses", func(w http.ResponseWriter, req *http.Request) { writeJSON(w, http.StatusOK, purchasing.Statuses()) })
        r.Post("/", func(w http.ResponseWriter, req *http.Request) {
            var in purchasing.PurchaseOrderCreate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            po, items, err := poSvc.Create(req.Context(), in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusCreated, map[string]any{"bestellung": po, "positionen": items})
        })
        r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            po, items, err := poSvc.Get(req.Context(), id)
            if err != nil { http.Error(w, err.Error(), http.StatusNotFound); return }
            writeJSON(w, http.StatusOK, map[string]any{"bestellung": po, "positionen": items})
        })
        // PDF-Ausgabe einer Bestellung
        r.Get("/{id}/pdf", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            po, items, err := poSvc.Get(req.Context(), id)
            if err != nil { http.Error(w, err.Error(), http.StatusNotFound); return }
            // Template-Einstellungen für 'purchase_order'
            t, err := pdfSvc.Get(req.Context(), "purchase_order")
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }

            // Daten auf Druckmodell mappen
            date := po.OrderDate.Format("02.01.2006")
            data := pdfgen.PurchaseOrderData{
                Number: po.Number,
                OrderDate: date,
                Currency: po.Currency,
                Status: po.Status,
                Note: po.Note,
            }
            data.Items = make([]pdfgen.PurchaseOrderItemData, 0, len(items))
            for _, it := range items {
                data.Items = append(data.Items, pdfgen.PurchaseOrderItemData{
                    Pos: it.Position,
                    Description: it.Description,
                    Qty: it.Qty,
                    UOM: it.UOM,
                    UnitPrice: it.UnitPrice,
                    Currency: it.Currency,
                })
            }

            // Template-Optionen
            opts := pdfgen.TemplateOptions{
                HeaderText: t.HeaderText,
                FooterText: t.FooterText,
                TopFirstMM: t.TopFirstMM,
                TopOtherMM: t.TopOtherMM,
            }
            // Bilder (Logo/Backgrounds) über GridFS laden
            imgIDs := map[string]string{}
            if t.LogoDocID != nil { imgIDs["logo"] = *t.LogoDocID }
            if t.BgFirstDocID != nil { imgIDs["bg_first"] = *t.BgFirstDocID }
            if t.BgOtherDocID != nil { imgIDs["bg_other"] = *t.BgOtherDocID }

            pdfBytes, err := pdfgen.RenderPurchaseOrder(req.Context(), mg, cfg.MongoDB, data, opts, imgIDs)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }

            // Antwort senden
            filename := fmt.Sprintf("Bestellung_%s.pdf", sanitizeFilename(po.Number))
            w.Header().Set("Content-Type", "application/pdf")
            w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
            w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
            if _, err := w.Write(pdfBytes); err != nil { return }
        })
        r.Patch("/{id}", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            var in purchasing.PurchaseOrderUpdate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            po, items, err := poSvc.Update(req.Context(), id, in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusOK, map[string]any{"bestellung": po, "positionen": items})
        })
        r.Post("/{id}/items", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            var in purchasing.PurchaseOrderItemInput
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            it, err := poSvc.CreateItem(req.Context(), id, in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusCreated, it)
        })
        r.Patch("/{id}/items/{itemID}", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            itemID := chi.URLParam(req, "itemID")
            var in purchasing.PurchaseOrderItemUpdate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            it, err := poSvc.UpdateItem(req.Context(), id, itemID, in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusOK, it)
        })
        r.Delete("/{id}/items/{itemID}", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            itemID := chi.URLParam(req, "itemID")
            if err := poSvc.DeleteItem(req.Context(), id, itemID); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            w.WriteHeader(http.StatusNoContent)
        })
    })

    // Projekte (basic CRUD – MVP)
    r.Route("/projects", func(r chi.Router) {
        // Logikal-Import (SQLite Upload)
        r.Post("/import/logikal", func(w http.ResponseWriter, req *http.Request) {
            log.Printf("[Import] Start %s len=%d ct=%s", req.Method, req.ContentLength, req.Header.Get("Content-Type"))
            ct := req.Header.Get("Content-Type")
            // temporäre Datei
            tmp, err := os.CreateTemp("", "logikal-*.sqlite")
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            tmpName := tmp.Name()
            var srcName string
            if strings.HasPrefix(ct, "multipart/form-data") {
                // bis zu 256MB im Speicher für Formdaten
                if err := req.ParseMultipartForm(256 << 20); err != nil { http.Error(w, "Ungültiges Upload-Formular", http.StatusBadRequest); return }
                f, header, err := req.FormFile("file")
                if err != nil { http.Error(w, "Datei fehlt (Feld 'file')", http.StatusBadRequest); return }
                defer f.Close()
                srcName = header.Filename
                if _, err := io.Copy(tmp, f); err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
                _ = tmp.Close()
            } else {
                // Rohdaten-Upload (application/octet-stream)
                srcName = req.Header.Get("X-Filename")
                if _, err := io.Copy(tmp, req.Body); err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
                _ = tmp.Close()
            }
            defer os.Remove(tmpName)
            if fi, err := os.Stat(tmpName); err == nil { log.Printf("[Import] temp file %s size=%d src=%s", tmpName, fi.Size(), srcName) }

            proj, importID, err := projSvc.ImportLogikal(req.Context(), tmpName, srcName)
            if err != nil { http.Error(w, "Import fehlgeschlagen: "+ err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusCreated, map[string]any{"projekt": proj, "quelle": srcName, "import_id": importID})
        })

        // Analyse Logikal (ohne Import) – liefert eine Zusammenfassung
        r.Post("/analyze/logikal", func(w http.ResponseWriter, req *http.Request) {
            ct := req.Header.Get("Content-Type")
            tmp, err := os.CreateTemp("", "logikal-analyze-*.sqlite")
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            tmpName := tmp.Name()
            var srcName string
            if strings.HasPrefix(ct, "multipart/form-data") {
                if err := req.ParseMultipartForm(64 << 20); err != nil { http.Error(w, "Ungültiges Upload-Formular", http.StatusBadRequest); return }
                f, header, err := req.FormFile("file")
                if err != nil { http.Error(w, "Datei fehlt (Feld 'file')", http.StatusBadRequest); return }
                defer f.Close()
                srcName = header.Filename
                if _, err := io.Copy(tmp, f); err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
                _ = tmp.Close()
            } else {
                srcName = req.Header.Get("X-Filename")
                if _, err := io.Copy(tmp, req.Body); err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
                _ = tmp.Close()
            }
            defer os.Remove(tmpName)

            summary, err := projSvc.AnalyzeLogikal(req.Context(), tmpName)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            summary["source"] = srcName
            writeJSON(w, http.StatusOK, summary)
        })

        // Import-Protokolle
        r.Get("/{id}/imports", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            list, err := projSvc.ListImports(req.Context(), id)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            writeJSON(w, http.StatusOK, list)
        })
        r.Get("/{id}/imports/{importID}/changes", func(w http.ResponseWriter, req *http.Request) {
            importID := chi.URLParam(req, "importID")
            q := req.URL.Query(); format := strings.ToLower(q.Get("format")); kind := q.Get("kind"); action := q.Get("action")
            if format == "csv" {
                csv, err := projSvc.ExportImportChangesCSV(req.Context(), importID)
                if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
                w.Header().Set("Content-Type", "text/csv; charset=utf-8")
                w.Header().Set("Content-Disposition", "attachment; filename=import-"+importID+".csv")
                _, _ = io.WriteString(w, csv)
                return
            }
            var list []projects.ImportChange
            var err error
            if kind != "" || action != "" { list, err = projSvc.ListImportChangesFiltered(req.Context(), importID, kind, action) } else { list, err = projSvc.ListImportChanges(req.Context(), importID) }
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            writeJSON(w, http.StatusOK, list)
        })
        r.Post("/{id}/imports/{importID}/undo", func(w http.ResponseWriter, req *http.Request) {
            importID := chi.URLParam(req, "importID")
            if err := projSvc.UndoImport(req.Context(), importID); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusOK, map[string]any{"status":"ok","undone_import": importID})
        })
        r.Get("/", func(w http.ResponseWriter, req *http.Request) {
            qv := req.URL.Query()
            lim, off := 0, 0
            if v := qv.Get("limit"); v != "" { if n, err := strconv.Atoi(v); err == nil { lim = n } }
            if v := qv.Get("offset"); v != "" { if n, err := strconv.Atoi(v); err == nil { off = n } }
            f := projects.ProjectFilter{ Q: qv.Get("q"), Status: qv.Get("status"), Limit: lim, Offset: off }
            list, err := projSvc.List(req.Context(), f)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            writeJSON(w, http.StatusOK, list)
        })
        r.Post("/", func(w http.ResponseWriter, req *http.Request) {
            var in projects.ProjectCreate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            out, err := projSvc.Create(req.Context(), in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusCreated, out)
        })
        r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            out, err := projSvc.Get(req.Context(), id)
            if err != nil { http.Error(w, err.Error(), http.StatusNotFound); return }
            writeJSON(w, http.StatusOK, out)
        })

        // Lose (Phases)
        r.Get("/{id}/phases", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            list, err := projSvc.ListPhases(req.Context(), id)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            writeJSON(w, http.StatusOK, list)
        })
        r.Post("/{id}/phases", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            var in projects.PhaseCreate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            out, err := projSvc.CreatePhase(req.Context(), id, in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusCreated, out)
        })
        r.Get("/{id}/phases/{phaseID}", func(w http.ResponseWriter, req *http.Request) {
            phaseID := chi.URLParam(req, "phaseID")
            out, err := projSvc.GetPhase(req.Context(), phaseID)
            if err != nil { http.Error(w, err.Error(), http.StatusNotFound); return }
            writeJSON(w, http.StatusOK, out)
        })
        r.Patch("/{id}/phases/{phaseID}", func(w http.ResponseWriter, req *http.Request) {
            phaseID := chi.URLParam(req, "phaseID")
            var in projects.PhaseUpdate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            out, err := projSvc.UpdatePhase(req.Context(), phaseID, in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusOK, out)
        })
        r.Delete("/{id}/phases/{phaseID}", func(w http.ResponseWriter, req *http.Request) {
            phaseID := chi.URLParam(req, "phaseID")
            if err := projSvc.DeletePhase(req.Context(), phaseID); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            w.WriteHeader(http.StatusNoContent)
        })

        // Elevations je Phase
        r.Get("/{id}/phases/{phaseID}/elevations", func(w http.ResponseWriter, req *http.Request) {
            phaseID := chi.URLParam(req, "phaseID")
            list, err := projSvc.ListElevations(req.Context(), phaseID)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            writeJSON(w, http.StatusOK, list)
        })
        r.Post("/{id}/phases/{phaseID}/elevations", func(w http.ResponseWriter, req *http.Request) {
            phaseID := chi.URLParam(req, "phaseID")
            var in projects.ElevationCreate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            out, err := projSvc.CreateElevation(req.Context(), phaseID, in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusCreated, out)
        })
        r.Get("/{id}/phases/{phaseID}/elevations/{elevID}", func(w http.ResponseWriter, req *http.Request) {
            elevID := chi.URLParam(req, "elevID")
            out, err := projSvc.GetElevation(req.Context(), elevID)
            if err != nil { http.Error(w, err.Error(), http.StatusNotFound); return }
            writeJSON(w, http.StatusOK, out)
        })
        r.Patch("/{id}/phases/{phaseID}/elevations/{elevID}", func(w http.ResponseWriter, req *http.Request) {
            elevID := chi.URLParam(req, "elevID")
            var in projects.ElevationUpdate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            out, err := projSvc.UpdateElevation(req.Context(), elevID, in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusOK, out)
        })
        r.Delete("/{id}/phases/{phaseID}/elevations/{elevID}", func(w http.ResponseWriter, req *http.Request) {
            elevID := chi.URLParam(req, "elevID")
            if err := projSvc.DeleteElevation(req.Context(), elevID); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            w.WriteHeader(http.StatusNoContent)
        })

        // Ausführungsvarianten je Elevation
        r.Get("/{id}/elevations/{elevID}/single-elevations", func(w http.ResponseWriter, req *http.Request) {
            elevID := chi.URLParam(req, "elevID")
            list, err := projSvc.ListSingleElevations(req.Context(), elevID)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            writeJSON(w, http.StatusOK, list)
        })
        r.Post("/{id}/elevations/{elevID}/single-elevations", func(w http.ResponseWriter, req *http.Request) {
            elevID := chi.URLParam(req, "elevID")
            var in projects.SingleElevationCreate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            out, err := projSvc.CreateSingleElevation(req.Context(), elevID, in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusCreated, out)
        })
        r.Get("/{id}/elevations/{elevID}/single-elevations/{sid}", func(w http.ResponseWriter, req *http.Request) {
            sid := chi.URLParam(req, "sid")
            out, err := projSvc.GetSingleElevation(req.Context(), sid)
            if err != nil { http.Error(w, err.Error(), http.StatusNotFound); return }
            writeJSON(w, http.StatusOK, out)
        })
        r.Patch("/{id}/elevations/{elevID}/single-elevations/{sid}", func(w http.ResponseWriter, req *http.Request) {
            sid := chi.URLParam(req, "sid")
            var in projects.SingleElevationUpdate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            out, err := projSvc.UpdateSingleElevation(req.Context(), sid, in)
            if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusOK, out)
        })
        r.Delete("/{id}/elevations/{elevID}/single-elevations/{sid}", func(w http.ResponseWriter, req *http.Request) {
            sid := chi.URLParam(req, "sid")
            if err := projSvc.DeleteSingleElevation(req.Context(), sid); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            w.WriteHeader(http.StatusNoContent)
        })

        // Materiallisten je Single-Elevation (Profile, Articles, Glass)
        r.Get("/{id}/single-elevations/{sid}/materials", func(w http.ResponseWriter, req *http.Request) {
            sid := chi.URLParam(req, "sid")
            profiles, err := projSvc.ListProfilesBySingle(req.Context(), sid)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            articles, err := projSvc.ListArticlesBySingle(req.Context(), sid)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            glass, err := projSvc.ListGlassBySingle(req.Context(), sid)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            writeJSON(w, http.StatusOK, map[string]any{
                "profiles": profiles,
                "articles": articles,
                "glass": glass,
            })
        })

        // Projekt-Assets (Zip mit Emfs/Rtfs) hochladen und abrufen
        r.Post("/{id}/assets", func(w http.ResponseWriter, req *http.Request) {
            pid := chi.URLParam(req, "id")
            if err := req.ParseMultipartForm(512 << 20); err != nil { http.Error(w, "Ungültiges Formular", http.StatusBadRequest); return }
            file, _, err := req.FormFile("file")
            if err != nil { http.Error(w, "Datei fehlt (Feld 'file')", http.StatusBadRequest); return }
            defer file.Close()
            // ZIP in Speicher lesen
            buf, err := io.ReadAll(file)
            if err != nil { http.Error(w, "Upload lesen fehlgeschlagen", http.StatusBadRequest); return }
            zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
            if err != nil { http.Error(w, "Ungültiges ZIP", http.StatusBadRequest); return }
            // GridFS Bucket öffnen
            db := mg.Database(cfg.MongoDB)
            bucket, err := gridfs.NewBucket(db)
            if err != nil { http.Error(w, "GridFS nicht verfügbar", http.StatusInternalServerError); return }
            type item struct{ Rel string `json:"rel"` }
            saved := make([]item, 0)
            converted := make([]item, 0)
            skipped := make([]item, 0)
            // Dateien iterieren
            for _, f := range zr.File {
                name := f.Name
                low := strings.ToLower(name)
                // Relativer Pfad ab Emfs/ oder Rtfs/ (case-insensitive, auch wenn ein Top-Level-Ordner vorangestellt ist)
                idx := strings.Index(low, "/emfs/")
                if idx < 0 { idx = strings.Index(low, "/rtfs/") }
                if idx < 0 {
                    if strings.HasPrefix(low, "emfs/") || strings.HasPrefix(low, "rtfs/") { idx = -1 } else { skipped = append(skipped, item{Rel: name}); continue }
                }
                rel := name
                if idx >= 0 { rel = name[idx+1:] }
                rc, err := f.Open(); if err != nil { continue }
                oid, err := bucket.UploadFromStream(rel, rc)
                rc.Close()
                if err != nil { continue }
                // ContentType heuristisch
                ct := "application/octet-stream"
                if strings.HasSuffix(strings.ToLower(rel), ".png") { ct = "image/png" } else if strings.HasSuffix(strings.ToLower(rel), ".jpg") || strings.HasSuffix(strings.ToLower(rel), ".jpeg") { ct = "image/jpeg" } else if strings.HasSuffix(strings.ToLower(rel), ".svg") { ct = "image/svg+xml" } else if strings.HasSuffix(strings.ToLower(rel), ".emf") { ct = "image/emf" }
                // Länge aus fs.files lesen
                var length int64
                var storedName string = name
                var meta struct{ Length int64 `bson:"length"`; Filename string `bson:"filename"` }
                _ = db.Collection("fs.files").FindOne(req.Context(), bson.M{"_id": oid}).Decode(&meta)
                if meta.Length > 0 { length = meta.Length }
                if meta.Filename != "" { storedName = meta.Filename }
                _ = projSvc.UpsertProjectAsset(req.Context(), pid, rel, oid.Hex(), storedName, ct, length)
                log.Printf("assets: gespeichert %s (%d bytes)", rel, length)
                saved = append(saved, item{Rel: rel})

                // EMF -> PNG Konvertierung, falls möglich
                if strings.HasSuffix(strings.ToLower(rel), ".emf") {
                    // EMF in Tempdatei schreiben
                    if rc2, err2 := f.Open(); err2 == nil {
                        emfFile, _ := os.CreateTemp("", "emf2png-*.emf")
                        pngFile, _ := os.CreateTemp("", "emf2png-*.png")
                        if emfFile != nil && pngFile != nil {
                            _, _ = io.Copy(emfFile, rc2)
                            _ = rc2.Close()
                            _ = emfFile.Close()
                            // 1) Inkscape bevorzugt
                            if p, _ := exec.LookPath("inkscape"); p != "" {
                                dpiStr := strconv.Itoa(cfg.EmfPngDPI)
                                widthStr := strconv.Itoa(cfg.EmfPngTargetWidth)
                                cmd := exec.Command(p, "--export-type=png", "--export-dpi="+dpiStr, "--export-width="+widthStr, "--export-background=white", "--export-background-opacity=1", "--export-filename", pngFile.Name(), emfFile.Name())
                                if errRun := cmd.Run(); errRun == nil {
                                    // Nachschärfen: minimale Breite sicherstellen (nur vergrößern, nicht verkleinern)
                                    if pp, _ := exec.LookPath("magick"); pp != "" {
                                        _ = exec.Command(pp, pngFile.Name(), "-filter", "Lanczos", "-define", "filter:blur=0.9", "-resize", fmt.Sprintf("%dx<", cfg.EmfPngMinWidth), "-colorspace", "sRGB", pngFile.Name()).Run()
                                    } else if pp2, _ := exec.LookPath("convert"); pp2 != "" {
                                        _ = exec.Command(pp2, pngFile.Name(), "-filter", "Lanczos", "-define", "filter:blur=0.9", "-resize", fmt.Sprintf("%dx<", cfg.EmfPngMinWidth), "-colorspace", "sRGB", pngFile.Name()).Run()
                                    }
                                    if pf, errOpen := os.Open(pngFile.Name()); errOpen == nil {
                                        base := rel[:strings.LastIndex(strings.ToLower(rel), ".")] + ".png"
                                        if oid2, errUp := bucket.UploadFromStream(base, pf); errUp == nil {
                                            _ = pf.Close()
                                            var meta2 struct{ Length int64 `bson:"length"`; Filename string `bson:"filename"` }
                                            _ = db.Collection("fs.files").FindOne(req.Context(), bson.M{"_id": oid2}).Decode(&meta2)
                                            _ = projSvc.UpsertProjectAsset(req.Context(), pid, base, oid2.Hex(), base, "image/png", meta2.Length)
                                            log.Printf("assets: konvertiert(inkscape) %s -> %s (%d bytes)", rel, base, meta2.Length)
                                            converted = append(converted, item{Rel: base})
                                        } else { _ = pf.Close(); log.Printf("assets: upload png fehlgeschlagen (inkscape): %v", errUp) }
                                    } else { log.Printf("assets: open png fehlgeschlagen (inkscape): %v", errOpen) }
                                } else { log.Printf("assets: konvertierung fehlgeschlagen (inkscape)") }
                            } else if p, _ := exec.LookPath("convert"); p != "" { // 2) ImageMagick 6
                                dpiStr := strconv.Itoa(cfg.EmfPngDPI)
                                minw := strconv.Itoa(cfg.EmfPngMinWidth)
                                targetw := strconv.Itoa(cfg.EmfPngTargetWidth)
                                cmd := exec.Command(p, "-density", dpiStr, emfFile.Name(), "-units", "PixelsPerInch", "-resample", dpiStr, "-background", "white", "-alpha", "remove", "-filter", "Lanczos", "-define", "filter:blur=0.9", "-resize", targetw, "-resize", minw+"x<", "-colorspace", "sRGB", pngFile.Name())
                                if errRun := cmd.Run(); errRun == nil {
                                    if pf, errOpen := os.Open(pngFile.Name()); errOpen == nil {
                                        base := rel[:strings.LastIndex(strings.ToLower(rel), ".")] + ".png"
                                        if oid2, errUp := bucket.UploadFromStream(base, pf); errUp == nil {
                                            _ = pf.Close()
                                            var meta2 struct{ Length int64 `bson:"length"`; Filename string `bson:"filename"` }
                                            _ = db.Collection("fs.files").FindOne(req.Context(), bson.M{"_id": oid2}).Decode(&meta2)
                                            _ = projSvc.UpsertProjectAsset(req.Context(), pid, base, oid2.Hex(), base, "image/png", meta2.Length)
                                            log.Printf("assets: konvertiert(convert) %s -> %s (%d bytes)", rel, base, meta2.Length)
                                            converted = append(converted, item{Rel: base})
                                        } else { _ = pf.Close(); log.Printf("assets: upload png fehlgeschlagen (convert): %v", errUp) }
                                    } else { log.Printf("assets: open png fehlgeschlagen (convert): %v", errOpen) }
                                } else { log.Printf("assets: konvertierung fehlgeschlagen (convert)") }
                            } else if p, _ := exec.LookPath("magick"); p != "" { // 3) ImageMagick 7
                                dpiStr := strconv.Itoa(cfg.EmfPngDPI)
                                minw := strconv.Itoa(cfg.EmfPngMinWidth)
                                targetw := strconv.Itoa(cfg.EmfPngTargetWidth)
                                cmd := exec.Command(p, "-density", dpiStr, emfFile.Name(), "-units", "PixelsPerInch", "-resample", dpiStr, "-background", "white", "-alpha", "remove", "-filter", "Lanczos", "-define", "filter:blur=0.9", "-resize", targetw, "-resize", minw+"x<", "-colorspace", "sRGB", pngFile.Name())
                                if errRun := cmd.Run(); errRun == nil {
                                    if pf, errOpen := os.Open(pngFile.Name()); errOpen == nil {
                                        base := rel[:strings.LastIndex(strings.ToLower(rel), ".")] + ".png"
                                        if oid2, errUp := bucket.UploadFromStream(base, pf); errUp == nil {
                                            _ = pf.Close()
                                            var meta2 struct{ Length int64 `bson:"length"`; Filename string `bson:"filename"` }
                                            _ = db.Collection("fs.files").FindOne(req.Context(), bson.M{"_id": oid2}).Decode(&meta2)
                                            _ = projSvc.UpsertProjectAsset(req.Context(), pid, base, oid2.Hex(), base, "image/png", meta2.Length)
                                            log.Printf("assets: konvertiert(magick) %s -> %s (%d bytes)", rel, base, meta2.Length)
                                            converted = append(converted, item{Rel: base})
                                        } else { _ = pf.Close(); log.Printf("assets: upload png fehlgeschlagen (magick): %v", errUp) }
                                    } else { log.Printf("assets: open png fehlgeschlagen (magick): %v", errOpen) }
                                } else { log.Printf("assets: konvertierung fehlgeschlagen (magick)") }
                            } else if p, _ := exec.LookPath("soffice"); p != "" { // 4) LibreOffice
                                outDir := filepath.Dir(pngFile.Name())
                                profDir, _ := os.MkdirTemp("", "lo-profile-")
                                userInst := "-env:UserInstallation=file://" + profDir
                                // Bevorzuge: EMF -> PDF -> PNG mit Magick (bessere Qualität)
                                // Bevorzuge: EMF -> PDF -> PNG mit Magick (bessere Qualität)
                                profDir2, _ := os.MkdirTemp("", "lo-profile-")
                                userInst2 := "-env:UserInstallation=file://" + profDir2
                                cmdPdf := exec.Command(p, userInst2, "--headless", "--convert-to", "pdf", "--outdir", outDir, emfFile.Name())
                                cmdPdf.Env = append(os.Environ(), "HOME=/tmp")
                                if errPdf := cmdPdf.Run(); errPdf == nil {
                                    pdfPath := filepath.Join(outDir, strings.TrimSuffix(filepath.Base(emfFile.Name()), filepath.Ext(emfFile.Name()))+".pdf")
                                    if pp, _ := exec.LookPath("magick"); pp != "" {
                                        dpiStr := strconv.Itoa(cfg.EmfPngDPI)
                                        minw := strconv.Itoa(cfg.EmfPngMinWidth)
                                        targetw := strconv.Itoa(cfg.EmfPngTargetWidth)
                                        cmd3 := exec.Command(pp, "-density", dpiStr, pdfPath, "-units", "PixelsPerInch", "-resample", dpiStr, "-background", "white", "-alpha", "remove", "-filter", "Lanczos", "-define", "filter:blur=0.9", "-resize", targetw, "-resize", minw+"x<", "-colorspace", "sRGB", pngFile.Name())
                                        if err3 := cmd3.Run(); err3 == nil {
                                            if pf, errOpen := os.Open(pngFile.Name()); errOpen == nil {
                                                base := rel[:strings.LastIndex(strings.ToLower(rel), ".")] + ".png"
                                                if oid2, errUp := bucket.UploadFromStream(base, pf); errUp == nil {
                                                    _ = pf.Close()
                                                    var meta2 struct{ Length int64 `bson:"length"`; Filename string `bson:"filename"` }
                                                    _ = db.Collection("fs.files").FindOne(req.Context(), bson.M{"_id": oid2}).Decode(&meta2)
                                                    _ = projSvc.UpsertProjectAsset(req.Context(), pid, base, oid2.Hex(), base, "image/png", meta2.Length)
                                                    log.Printf("assets: konvertiert(soffice+magick) %s -> %s (%d bytes)", rel, base, meta2.Length)
                                                    converted = append(converted, item{Rel: base})
                                                } else { _ = pf.Close(); log.Printf("assets: upload png fehlgeschlagen (soffice+magick): %v", errUp) }
                                            } else { log.Printf("assets: open png fehlgeschlagen (soffice+magick): %v", errOpen) }
                                        } else { log.Printf("assets: pdf->png fehlgeschlagen (magick): %v", err3) }
                                    }
                                    _ = os.Remove(pdfPath)
                                } else {
                                    // Fallback: LibreOffice direkt nach PNG (geringere Qualität)
                                    cmd2 := exec.Command(p, userInst, "--headless", "--convert-to", "png", "--outdir", outDir, emfFile.Name())
                                    cmd2.Env = append(os.Environ(), "HOME=/tmp")
                                    if err2 := cmd2.Run(); err2 == nil {
                                        baseFile := strings.TrimSuffix(filepath.Base(emfFile.Name()), filepath.Ext(emfFile.Name())) + ".png"
                                        gen := filepath.Join(outDir, baseFile)
                                        // minimale Breite sicherstellen (nur vergrößern)
                                        if pp, _ := exec.LookPath("magick"); pp != "" {
                                            _ = exec.Command(pp, gen, "-filter", "Lanczos", "-define", "filter:blur=0.9", "-resize", fmt.Sprintf("%dx<", cfg.EmfPngMinWidth), "-colorspace", "sRGB", gen).Run()
                                        } else if pp2, _ := exec.LookPath("convert"); pp2 != "" {
                                            _ = exec.Command(pp2, gen, "-filter", "Lanczos", "-define", "filter:blur=0.9", "-resize", fmt.Sprintf("%dx<", cfg.EmfPngMinWidth), "-colorspace", "sRGB", gen).Run()
                                        }
                                        if pf, errOpen := os.Open(gen); errOpen == nil {
                                            base := rel[:strings.LastIndex(strings.ToLower(rel), ".")] + ".png"
                                            if oid2, errUp := bucket.UploadFromStream(base, pf); errUp == nil {
                                                _ = pf.Close()
                                                var meta2 struct{ Length int64 `bson:"length"`; Filename string `bson:"filename"` }
                                                _ = db.Collection("fs.files").FindOne(req.Context(), bson.M{"_id": oid2}).Decode(&meta2)
                                                _ = projSvc.UpsertProjectAsset(req.Context(), pid, base, oid2.Hex(), base, "image/png", meta2.Length)
                                                log.Printf("assets: konvertiert(soffice) %s -> %s (%d bytes)", rel, base, meta2.Length)
                                                converted = append(converted, item{Rel: base})
                                            } else { _ = pf.Close(); log.Printf("assets: upload png fehlgeschlagen (soffice): %v", errUp) }
                                        } else { log.Printf("assets: open png fehlgeschlagen (soffice): %v", errOpen) }
                                    } else { log.Printf("assets: konvertierung fehlgeschlagen (soffice png): %v", err2) }
                                }
                                if profDir != "" { _ = os.RemoveAll(profDir) }
                            } else {
                                log.Printf("assets: kein Konvertierungstool gefunden (inkscape/convert/magick/soffice)")
                            }
                        }
                        if emfFile != nil { _ = os.Remove(emfFile.Name()) }
                        if pngFile != nil { _ = os.Remove(pngFile.Name()) }
                    }
                }
            }
            if req.URL.Query().Get("summary") == "1" {
                writeJSON(w, http.StatusOK, map[string]any{"saved": saved, "converted": converted, "skipped": skipped})
            } else {
                w.WriteHeader(http.StatusNoContent)
            }
        })
        r.Get("/{id}/assets/list", func(w http.ResponseWriter, req *http.Request) {
            pid := chi.URLParam(req, "id")
            list, err := projSvc.ListProjectAssets(req.Context(), pid)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            writeJSON(w, http.StatusOK, list)
        })
        r.Get("/{id}/assets", func(w http.ResponseWriter, req *http.Request) {
            pid := chi.URLParam(req, "id")
            rel := strings.TrimSpace(req.URL.Query().Get("path"))
            if rel == "" { http.Error(w, "path erforderlich", http.StatusBadRequest); return }
            // Bevorzugt PNG, wenn .emf angefragt: erst PNG versuchen, dann EMF
            var gridID, filename, contentType string
            var length int64
            var err error
            if strings.HasSuffix(strings.ToLower(rel), ".emf") {
                alt := strings.TrimSuffix(rel, rel[strings.LastIndex(rel, "."):]) + ".png"
                if gid, fn, ct, ln, e := projSvc.GetProjectAsset(req.Context(), pid, alt); e == nil {
                    gridID, filename, contentType, length = gid, fn, ct, ln
                } else {
                    gridID, filename, contentType, length, err = projSvc.GetProjectAsset(req.Context(), pid, rel)
                }
            } else {
                gridID, filename, contentType, length, err = projSvc.GetProjectAsset(req.Context(), pid, rel)
            }
            if err != nil { http.Error(w, "Asset nicht gefunden", http.StatusNotFound); return }
            // GridFS stream öffnen
            db := mg.Database(cfg.MongoDB)
            bucket, err := gridfs.NewBucket(db)
            if err != nil { http.Error(w, "GridFS nicht verfügbar", http.StatusInternalServerError); return }
            oid, err := primitive.ObjectIDFromHex(gridID)
            if err != nil { http.Error(w, "Ungültige ID", http.StatusBadRequest); return }
            rc, err := bucket.OpenDownloadStream(oid)
            if err != nil { http.Error(w, "Stream fehlgeschlagen", http.StatusInternalServerError); return }
            defer rc.Close()
            if contentType == "" { contentType = "application/octet-stream" }
            w.Header().Set("Content-Type", contentType)
            if filename != "" { w.Header().Set("Content-Disposition", "inline; filename=\""+filename+"\"") }
            if length > 0 { w.Header().Set("Content-Length", fmt.Sprintf("%d", length)) }
            _, _ = io.Copy(w, rc)
        })
        // Link eines Materials aus der Varianten-Materialliste zu Stammmaterial setzen
        r.Patch("/{id}/single-elevations/{sid}/materials/{kind}/{itemID}", func(w http.ResponseWriter, req *http.Request) {
            kind := chi.URLParam(req, "kind")
            itemID := chi.URLParam(req, "itemID")
            var in struct{ MaterialID string `json:"material_id"` }
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            if err := projSvc.LinkVariantMaterial(req.Context(), kind, itemID, in.MaterialID); err != nil {
                http.Error(w, err.Error(), http.StatusBadRequest)
                return
            }
            w.WriteHeader(http.StatusNoContent)
        })
    })

    // Einstellungen – Nummernkreise
    r.Route("/settings/numbering", func(r chi.Router) {
        r.Get("/{entity}", func(w http.ResponseWriter, req *http.Request) {
            entity := chi.URLParam(req, "entity")
            cfg, err := numSvc.Get(req.Context(), entity)
            if err != nil { http.Error(w, err.Error(), http.StatusNotFound); return }
            writeJSON(w, http.StatusOK, cfg)
        })
        r.Get("/{entity}/preview", func(w http.ResponseWriter, req *http.Request) {
            entity := chi.URLParam(req, "entity")
            s, err := numSvc.Preview(req.Context(), entity)
            if err != nil { http.Error(w, err.Error(), http.StatusNotFound); return }
            writeJSON(w, http.StatusOK, map[string]string{"preview": s})
        })
        r.Put("/{entity}", func(w http.ResponseWriter, req *http.Request) {
            entity := chi.URLParam(req, "entity")
            var in struct{ Pattern string `json:"pattern"` }
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            if strings.TrimSpace(in.Pattern) == "" { http.Error(w, "Pattern erforderlich", http.StatusBadRequest); return }
            if err := numSvc.UpdatePattern(req.Context(), entity, in.Pattern); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            w.WriteHeader(http.StatusNoContent)
        })
    })

    // Einstellungen – PDF Templates
    r.Route("/settings/pdf", func(r chi.Router) {
        r.Get("/{entity}", func(w http.ResponseWriter, req *http.Request) {
            entity := chi.URLParam(req, "entity")
            t, err := pdfSvc.Get(req.Context(), entity)
            if err != nil { http.Error(w, err.Error(), http.StatusNotFound); return }
            writeJSON(w, http.StatusOK, t)
        })
        r.Put("/{entity}", func(w http.ResponseWriter, req *http.Request) {
            entity := chi.URLParam(req, "entity")
            var in struct{
                HeaderText string  `json:"header_text"`
                FooterText string  `json:"footer_text"`
                TopFirstMM float64 `json:"top_first_mm"`
                TopOtherMM float64 `json:"top_other_mm"`
            }
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            if in.TopFirstMM <= 0 { in.TopFirstMM = 30 }
            if in.TopOtherMM <= 0 { in.TopOtherMM = 20 }
            if err := pdfSvc.Upsert(req.Context(), settings.PDFTemplate{Entity: entity, HeaderText: in.HeaderText, FooterText: in.FooterText, TopFirstMM: in.TopFirstMM, TopOtherMM: in.TopOtherMM}); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            w.WriteHeader(http.StatusNoContent)
        })
        // Uploads: logo, bg-first, bg-other
        r.Post("/{entity}/upload/{kind}", func(w http.ResponseWriter, req *http.Request) {
            entity := chi.URLParam(req, "entity")
            kind := chi.URLParam(req, "kind")
            if err := req.ParseMultipartForm(16<<20); err != nil { http.Error(w, "Ungültiges Formular", http.StatusBadRequest); return }
            file, header, err := req.FormFile("file")
            if err != nil { http.Error(w, "Datei fehlt (file)", http.StatusBadRequest); return }
            defer file.Close()
            // Upload to GridFS
            db := mg.Database(cfg.MongoDB)
            bucket, err := gridfs.NewBucket(db)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            oid, err := bucket.UploadFromStream(header.Filename, file)
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            hex := oid.Hex()
            if err := pdfSvc.SetImage(req.Context(), entity, mapPDFKind(kind), &hex); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            writeJSON(w, http.StatusCreated, map[string]any{"document_id": hex})
        })
        r.Delete("/{entity}/upload/{kind}", func(w http.ResponseWriter, req *http.Request) {
            entity := chi.URLParam(req, "entity")
            kind := chi.URLParam(req, "kind")
            if err := pdfSvc.SetImage(req.Context(), entity, mapPDFKind(kind), nil); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            w.WriteHeader(http.StatusNoContent)
        })
    })

    // Einstellungen – Einheiten (für Materialeinheiten, Dimensionen etc.)
    r.Route("/settings/units", func(r chi.Router) {
        r.Get("/", func(w http.ResponseWriter, req *http.Request) {
            list, err := unitSvc.List(req.Context())
            if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            writeJSON(w, http.StatusOK, list)
        })
        r.Post("/", func(w http.ResponseWriter, req *http.Request) {
            var in struct{ Code, Name string }
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil { http.Error(w, "Ungültige Eingabe", http.StatusBadRequest); return }
            if err := unitSvc.Upsert(req.Context(), in.Code, in.Name); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            w.WriteHeader(http.StatusNoContent)
        })
        r.Delete("/{code}", func(w http.ResponseWriter, req *http.Request) {
            code := chi.URLParam(req, "code")
            if err := unitSvc.Delete(req.Context(), code); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
            w.WriteHeader(http.StatusNoContent)
        })
    })

    // Download eines Dokuments über DocumentID (GridFS ObjectID Hex)
    r.Get("/documents/{docID}", func(w http.ResponseWriter, req *http.Request) {
        docID := chi.URLParam(req, "docID")
        rc, filename, contentType, length, err := matSvc.OpenDocumentStream(req.Context(), docID)
        if err != nil {
            http.Error(w, err.Error(), http.StatusNotFound)
            return
        }
        defer rc.Close()
        if contentType == "" { contentType = "application/octet-stream" }
        w.Header().Set("Content-Type", contentType)
        if filename != "" {
            w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
        }
        if length > 0 { w.Header().Set("Content-Length", fmt.Sprintf("%d", length)) }
        if _, err := io.Copy(w, rc); err != nil {
            // can't write header after body; just stop
            return
        }
    })

    r.Route("/stock-movements", func(r chi.Router) {
        r.Post("/", func(w http.ResponseWriter, req *http.Request) {
            var in materials.StockMovementCreate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
                http.Error(w, "Ungültige Eingabe", http.StatusBadRequest)
                return
            }
            out, err := matSvc.CreateMovement(req.Context(), in)
            if err != nil {
                http.Error(w, err.Error(), http.StatusBadRequest)
                return
            }
            writeJSON(w, http.StatusCreated, out)
        })
    })

    r.Route("/warehouses", func(r chi.Router) {
        r.Post("/", func(w http.ResponseWriter, req *http.Request) {
            var in materials.WarehouseCreate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
                http.Error(w, "Ungültige Eingabe", http.StatusBadRequest)
                return
            }
            out, err := matSvc.CreateWarehouse(req.Context(), in)
            if err != nil {
                http.Error(w, err.Error(), http.StatusBadRequest)
                return
            }
            writeJSON(w, http.StatusCreated, out)
        })
        r.Get("/", func(w http.ResponseWriter, req *http.Request) {
            out, err := matSvc.ListWarehouses(req.Context())
            if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            writeJSON(w, http.StatusOK, out)
        })
        r.Post("/{id}/locations", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            var in materials.LocationCreate
            if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
                http.Error(w, "Ungültige Eingabe", http.StatusBadRequest)
                return
            }
            out, err := matSvc.CreateLocation(req.Context(), id, in)
            if err != nil {
                http.Error(w, err.Error(), http.StatusBadRequest)
                return
            }
            writeJSON(w, http.StatusCreated, out)
        })
        r.Get("/{id}/locations", func(w http.ResponseWriter, req *http.Request) {
            id := chi.URLParam(req, "id")
            out, err := matSvc.ListLocations(req.Context(), id)
            if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            writeJSON(w, http.StatusOK, out)
        })
    })

    return r
}

func writeJSON(w http.ResponseWriter, code int, v any) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(code)
    _ = json.NewEncoder(w).Encode(v)
}

// helper mapping for upload kinds for PDF settings
func mapPDFKind(k string) string {
    switch strings.ToLower(k) {
    case "logo": return "logo"
    case "bg-first", "bg_first": return "bg_first"
    case "bg-other", "bg_other": return "bg_other"
    default: return k
    }
}

// sanitizeFilename entfernt problematische Zeichen für Dateinamen in Content-Disposition
func sanitizeFilename(s string) string {
    if s == "" { return time.Now().Format("20060102") }
    repl := []string{"/", "-", "\\", "-", ":", "-", "\"", "'", "\n", " ", "\r", " "}
    r := strings.NewReplacer(repl...)
    out := r.Replace(s)
    if strings.TrimSpace(out) == "" { return time.Now().Format("20060102") }
    return out
}
