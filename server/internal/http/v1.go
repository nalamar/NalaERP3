package apihttp

import (
    "encoding/json"
    "net/http"
    "io"
    "fmt"

    "github.com/go-chi/chi/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"
    "go.mongodb.org/mongo-driver/mongo"
    "nalaerp3/internal/config"
    "nalaerp3/internal/materials"
)

func NewV1Router(pg *pgxpool.Pool, mg *mongo.Client, rd *redis.Client, cfg *config.Config) http.Handler {
    r := chi.NewRouter()

    matSvc := materials.NewService(pg, mg, cfg.MongoDB)

    r.Route("/materials", func(r chi.Router) {
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
            list, err := matSvc.List(req.Context(), materials.MaterialFilter{})
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
