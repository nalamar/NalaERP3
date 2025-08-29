package apihttp

import (
    "encoding/json"
    "net/http"
    "io"
    "fmt"
    "strconv"
    "strings"

    "github.com/go-chi/chi/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/gridfs"
    "nalaerp3/internal/config"
    "nalaerp3/internal/materials"
    "nalaerp3/internal/contacts"
    "nalaerp3/internal/purchasing"
    "nalaerp3/internal/settings"
    "nalaerp3/internal/pdfgen"
    "time"
)

func NewV1Router(pg *pgxpool.Pool, mg *mongo.Client, rd *redis.Client, cfg *config.Config) http.Handler {
    r := chi.NewRouter()

    matSvc := materials.NewService(pg, mg, cfg.MongoDB)
    conSvc := contacts.NewService(pg)
    poSvc := purchasing.NewService(pg)
    numSvc := settings.NewNumberingService(pg)
    pdfSvc := settings.NewPDFService(pg)

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
