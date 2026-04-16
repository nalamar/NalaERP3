package apihttp

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"nalaerp3/internal/accounting"
	"nalaerp3/internal/auth"
	"nalaerp3/internal/config"
	"nalaerp3/internal/contacts"
	"nalaerp3/internal/materials"
	"nalaerp3/internal/pdfgen"
	"nalaerp3/internal/projects"
	"nalaerp3/internal/purchasing"
	"nalaerp3/internal/quotes"
	"nalaerp3/internal/sales"
	"nalaerp3/internal/settings"
	"time"
)

func NewV1Router(pg *pgxpool.Pool, mg *mongo.Client, rd *redis.Client, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	matSvc := materials.NewService(pg, mg, cfg.MongoDB)
	authRepo := auth.NewRepository(pg)
	authStore := auth.NewSessionStore(rd)
	authSvc := auth.NewService(authRepo, authStore, cfg)
	protected := r.With(requireAuth(authSvc))
	conSvc := contacts.NewService(pg).WithMongo(mg, cfg.MongoDB)
	poSvc := purchasing.NewService(pg)
	numSvc := settings.NewNumberingService(pg)
	journalSvc := accounting.NewJournalService(pg)
	arSvc := accounting.NewARService(pg, numSvc, journalSvc)
	paymentSvc := accounting.NewPaymentService(pg, journalSvc)
	pdfSvc := settings.NewPDFService(pg)
	unitSvc := settings.NewUnitService(pg)
	materialGroupSvc := settings.NewMaterialGroupService(pg)
	quoteTextBlockSvc := settings.NewQuoteTextBlockService(pg)
	companySvc := settings.NewCompanyService(pg)
	locSvc := settings.NewLocalizationService(pg)
	brandingSvc := settings.NewBrandingService(pg)
	projSvc := projects.NewService(pg)
	quoteSvc := quotes.NewService(pg, numSvc).WithMongo(mg, cfg.MongoDB)
	salesSvc := sales.NewService(pg, numSvc)

	r.Route("/auth", func(r chi.Router) {
		r.Post("/login", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				Login    string `json:"login"`
				Password string `json:"password"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			res, err := authSvc.Login(req.Context(), auth.LoginInput{
				Login:     in.Login,
				Password:  in.Password,
				IPAddress: req.RemoteAddr,
				UserAgent: req.UserAgent(),
			})
			if err != nil {
				switch err {
				case auth.ErrInvalidCredentials, auth.ErrUserInactive, auth.ErrUserLocked:
					writeAPIError(w, req, http.StatusUnauthorized, "auth_failed", "Anmeldung fehlgeschlagen")
				default:
					writeAPIError(w, req, http.StatusInternalServerError, "internal_error", "Interner Fehler")
				}
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": res})
		})
		r.Post("/refresh", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				RefreshToken string `json:"refresh_token"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			pair, err := authSvc.Refresh(req.Context(), in.RefreshToken)
			if err != nil {
				writeAPIError(w, req, http.StatusUnauthorized, "invalid_token", "Token ungültig oder abgelaufen")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": pair})
		})
		r.Post("/logout", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				RefreshToken string `json:"refresh_token"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			if err := authSvc.Logout(req.Context(), in.RefreshToken); err != nil {
				writeAPIError(w, req, http.StatusUnauthorized, "invalid_token", "Token ungültig oder abgelaufen")
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		r.With(requireAuth(authSvc)).Get("/me", func(w http.ResponseWriter, req *http.Request) {
			user, ok := authUserFromContext(req.Context())
			if !ok {
				writeAPIError(w, req, http.StatusUnauthorized, "unauthorized", "Nicht angemeldet")
				return
			}
			roles, _ := authRolesFromContext(req.Context())
			permissions, _ := authPermissionsFromContext(req.Context())
			writeJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{
					"user":        user,
					"roles":       roles,
					"permissions": permissions,
				},
			})
		})
	})

	protected.Route("/materials", func(r chi.Router) {
		r.With(requirePermission("materials.read")).Get("/types", func(w http.ResponseWriter, req *http.Request) {
			list, err := matSvc.ListTypes(req.Context())
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.With(requirePermission("materials.read")).Get("/categories", func(w http.ResponseWriter, req *http.Request) {
			list, err := matSvc.ListCategories(req.Context())
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.With(requirePermission("materials.write")).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in materials.MaterialCreate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := matSvc.Create(req.Context(), in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})

		r.With(requirePermission("materials.read")).Get("/", func(w http.ResponseWriter, req *http.Request) {
			q := req.URL.Query()
			lim := 0
			off := 0
			if v := q.Get("limit"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					lim = n
				}
			}
			if v := q.Get("offset"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					off = n
				}
			}
			filter := materials.MaterialFilter{
				Q:         q.Get("q"),
				Typ:       q.Get("typ"),
				Kategorie: q.Get("kategorie"),
				Limit:     lim,
				Offset:    off,
			}
			list, err := matSvc.List(req.Context(), filter)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})

		r.With(requirePermission("materials.read")).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			m, err := matSvc.Get(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, m)
		})

		r.With(requirePermission("materials.write")).Patch("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			var in materials.MaterialUpdate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := matSvc.Update(req.Context(), id, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		r.With(requirePermission("materials.write")).Delete("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			if err := matSvc.DeleteSoft(req.Context(), id); err != nil {
				writeDomainError(w, req, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		r.With(requirePermission("materials.read")).Get("/{id}/stock", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			res, err := matSvc.StockByMaterial(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, res)
		})

		// Upload Dokument zu Material
		r.With(requirePermission("materials.write")).Post("/{id}/documents", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			// bis zu 32MB Formdaten im Speicher
			if err := req.ParseMultipartForm(32 << 20); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültiges Upload-Formular", err)
				return
			}
			file, header, err := req.FormFile("file")
			if err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Datei fehlt (Feld 'file')", err)
				return
			}
			defer file.Close()

			contentType := header.Header.Get("Content-Type")
			doc, err := matSvc.UploadMaterialDocument(req.Context(), id, file, header.Filename, contentType)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, doc)
		})

		// Liste Dokumente eines Materials
		r.With(requirePermission("materials.read")).Get("/{id}/documents", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			docs, err := matSvc.ListMaterialDocuments(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, docs)
		})
	})

	// Kontakte (CRM)
	protected.Route("/contacts", func(r chi.Router) {
		r.With(requirePermission("contacts.read")).Get("/", func(w http.ResponseWriter, req *http.Request) {
			q := req.URL.Query()
			lim := 0
			off := 0
			if v := q.Get("limit"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					lim = n
				}
			}
			if v := q.Get("offset"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					off = n
				}
			}
			filter := contacts.ContactFilter{Q: q.Get("q"), Rolle: q.Get("rolle"), Status: q.Get("status"), Typ: q.Get("typ"), Limit: lim, Offset: off}
			list, err := conSvc.List(req.Context(), filter)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.With(requirePermission("contacts.read")).Get("/roles", func(w http.ResponseWriter, req *http.Request) { writeJSON(w, http.StatusOK, contacts.Roles()) })
		r.With(requirePermission("contacts.read")).Get("/statuses", func(w http.ResponseWriter, req *http.Request) { writeJSON(w, http.StatusOK, contacts.Statuses()) })
		r.With(requirePermission("contacts.read")).Get("/types", func(w http.ResponseWriter, req *http.Request) { writeJSON(w, http.StatusOK, contacts.Types()) })
		r.With(requirePermission("contacts.write")).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in contacts.ContactCreate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := conSvc.Create(req.Context(), in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("contacts.read")).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			c, err := conSvc.Get(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, c)
		})
		r.With(requirePermission("contacts.write")).Patch("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			var in contacts.ContactUpdate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := conSvc.Update(req.Context(), id, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("contacts.write")).Delete("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			if err := conSvc.DeleteSoft(req.Context(), id); err != nil {
				writeDomainError(w, req, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		// Addresses
		r.With(requirePermission("contacts.read")).Get("/{id}/addresses", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			out, err := conSvc.ListAddresses(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("contacts.write")).Post("/{id}/addresses", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			var in contacts.AddressCreate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := conSvc.CreateAddress(req.Context(), id, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("contacts.write")).Patch("/{id}/addresses/{addrID}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			addrID := chi.URLParam(req, "addrID")
			var in contacts.AddressUpdate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := conSvc.UpdateAddress(req.Context(), id, addrID, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("contacts.write")).Delete("/{id}/addresses/{addrID}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			addrID := chi.URLParam(req, "addrID")
			if err := conSvc.DeleteAddress(req.Context(), id, addrID); err != nil {
				writeDomainError(w, req, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		// Persons
		r.With(requirePermission("contacts.read")).Get("/{id}/persons", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			out, err := conSvc.ListPersons(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("contacts.write")).Post("/{id}/persons", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			var in contacts.PersonCreate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := conSvc.CreatePerson(req.Context(), id, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("contacts.write")).Patch("/{id}/persons/{pid}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			pid := chi.URLParam(req, "pid")
			var in contacts.PersonUpdate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := conSvc.UpdatePerson(req.Context(), id, pid, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("contacts.write")).Delete("/{id}/persons/{pid}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			pid := chi.URLParam(req, "pid")
			if err := conSvc.DeletePerson(req.Context(), id, pid); err != nil {
				writeDomainError(w, req, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		// Notes
		r.With(requirePermission("contacts.read")).Get("/{id}/notes", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			out, err := conSvc.ListNotes(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("contacts.write")).Post("/{id}/notes", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			var in contacts.NoteCreate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := conSvc.CreateNote(req.Context(), id, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("contacts.write")).Patch("/{id}/notes/{noteID}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			noteID := chi.URLParam(req, "noteID")
			var in contacts.NoteUpdate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := conSvc.UpdateNote(req.Context(), id, noteID, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("contacts.write")).Delete("/{id}/notes/{noteID}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			noteID := chi.URLParam(req, "noteID")
			if err := conSvc.DeleteNote(req.Context(), id, noteID); err != nil {
				writeDomainError(w, req, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		// Tasks
		r.With(requirePermission("contacts.read")).Get("/{id}/tasks", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			out, err := conSvc.ListTasks(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("contacts.read")).Get("/{id}/activity", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			out, err := conSvc.ListActivity(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("contacts.read")).Get("/{id}/commercial-context", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			out, err := buildContactCommercialContext(req.Context(), id, quoteSvc, salesSvc, arSvc)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("contacts.write")).Post("/{id}/tasks", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			var in contacts.TaskCreate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := conSvc.CreateTask(req.Context(), id, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("contacts.write")).Patch("/{id}/tasks/{taskID}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			taskID := chi.URLParam(req, "taskID")
			var in contacts.TaskUpdate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := conSvc.UpdateTask(req.Context(), id, taskID, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("contacts.write")).Delete("/{id}/tasks/{taskID}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			taskID := chi.URLParam(req, "taskID")
			if err := conSvc.DeleteTask(req.Context(), id, taskID); err != nil {
				writeDomainError(w, req, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		// Documents
		r.With(requirePermission("contacts.write")).Post("/{id}/documents", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			if err := req.ParseMultipartForm(32 << 20); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültiges Upload-Formular", err)
				return
			}
			file, header, err := req.FormFile("file")
			if err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Datei fehlt (Feld 'file')", err)
				return
			}
			defer file.Close()

			contentType := header.Header.Get("Content-Type")
			doc, err := conSvc.UploadContactDocument(req.Context(), id, file, header.Filename, contentType)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, doc)
		})
		r.With(requirePermission("contacts.read")).Get("/{id}/documents", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			docs, err := conSvc.ListContactDocuments(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, docs)
		})
	})

	// Bestellungen (Purchase Orders)
	protected.Route("/purchase-orders", func(r chi.Router) {
		r.With(requirePermission("purchase_orders.read")).Get("/", func(w http.ResponseWriter, req *http.Request) {
			qv := req.URL.Query()
			lim, off := 0, 0
			if v := qv.Get("limit"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					lim = n
				}
			}
			if v := qv.Get("offset"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					off = n
				}
			}
			f := purchasing.PurchaseOrderFilter{Q: qv.Get("q"), SupplierID: qv.Get("supplier_id"), Status: qv.Get("status"), Limit: lim, Offset: off}
			list, err := poSvc.List(req.Context(), f)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.With(requirePermission("purchase_orders.read")).Get("/statuses", func(w http.ResponseWriter, req *http.Request) { writeJSON(w, http.StatusOK, purchasing.Statuses()) })
		r.With(requirePermission("purchase_orders.write")).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in purchasing.PurchaseOrderCreate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			po, items, err := poSvc.Create(req.Context(), in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"bestellung": po, "positionen": items})
		})
		r.With(requirePermission("purchase_orders.read")).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			po, items, err := poSvc.Get(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"bestellung": po, "positionen": items})
		})
		// PDF-Ausgabe einer Bestellung
		r.With(requirePermission("purchase_orders.read")).Get("/{id}/pdf", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			po, items, err := poSvc.Get(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			// Template-Einstellungen für 'purchase_order'
			t, err := pdfSvc.Get(req.Context(), "purchase_order")
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}
			effectiveTemplate := *t
			primaryColor := "#1F4B99"
			accentColor := "#6B7280"
			if branding, berr := brandingSvc.Get(req.Context()); berr == nil {
				effectiveTemplate = settings.ApplyBrandingDefaults(effectiveTemplate, branding)
				primaryColor = branding.PrimaryColor
				accentColor = branding.AccentColor
			}

			// Daten auf Druckmodell mappen
			date := po.OrderDate.Format("02.01.2006")
			data := pdfgen.PurchaseOrderData{
				Number:    po.Number,
				OrderDate: date,
				Currency:  po.Currency,
				Status:    po.Status,
				Note:      po.Note,
			}
			data.Items = make([]pdfgen.PurchaseOrderItemData, 0, len(items))
			for _, it := range items {
				data.Items = append(data.Items, pdfgen.PurchaseOrderItemData{
					Pos:         it.Position,
					Description: it.Description,
					Qty:         it.Qty,
					UOM:         it.UOM,
					UnitPrice:   it.UnitPrice,
					Currency:    it.Currency,
				})
			}

			// Template-Optionen
			opts := pdfgen.TemplateOptions{
				HeaderText:   effectiveTemplate.HeaderText,
				FooterText:   effectiveTemplate.FooterText,
				TopFirstMM:   effectiveTemplate.TopFirstMM,
				TopOtherMM:   effectiveTemplate.TopOtherMM,
				PrimaryColor: primaryColor,
				AccentColor:  accentColor,
			}
			// Bilder (Logo/Backgrounds) über GridFS laden
			imgIDs := map[string]string{}
			if t.LogoDocID != nil {
				imgIDs["logo"] = *t.LogoDocID
			}
			if t.BgFirstDocID != nil {
				imgIDs["bg_first"] = *t.BgFirstDocID
			}
			if t.BgOtherDocID != nil {
				imgIDs["bg_other"] = *t.BgOtherDocID
			}

			pdfBytes, err := pdfgen.RenderPurchaseOrder(req.Context(), mg, cfg.MongoDB, data, opts, imgIDs)
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}

			// Antwort senden
			filename := fmt.Sprintf("Bestellung_%s.pdf", sanitizeFilename(po.Number))
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
			if _, err := w.Write(pdfBytes); err != nil {
				return
			}
		})
		r.With(requirePermission("purchase_orders.write")).Patch("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			var in purchasing.PurchaseOrderUpdate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			po, items, err := poSvc.Update(req.Context(), id, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"bestellung": po, "positionen": items})
		})
		r.With(requirePermission("purchase_orders.write")).Post("/{id}/items", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			var in purchasing.PurchaseOrderItemInput
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			it, err := poSvc.CreateItem(req.Context(), id, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, it)
		})
		r.With(requirePermission("purchase_orders.write")).Patch("/{id}/items/{itemID}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			itemID := chi.URLParam(req, "itemID")
			var in purchasing.PurchaseOrderItemUpdate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			it, err := poSvc.UpdateItem(req.Context(), id, itemID, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, it)
		})
		r.With(requirePermission("purchase_orders.write")).Delete("/{id}/items/{itemID}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			itemID := chi.URLParam(req, "itemID")
			if err := poSvc.DeleteItem(req.Context(), id, itemID); err != nil {
				writeDomainError(w, req, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})

	protected.Route("/invoices-out", func(r chi.Router) {
		r.With(requirePermission("invoices_out.read")).Get("/", func(w http.ResponseWriter, req *http.Request) {
			q := req.URL.Query()
			lim, off := 0, 0
			if v := q.Get("limit"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					lim = n
				}
			}
			if v := q.Get("offset"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					off = n
				}
			}
			list, err := arSvc.List(req.Context(), accounting.InvoiceFilter{
				Status:             q.Get("status"),
				ContactID:          q.Get("contact_id"),
				SourceSalesOrderID: q.Get("source_sales_order_id"),
				Search:             q.Get("q"),
				Limit:              lim,
				Offset:             off,
			})
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.With(requirePermission("invoices_out.write")).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in accounting.InvoiceOutInput
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := arSvc.Create(req.Context(), in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("invoices_out.read")).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			invoiceID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Rechnungs-ID")
				return
			}
			out, err := arSvc.Get(req.Context(), invoiceID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("invoices_out.write")).Post("/{id}/book", func(w http.ResponseWriter, req *http.Request) {
			invoiceID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Rechnungs-ID")
				return
			}
			out, err := arSvc.Book(req.Context(), invoiceID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("invoices_out.read")).Get("/{id}/payments", func(w http.ResponseWriter, req *http.Request) {
			invoiceID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Rechnungs-ID")
				return
			}
			out, err := paymentSvc.List(req.Context(), invoiceID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("invoices_out.write")).Post("/{id}/payments", func(w http.ResponseWriter, req *http.Request) {
			invoiceID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Rechnungs-ID")
				return
			}
			var in struct {
				Amount    float64   `json:"amount"`
				Currency  string    `json:"currency"`
				Method    string    `json:"method"`
				Reference string    `json:"reference"`
				Date      time.Time `json:"date"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := paymentSvc.Apply(req.Context(), accounting.PaymentInput{
				InvoiceID: invoiceID,
				Amount:    in.Amount,
				Currency:  in.Currency,
				Method:    in.Method,
				Reference: in.Reference,
				Date:      in.Date,
			})
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("invoices_out.read")).Get("/{id}/pdf", func(w http.ResponseWriter, req *http.Request) {
			invoiceID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Rechnungs-ID")
				return
			}
			inv, err := arSvc.Get(req.Context(), invoiceID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}

			t, err := pdfSvc.Get(req.Context(), "invoice_out")
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}
			effectiveTemplate := *t
			primaryColor := "#1F4B99"
			accentColor := "#6B7280"
			if branding, berr := brandingSvc.Get(req.Context()); berr == nil {
				effectiveTemplate = settings.ApplyBrandingDefaults(effectiveTemplate, branding)
				primaryColor = branding.PrimaryColor
				accentColor = branding.AccentColor
			}

			number := invoiceID.String()
			if inv.Number != nil && strings.TrimSpace(*inv.Number) != "" {
				number = *inv.Number
			}
			dueDate := ""
			if inv.DueDate != nil {
				dueDate = inv.DueDate.Format("02.01.2006")
			}
			data := pdfgen.InvoiceOutData{
				Number:      number,
				InvoiceDate: inv.InvoiceDate.Format("02.01.2006"),
				DueDate:     dueDate,
				Currency:    inv.Currency,
				Status:      inv.Status,
				ContactName: inv.ContactName,
				ContactID:   inv.ContactID,
				NetAmount:   inv.NetAmount,
				TaxAmount:   inv.TaxAmount,
				GrossAmount: inv.GrossAmount,
				PaidAmount:  inv.PaidAmount,
				Items:       make([]pdfgen.InvoiceOutItemData, 0, len(inv.Items)),
			}
			for idx, it := range inv.Items {
				data.Items = append(data.Items, pdfgen.InvoiceOutItemData{
					Pos:         idx + 1,
					Description: it.Description,
					Qty:         it.Qty,
					UnitPrice:   it.UnitPrice,
					TaxCode:     it.TaxCode,
					Currency:    inv.Currency,
				})
			}

			opts := pdfgen.TemplateOptions{
				HeaderText:   effectiveTemplate.HeaderText,
				FooterText:   effectiveTemplate.FooterText,
				TopFirstMM:   effectiveTemplate.TopFirstMM,
				TopOtherMM:   effectiveTemplate.TopOtherMM,
				PrimaryColor: primaryColor,
				AccentColor:  accentColor,
			}
			imgIDs := map[string]string{}
			if t.LogoDocID != nil {
				imgIDs["logo"] = *t.LogoDocID
			}
			if t.BgFirstDocID != nil {
				imgIDs["bg_first"] = *t.BgFirstDocID
			}
			if t.BgOtherDocID != nil {
				imgIDs["bg_other"] = *t.BgOtherDocID
			}

			pdfBytes, err := pdfgen.RenderInvoiceOut(req.Context(), mg, cfg.MongoDB, data, opts, imgIDs)
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}

			filename := fmt.Sprintf("Rechnung_%s.pdf", sanitizeFilename(number))
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
			if _, err := w.Write(pdfBytes); err != nil {
				return
			}
		})
	})

	protected.Route("/sales-orders", func(r chi.Router) {
		r.With(requirePermission("sales_orders.read")).Get("/", func(w http.ResponseWriter, req *http.Request) {
			q := req.URL.Query()
			lim, off := 0, 0
			if v := q.Get("limit"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					lim = n
				}
			}
			if v := q.Get("offset"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					off = n
				}
			}
			out, err := salesSvc.List(req.Context(), sales.SalesOrderFilter{
				Status:    q.Get("status"),
				ContactID: q.Get("contact_id"),
				ProjectID: q.Get("project_id"),
				Search:    q.Get("q"),
				Limit:     lim,
				Offset:    off,
			})
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("sales_orders.read")).Get("/statuses", func(w http.ResponseWriter, req *http.Request) {
			writeJSON(w, http.StatusOK, sales.Statuses())
		})
		r.With(requirePermission("sales_orders.read")).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			orderID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Auftrags-ID")
				return
			}
			out, err := salesSvc.Get(req.Context(), orderID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("sales_orders.write")).Patch("/{id}", func(w http.ResponseWriter, req *http.Request) {
			orderID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Auftrags-ID")
				return
			}
			var in sales.SalesOrderUpdate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := salesSvc.Update(req.Context(), orderID, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("sales_orders.read")).Get("/{id}/pdf", func(w http.ResponseWriter, req *http.Request) {
			orderID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Auftrags-ID")
				return
			}
			order, err := salesSvc.Get(req.Context(), orderID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}

			t, err := pdfSvc.Get(req.Context(), "sales_order")
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}
			effectiveTemplate := *t
			primaryColor := "#1F4B99"
			accentColor := "#6B7280"
			if branding, berr := brandingSvc.Get(req.Context()); berr == nil {
				effectiveTemplate = settings.ApplyBrandingDefaults(effectiveTemplate, branding)
				primaryColor = branding.PrimaryColor
				accentColor = branding.AccentColor
			}

			data := pdfgen.SalesOrderData{
				Number:          order.Number,
				OrderDate:       order.OrderDate.Format("02.01.2006"),
				Status:          order.Status,
				ProjectName:     order.ProjectName,
				CustomerName:    order.ContactName,
				SourceQuoteID:   order.SourceQuoteID.String(),
				LinkedInvoiceID: order.LinkedInvoiceOutID,
				Currency:        order.Currency,
				Note:            order.Note,
				NetAmount:       order.NetAmount,
				TaxAmount:       order.TaxAmount,
				GrossAmount:     order.GrossAmount,
				Items:           make([]pdfgen.SalesOrderItemData, 0, len(order.Items)),
			}
			for idx, it := range order.Items {
				data.Items = append(data.Items, pdfgen.SalesOrderItemData{
					Pos:         idx + 1,
					Description: it.Description,
					Qty:         it.Qty,
					Unit:        it.Unit,
					UnitPrice:   it.UnitPrice,
					TaxCode:     it.TaxCode,
					Currency:    order.Currency,
				})
			}

			opts := pdfgen.TemplateOptions{
				HeaderText:   effectiveTemplate.HeaderText,
				FooterText:   effectiveTemplate.FooterText,
				TopFirstMM:   effectiveTemplate.TopFirstMM,
				TopOtherMM:   effectiveTemplate.TopOtherMM,
				PrimaryColor: primaryColor,
				AccentColor:  accentColor,
			}
			imgIDs := map[string]string{}
			if t.LogoDocID != nil {
				imgIDs["logo"] = *t.LogoDocID
			}
			if t.BgFirstDocID != nil {
				imgIDs["bg_first"] = *t.BgFirstDocID
			}
			if t.BgOtherDocID != nil {
				imgIDs["bg_other"] = *t.BgOtherDocID
			}

			pdfBytes, err := pdfgen.RenderSalesOrder(req.Context(), mg, cfg.MongoDB, data, opts, imgIDs)
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}

			filename := fmt.Sprintf("Auftrag_%s.pdf", sanitizeFilename(order.Number))
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
			if _, err := w.Write(pdfBytes); err != nil {
				return
			}
		})
		r.With(requirePermission("sales_orders.write")).Post("/{id}/status", func(w http.ResponseWriter, req *http.Request) {
			orderID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Auftrags-ID")
				return
			}
			var in struct {
				Status string `json:"status"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := salesSvc.UpdateStatus(req.Context(), orderID, in.Status)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("sales_orders.write")).Post("/{id}/items", func(w http.ResponseWriter, req *http.Request) {
			orderID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Auftrags-ID")
				return
			}
			var in sales.SalesOrderItemInput
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			item, order, err := salesSvc.CreateItem(req.Context(), orderID, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"item": item, "sales_order": order})
		})
		r.With(requirePermission("sales_orders.write")).Patch("/{id}/items/{itemID}", func(w http.ResponseWriter, req *http.Request) {
			orderID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Auftrags-ID")
				return
			}
			itemID, err := uuid.Parse(chi.URLParam(req, "itemID"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Positions-ID")
				return
			}
			var in sales.SalesOrderItemUpdate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			item, order, err := salesSvc.UpdateItem(req.Context(), orderID, itemID, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"item": item, "sales_order": order})
		})
		r.With(requirePermission("sales_orders.write")).Delete("/{id}/items/{itemID}", func(w http.ResponseWriter, req *http.Request) {
			orderID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Auftrags-ID")
				return
			}
			itemID, err := uuid.Parse(chi.URLParam(req, "itemID"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Positions-ID")
				return
			}
			order, err := salesSvc.DeleteItem(req.Context(), orderID, itemID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, order)
		})
		r.With(requirePermission("sales_orders.write"), requirePermission("invoices_out.write")).Post("/{id}/convert-to-invoice", func(w http.ResponseWriter, req *http.Request) {
			orderID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Auftrags-ID")
				return
			}
			var in sales.ConvertToInvoiceInput
			if req.Body != nil {
				if err := json.NewDecoder(req.Body).Decode(&in); err != nil && err != io.EOF {
					writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
					return
				}
			}
			out, err := salesSvc.ConvertToInvoice(req.Context(), orderID, arSvc, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
	})

	protected.Route("/quotes", func(r chi.Router) {
		r.With(requirePermission("quotes.read")).Get("/imports", func(w http.ResponseWriter, req *http.Request) {
			q := req.URL.Query()
			lim, off := 0, 0
			if v := q.Get("limit"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					lim = n
				}
			}
			if v := q.Get("offset"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					off = n
				}
			}
			list, err := quoteSvc.ListImports(req.Context(), quotes.QuoteImportFilter{
				ProjectID: q.Get("project_id"),
				ContactID: q.Get("contact_id"),
				Limit:     lim,
				Offset:    off,
			})
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.With(requirePermission("quotes.write")).Post("/imports/gaeb", func(w http.ResponseWriter, req *http.Request) {
			if err := req.ParseMultipartForm(32 << 20); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültiges Upload-Formular", err)
				return
			}
			file, header, err := req.FormFile("file")
			if err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Datei fehlt (Feld 'file')", err)
				return
			}
			defer file.Close()
			out, err := quoteSvc.CreateGAEBImport(req.Context(), quotes.QuoteImportCreateInput{
				ProjectID: req.FormValue("project_id"),
				ContactID: req.FormValue("contact_id"),
			}, file, header.Filename)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("quotes.read")).Get("/imports/{id}", func(w http.ResponseWriter, req *http.Request) {
			out, err := quoteSvc.GetImport(req.Context(), chi.URLParam(req, "id"))
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("quotes.read")).Get("/imports/{id}/items", func(w http.ResponseWriter, req *http.Request) {
			out, err := quoteSvc.ListImportItems(req.Context(), chi.URLParam(req, "id"))
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("quotes.read")).Get("/imports/{id}/items/{itemID}", func(w http.ResponseWriter, req *http.Request) {
			out, err := quoteSvc.GetImportItem(req.Context(), chi.URLParam(req, "id"), chi.URLParam(req, "itemID"))
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("quotes.write")).Patch("/imports/{id}/review", func(w http.ResponseWriter, req *http.Request) {
			out, err := quoteSvc.MarkImportReviewed(req.Context(), chi.URLParam(req, "id"))
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("quotes.write")).Patch("/imports/{id}/items/{itemID}/review", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				ReviewStatus string `json:"review_status"`
				ReviewNote   string `json:"review_note"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := quoteSvc.UpdateImportItemReview(
				req.Context(),
				chi.URLParam(req, "id"),
				chi.URLParam(req, "itemID"),
				in.ReviewStatus,
				in.ReviewNote,
			)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("quotes.write")).Post("/imports/{id}/apply", func(w http.ResponseWriter, req *http.Request) {
			out, err := quoteSvc.ApplyImportToDraftQuote(req.Context(), chi.URLParam(req, "id"))
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("quotes.read")).Get("/", func(w http.ResponseWriter, req *http.Request) {
			q := req.URL.Query()
			lim, off := 0, 0
			if v := q.Get("limit"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					lim = n
				}
			}
			if v := q.Get("offset"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					off = n
				}
			}
			list, err := quoteSvc.List(req.Context(), quotes.QuoteFilter{
				Status:    q.Get("status"),
				ContactID: q.Get("contact_id"),
				ProjectID: q.Get("project_id"),
				Search:    q.Get("q"),
				Limit:     lim,
				Offset:    off,
			})
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.With(requirePermission("quotes.write")).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in quotes.QuoteInput
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := quoteSvc.Create(req.Context(), in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("quotes.read")).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			quoteID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Angebots-ID")
				return
			}
			out, err := quoteSvc.Get(req.Context(), quoteID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("quotes.write")).Patch("/{id}", func(w http.ResponseWriter, req *http.Request) {
			quoteID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Angebots-ID")
				return
			}
			var in quotes.QuoteInput
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := quoteSvc.Update(req.Context(), quoteID, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("quotes.write")).Post("/{id}/items/{itemID}/apply-material-candidate", func(w http.ResponseWriter, req *http.Request) {
			quoteID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Angebots-ID")
				return
			}
			itemID, err := uuid.Parse(chi.URLParam(req, "itemID"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Positions-ID")
				return
			}
			var in struct {
				MaterialID string `json:"material_id"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := quoteSvc.ApplyMaterialCandidate(req.Context(), quoteID, itemID, in.MaterialID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("quotes.write")).Post("/{id}/status", func(w http.ResponseWriter, req *http.Request) {
			quoteID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Angebots-ID")
				return
			}
			var in struct {
				Status string `json:"status"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := quoteSvc.UpdateStatus(req.Context(), quoteID, in.Status)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("quotes.write")).Post("/{id}/accept", func(w http.ResponseWriter, req *http.Request) {
			quoteID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Angebots-ID")
				return
			}
			var in quotes.AcceptInput
			if req.Body != nil {
				if err := json.NewDecoder(req.Body).Decode(&in); err != nil && err != io.EOF {
					writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
					return
				}
			}
			if strings.TrimSpace(in.ProjectStatus) != "" {
				permissions, ok := authPermissionsFromContext(req.Context())
				if !ok {
					writeAPIError(w, req, http.StatusForbidden, "forbidden", "Berechtigungen fehlen")
					return
				}
				hasProjectsWrite := false
				for _, permission := range permissions {
					if permission == "projects.write" || permission == "users.manage" {
						hasProjectsWrite = true
						break
					}
				}
				if !hasProjectsWrite {
					writeAPIError(w, req, http.StatusForbidden, "forbidden", "Projektstatus darf nicht geändert werden")
					return
				}
			}
			out, err := quoteSvc.Accept(req.Context(), quoteID, projSvc, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("quotes.write")).Post("/{id}/revise", func(w http.ResponseWriter, req *http.Request) {
			quoteID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Angebots-ID")
				return
			}
			out, err := quoteSvc.Revise(req.Context(), quoteID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("quotes.write"), requirePermission("invoices_out.write")).Post("/{id}/convert-to-invoice", func(w http.ResponseWriter, req *http.Request) {
			quoteID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Angebots-ID")
				return
			}
			var in quotes.ConvertToInvoiceInput
			if req.Body != nil {
				if err := json.NewDecoder(req.Body).Decode(&in); err != nil && err != io.EOF {
					writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
					return
				}
			}
			out, err := quoteSvc.ConvertToInvoice(req.Context(), quoteID, arSvc, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("quotes.write"), requirePermission("sales_orders.write")).Post("/{id}/convert-to-sales-order", func(w http.ResponseWriter, req *http.Request) {
			quoteID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Angebots-ID")
				return
			}
			out, err := salesSvc.CreateFromQuote(req.Context(), quoteID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("quotes.read")).Get("/{id}/pdf", func(w http.ResponseWriter, req *http.Request) {
			quoteID, err := uuid.Parse(chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Angebots-ID")
				return
			}
			qt, err := quoteSvc.Get(req.Context(), quoteID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}

			t, err := pdfSvc.Get(req.Context(), "quote")
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}
			effectiveTemplate := *t
			primaryColor := "#1F4B99"
			accentColor := "#6B7280"
			if branding, berr := brandingSvc.Get(req.Context()); berr == nil {
				effectiveTemplate = settings.ApplyBrandingDefaults(effectiveTemplate, branding)
				primaryColor = branding.PrimaryColor
				accentColor = branding.AccentColor
			}

			data := pdfgen.QuoteData{
				Number:        qt.Number,
				ProjectName:   qt.ProjectName,
				ProjectStatus: qt.Status,
				CreatedDate:   qt.QuoteDate.Format("02.01.2006"),
				CustomerName:  qt.ContactName,
				Currency:      qt.Currency,
				Note:          qt.Note,
				NetAmount:     qt.NetAmount,
				TaxAmount:     qt.TaxAmount,
				GrossAmount:   qt.GrossAmount,
				Items:         make([]pdfgen.QuoteItemData, 0, len(qt.Items)),
			}
			if qt.ValidUntil != nil {
				data.ValidUntil = qt.ValidUntil.Format("02.01.2006")
			}
			for idx, item := range qt.Items {
				data.Items = append(data.Items, pdfgen.QuoteItemData{
					Pos:         idx + 1,
					Description: item.Description,
					Qty:         item.Qty,
					Unit:        item.Unit,
					UnitPrice:   item.UnitPrice,
					TaxCode:     item.TaxCode,
					LineTotal:   item.Qty * item.UnitPrice,
				})
			}

			opts := pdfgen.TemplateOptions{
				HeaderText:   effectiveTemplate.HeaderText,
				FooterText:   effectiveTemplate.FooterText,
				TopFirstMM:   effectiveTemplate.TopFirstMM,
				TopOtherMM:   effectiveTemplate.TopOtherMM,
				PrimaryColor: primaryColor,
				AccentColor:  accentColor,
			}
			imgIDs := map[string]string{}
			if t.LogoDocID != nil {
				imgIDs["logo"] = *t.LogoDocID
			}
			if t.BgFirstDocID != nil {
				imgIDs["bg_first"] = *t.BgFirstDocID
			}
			if t.BgOtherDocID != nil {
				imgIDs["bg_other"] = *t.BgOtherDocID
			}
			pdfBytes, err := pdfgen.RenderQuote(req.Context(), mg, cfg.MongoDB, data, opts, imgIDs)
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}

			filename := fmt.Sprintf("Angebot_%s.pdf", sanitizeFilename(qt.Number))
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
			if _, err := w.Write(pdfBytes); err != nil {
				return
			}
		})
	})

	// Projekte (basic CRUD – MVP)
	protected.Route("/projects", func(r chi.Router) {
		// Logikal-Import (SQLite Upload)
		r.With(requirePermission("projects.write")).Post("/import/logikal", func(w http.ResponseWriter, req *http.Request) {
			log.Printf("[Import] Start %s len=%d ct=%s", req.Method, req.ContentLength, req.Header.Get("Content-Type"))
			ct := req.Header.Get("Content-Type")
			// temporäre Datei
			tmp, err := os.CreateTemp("", "logikal-*.sqlite")
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}
			tmpName := tmp.Name()
			var srcName string
			if strings.HasPrefix(ct, "multipart/form-data") {
				// bis zu 256MB im Speicher für Formdaten
				if err := req.ParseMultipartForm(256 << 20); err != nil {
					writeHTTPError(w, req, http.StatusBadRequest, "Ungültiges Upload-Formular", err)
					return
				}
				f, header, err := req.FormFile("file")
				if err != nil {
					writeHTTPError(w, req, http.StatusBadRequest, "Datei fehlt (Feld 'file')", err)
					return
				}
				defer f.Close()
				srcName = header.Filename
				if _, err := io.Copy(tmp, f); err != nil {
					writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
					return
				}
				_ = tmp.Close()
			} else {
				// Rohdaten-Upload (application/octet-stream)
				srcName = req.Header.Get("X-Filename")
				if _, err := io.Copy(tmp, req.Body); err != nil {
					writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
					return
				}
				_ = tmp.Close()
			}
			defer os.Remove(tmpName)
			if fi, err := os.Stat(tmpName); err == nil {
				log.Printf("[Import] temp file %s size=%d src=%s", tmpName, fi.Size(), srcName)
			}

			proj, importID, err := projSvc.ImportLogikal(req.Context(), tmpName, srcName)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"projekt": proj, "quelle": srcName, "import_id": importID})
		})

		// Analyse Logikal (ohne Import) – liefert eine Zusammenfassung
		r.With(requirePermission("projects.write")).Post("/analyze/logikal", func(w http.ResponseWriter, req *http.Request) {
			ct := req.Header.Get("Content-Type")
			tmp, err := os.CreateTemp("", "logikal-analyze-*.sqlite")
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}
			tmpName := tmp.Name()
			var srcName string
			if strings.HasPrefix(ct, "multipart/form-data") {
				if err := req.ParseMultipartForm(64 << 20); err != nil {
					writeHTTPError(w, req, http.StatusBadRequest, "Ungültiges Upload-Formular", err)
					return
				}
				f, header, err := req.FormFile("file")
				if err != nil {
					writeHTTPError(w, req, http.StatusBadRequest, "Datei fehlt (Feld 'file')", err)
					return
				}
				defer f.Close()
				srcName = header.Filename
				if _, err := io.Copy(tmp, f); err != nil {
					writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
					return
				}
				_ = tmp.Close()
			} else {
				srcName = req.Header.Get("X-Filename")
				if _, err := io.Copy(tmp, req.Body); err != nil {
					writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
					return
				}
				_ = tmp.Close()
			}
			defer os.Remove(tmpName)

			summary, err := projSvc.AnalyzeLogikal(req.Context(), tmpName)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			summary["source"] = srcName
			writeJSON(w, http.StatusOK, summary)
		})

		// Import-Protokolle
		r.With(requirePermission("projects.read")).Get("/{id}/imports", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			list, err := projSvc.ListImports(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.With(requirePermission("projects.read")).Get("/{id}/imports/{importID}/changes", func(w http.ResponseWriter, req *http.Request) {
			importID := chi.URLParam(req, "importID")
			q := req.URL.Query()
			format := strings.ToLower(q.Get("format"))
			kind := q.Get("kind")
			action := q.Get("action")
			if format == "csv" {
				csv, err := projSvc.ExportImportChangesCSV(req.Context(), importID)
				if err != nil {
					writeDomainError(w, req, err)
					return
				}
				w.Header().Set("Content-Type", "text/csv; charset=utf-8")
				w.Header().Set("Content-Disposition", "attachment; filename=import-"+importID+".csv")
				_, _ = io.WriteString(w, csv)
				return
			}
			var list []projects.ImportChange
			var err error
			if kind != "" || action != "" {
				list, err = projSvc.ListImportChangesFiltered(req.Context(), importID, kind, action)
			} else {
				list, err = projSvc.ListImportChanges(req.Context(), importID)
			}
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.With(requirePermission("projects.write")).Post("/{id}/imports/{importID}/undo", func(w http.ResponseWriter, req *http.Request) {
			importID := chi.URLParam(req, "importID")
			if err := projSvc.UndoImport(req.Context(), importID); err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "undone_import": importID})
		})
		r.With(requirePermission("projects.read")).Get("/", func(w http.ResponseWriter, req *http.Request) {
			qv := req.URL.Query()
			lim, off := 0, 0
			if v := qv.Get("limit"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					lim = n
				}
			}
			if v := qv.Get("offset"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					off = n
				}
			}
			f := projects.ProjectFilter{Q: qv.Get("q"), Status: qv.Get("status"), Limit: lim, Offset: off}
			list, err := projSvc.List(req.Context(), f)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.With(requirePermission("projects.write")).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in projects.ProjectCreate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := projSvc.Create(req.Context(), in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("projects.read")).Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			out, err := projSvc.Get(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("projects.read")).Get("/{id}/commercial-context", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			out, err := buildProjectCommercialContext(req.Context(), id, pg, quoteSvc, salesSvc)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("projects.read")).Get("/{id}/quote-pdf", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			snapshot, err := projSvc.BuildQuoteSnapshot(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}

			t, err := pdfSvc.Get(req.Context(), "quote")
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}
			effectiveTemplate := *t
			primaryColor := "#1F4B99"
			accentColor := "#6B7280"
			if branding, berr := brandingSvc.Get(req.Context()); berr == nil {
				effectiveTemplate = settings.ApplyBrandingDefaults(effectiveTemplate, branding)
				primaryColor = branding.PrimaryColor
				accentColor = branding.AccentColor
			}

			data := pdfgen.QuoteData{
				Number:        snapshot.Project.Nummer,
				ProjectName:   snapshot.Project.Name,
				ProjectStatus: snapshot.Project.Status,
				CreatedDate:   snapshot.Project.Angelegt.Format("02.01.2006"),
				CustomerName:  snapshot.CustomerName,
				CustomerEmail: snapshot.CustomerEmail,
				CustomerPhone: snapshot.CustomerPhone,
				Items:         make([]pdfgen.QuoteItemData, 0, len(snapshot.Positionen)),
			}
			for idx, line := range snapshot.Positionen {
				descriptionParts := []string{}
				positionLabel := strings.TrimSpace(strings.TrimSpace(line.PositionsNummer) + " " + strings.TrimSpace(line.PositionsName))
				if positionLabel != "" {
					descriptionParts = append(descriptionParts, positionLabel)
				}
				if strings.TrimSpace(line.Beschreibung) != "" {
					descriptionParts = append(descriptionParts, line.Beschreibung)
				}
				surfaceParts := []string{}
				if strings.TrimSpace(line.Serie) != "" {
					surfaceParts = append(surfaceParts, line.Serie)
				}
				if strings.TrimSpace(line.Oberflaeche) != "" {
					surfaceParts = append(surfaceParts, line.Oberflaeche)
				}
				data.Items = append(data.Items, pdfgen.QuoteItemData{
					Pos:             idx + 1,
					PhaseLabel:      strings.TrimSpace(strings.TrimSpace(line.PhaseNummer) + " " + strings.TrimSpace(line.PhaseName)),
					Description:     strings.Join(descriptionParts, " - "),
					Qty:             line.Menge,
					Unit:            "Stk",
					DimensionsLabel: projectQuoteDimensionsLabel(line.WidthMM, line.HeightMM),
					SurfaceLabel:    strings.Join(surfaceParts, " / "),
				})
			}

			opts := pdfgen.TemplateOptions{
				HeaderText:   effectiveTemplate.HeaderText,
				FooterText:   effectiveTemplate.FooterText,
				TopFirstMM:   effectiveTemplate.TopFirstMM,
				TopOtherMM:   effectiveTemplate.TopOtherMM,
				PrimaryColor: primaryColor,
				AccentColor:  accentColor,
			}
			imgIDs := map[string]string{}
			if t.LogoDocID != nil {
				imgIDs["logo"] = *t.LogoDocID
			}
			if t.BgFirstDocID != nil {
				imgIDs["bg_first"] = *t.BgFirstDocID
			}
			if t.BgOtherDocID != nil {
				imgIDs["bg_other"] = *t.BgOtherDocID
			}

			pdfBytes, err := pdfgen.RenderQuote(req.Context(), mg, cfg.MongoDB, data, opts, imgIDs)
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}

			filename := fmt.Sprintf("Angebot_%s.pdf", sanitizeFilename(snapshot.Project.Nummer))
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
			if _, err := w.Write(pdfBytes); err != nil {
				return
			}
		})

		// Lose (Phases)
		r.With(requirePermission("projects.read")).Get("/{id}/phases", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			list, err := projSvc.ListPhases(req.Context(), id)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.With(requirePermission("projects.write")).Post("/{id}/phases", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			var in projects.PhaseCreate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := projSvc.CreatePhase(req.Context(), id, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("projects.read")).Get("/{id}/phases/{phaseID}", func(w http.ResponseWriter, req *http.Request) {
			phaseID := chi.URLParam(req, "phaseID")
			out, err := projSvc.GetPhase(req.Context(), phaseID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("projects.write")).Patch("/{id}/phases/{phaseID}", func(w http.ResponseWriter, req *http.Request) {
			phaseID := chi.URLParam(req, "phaseID")
			var in projects.PhaseUpdate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := projSvc.UpdatePhase(req.Context(), phaseID, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("projects.write")).Delete("/{id}/phases/{phaseID}", func(w http.ResponseWriter, req *http.Request) {
			phaseID := chi.URLParam(req, "phaseID")
			if err := projSvc.DeletePhase(req.Context(), phaseID); err != nil {
				writeDomainError(w, req, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		// Elevations je Phase
		r.With(requirePermission("projects.read")).Get("/{id}/phases/{phaseID}/elevations", func(w http.ResponseWriter, req *http.Request) {
			phaseID := chi.URLParam(req, "phaseID")
			list, err := projSvc.ListElevations(req.Context(), phaseID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.With(requirePermission("projects.write")).Post("/{id}/phases/{phaseID}/elevations", func(w http.ResponseWriter, req *http.Request) {
			phaseID := chi.URLParam(req, "phaseID")
			var in projects.ElevationCreate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := projSvc.CreateElevation(req.Context(), phaseID, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("projects.read")).Get("/{id}/phases/{phaseID}/elevations/{elevID}", func(w http.ResponseWriter, req *http.Request) {
			elevID := chi.URLParam(req, "elevID")
			out, err := projSvc.GetElevation(req.Context(), elevID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("projects.write")).Patch("/{id}/phases/{phaseID}/elevations/{elevID}", func(w http.ResponseWriter, req *http.Request) {
			elevID := chi.URLParam(req, "elevID")
			var in projects.ElevationUpdate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := projSvc.UpdateElevation(req.Context(), elevID, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("projects.write")).Delete("/{id}/phases/{phaseID}/elevations/{elevID}", func(w http.ResponseWriter, req *http.Request) {
			elevID := chi.URLParam(req, "elevID")
			if err := projSvc.DeleteElevation(req.Context(), elevID); err != nil {
				writeDomainError(w, req, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		// Ausführungsvarianten je Elevation
		r.With(requirePermission("projects.read")).Get("/{id}/elevations/{elevID}/single-elevations", func(w http.ResponseWriter, req *http.Request) {
			elevID := chi.URLParam(req, "elevID")
			list, err := projSvc.ListSingleElevations(req.Context(), elevID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.With(requirePermission("projects.write")).Post("/{id}/elevations/{elevID}/single-elevations", func(w http.ResponseWriter, req *http.Request) {
			elevID := chi.URLParam(req, "elevID")
			var in projects.SingleElevationCreate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := projSvc.CreateSingleElevation(req.Context(), elevID, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("projects.read")).Get("/{id}/elevations/{elevID}/single-elevations/{sid}", func(w http.ResponseWriter, req *http.Request) {
			sid := chi.URLParam(req, "sid")
			out, err := projSvc.GetSingleElevation(req.Context(), sid)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("projects.write")).Patch("/{id}/elevations/{elevID}/single-elevations/{sid}", func(w http.ResponseWriter, req *http.Request) {
			sid := chi.URLParam(req, "sid")
			var in projects.SingleElevationUpdate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			out, err := projSvc.UpdateSingleElevation(req.Context(), sid, in)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("projects.write")).Delete("/{id}/elevations/{elevID}/single-elevations/{sid}", func(w http.ResponseWriter, req *http.Request) {
			sid := chi.URLParam(req, "sid")
			if err := projSvc.DeleteSingleElevation(req.Context(), sid); err != nil {
				writeDomainError(w, req, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		// Materiallisten je Single-Elevation (Profile, Articles, Glass)
		r.With(requirePermission("projects.read")).Get("/{id}/single-elevations/{sid}/materials", func(w http.ResponseWriter, req *http.Request) {
			sid := chi.URLParam(req, "sid")
			profiles, err := projSvc.ListProfilesBySingle(req.Context(), sid)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			articles, err := projSvc.ListArticlesBySingle(req.Context(), sid)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			glass, err := projSvc.ListGlassBySingle(req.Context(), sid)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"profiles": profiles,
				"articles": articles,
				"glass":    glass,
			})
		})

		// Projekt-Assets (Zip mit Emfs/Rtfs) hochladen und abrufen
		r.With(requirePermission("projects.write")).Post("/{id}/assets", func(w http.ResponseWriter, req *http.Request) {
			pid := chi.URLParam(req, "id")
			if err := req.ParseMultipartForm(512 << 20); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültiges Formular", err)
				return
			}
			file, _, err := req.FormFile("file")
			if err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Datei fehlt (Feld 'file')", err)
				return
			}
			defer file.Close()
			// ZIP in Speicher lesen
			buf, err := io.ReadAll(file)
			if err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Upload lesen fehlgeschlagen", err)
				return
			}
			zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
			if err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültiges ZIP", err)
				return
			}
			// GridFS Bucket öffnen
			db := mg.Database(cfg.MongoDB)
			bucket, err := gridfs.NewBucket(db)
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, "GridFS nicht verfügbar", err)
				return
			}
			type item struct {
				Rel string `json:"rel"`
			}
			saved := make([]item, 0)
			converted := make([]item, 0)
			skipped := make([]item, 0)
			// Dateien iterieren
			for _, f := range zr.File {
				name := f.Name
				low := strings.ToLower(name)
				// Relativer Pfad ab Emfs/ oder Rtfs/ (case-insensitive, auch wenn ein Top-Level-Ordner vorangestellt ist)
				idx := strings.Index(low, "/emfs/")
				if idx < 0 {
					idx = strings.Index(low, "/rtfs/")
				}
				if idx < 0 {
					if strings.HasPrefix(low, "emfs/") || strings.HasPrefix(low, "rtfs/") {
						idx = -1
					} else {
						skipped = append(skipped, item{Rel: name})
						continue
					}
				}
				rel := name
				if idx >= 0 {
					rel = name[idx+1:]
				}
				rc, err := f.Open()
				if err != nil {
					continue
				}
				oid, err := bucket.UploadFromStream(rel, rc)
				rc.Close()
				if err != nil {
					continue
				}
				// ContentType heuristisch
				ct := "application/octet-stream"
				if strings.HasSuffix(strings.ToLower(rel), ".png") {
					ct = "image/png"
				} else if strings.HasSuffix(strings.ToLower(rel), ".jpg") || strings.HasSuffix(strings.ToLower(rel), ".jpeg") {
					ct = "image/jpeg"
				} else if strings.HasSuffix(strings.ToLower(rel), ".svg") {
					ct = "image/svg+xml"
				} else if strings.HasSuffix(strings.ToLower(rel), ".emf") {
					ct = "image/emf"
				}
				// Länge aus fs.files lesen
				var length int64
				var storedName string = name
				var meta struct {
					Length   int64  `bson:"length"`
					Filename string `bson:"filename"`
				}
				_ = db.Collection("fs.files").FindOne(req.Context(), bson.M{"_id": oid}).Decode(&meta)
				if meta.Length > 0 {
					length = meta.Length
				}
				if meta.Filename != "" {
					storedName = meta.Filename
				}
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
											var meta2 struct {
												Length   int64  `bson:"length"`
												Filename string `bson:"filename"`
											}
											_ = db.Collection("fs.files").FindOne(req.Context(), bson.M{"_id": oid2}).Decode(&meta2)
											_ = projSvc.UpsertProjectAsset(req.Context(), pid, base, oid2.Hex(), base, "image/png", meta2.Length)
											log.Printf("assets: konvertiert(inkscape) %s -> %s (%d bytes)", rel, base, meta2.Length)
											converted = append(converted, item{Rel: base})
										} else {
											_ = pf.Close()
											log.Printf("assets: upload png fehlgeschlagen (inkscape): %v", errUp)
										}
									} else {
										log.Printf("assets: open png fehlgeschlagen (inkscape): %v", errOpen)
									}
								} else {
									log.Printf("assets: konvertierung fehlgeschlagen (inkscape)")
								}
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
											var meta2 struct {
												Length   int64  `bson:"length"`
												Filename string `bson:"filename"`
											}
											_ = db.Collection("fs.files").FindOne(req.Context(), bson.M{"_id": oid2}).Decode(&meta2)
											_ = projSvc.UpsertProjectAsset(req.Context(), pid, base, oid2.Hex(), base, "image/png", meta2.Length)
											log.Printf("assets: konvertiert(convert) %s -> %s (%d bytes)", rel, base, meta2.Length)
											converted = append(converted, item{Rel: base})
										} else {
											_ = pf.Close()
											log.Printf("assets: upload png fehlgeschlagen (convert): %v", errUp)
										}
									} else {
										log.Printf("assets: open png fehlgeschlagen (convert): %v", errOpen)
									}
								} else {
									log.Printf("assets: konvertierung fehlgeschlagen (convert)")
								}
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
											var meta2 struct {
												Length   int64  `bson:"length"`
												Filename string `bson:"filename"`
											}
											_ = db.Collection("fs.files").FindOne(req.Context(), bson.M{"_id": oid2}).Decode(&meta2)
											_ = projSvc.UpsertProjectAsset(req.Context(), pid, base, oid2.Hex(), base, "image/png", meta2.Length)
											log.Printf("assets: konvertiert(magick) %s -> %s (%d bytes)", rel, base, meta2.Length)
											converted = append(converted, item{Rel: base})
										} else {
											_ = pf.Close()
											log.Printf("assets: upload png fehlgeschlagen (magick): %v", errUp)
										}
									} else {
										log.Printf("assets: open png fehlgeschlagen (magick): %v", errOpen)
									}
								} else {
									log.Printf("assets: konvertierung fehlgeschlagen (magick)")
								}
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
													var meta2 struct {
														Length   int64  `bson:"length"`
														Filename string `bson:"filename"`
													}
													_ = db.Collection("fs.files").FindOne(req.Context(), bson.M{"_id": oid2}).Decode(&meta2)
													_ = projSvc.UpsertProjectAsset(req.Context(), pid, base, oid2.Hex(), base, "image/png", meta2.Length)
													log.Printf("assets: konvertiert(soffice+magick) %s -> %s (%d bytes)", rel, base, meta2.Length)
													converted = append(converted, item{Rel: base})
												} else {
													_ = pf.Close()
													log.Printf("assets: upload png fehlgeschlagen (soffice+magick): %v", errUp)
												}
											} else {
												log.Printf("assets: open png fehlgeschlagen (soffice+magick): %v", errOpen)
											}
										} else {
											log.Printf("assets: pdf->png fehlgeschlagen (magick): %v", err3)
										}
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
												var meta2 struct {
													Length   int64  `bson:"length"`
													Filename string `bson:"filename"`
												}
												_ = db.Collection("fs.files").FindOne(req.Context(), bson.M{"_id": oid2}).Decode(&meta2)
												_ = projSvc.UpsertProjectAsset(req.Context(), pid, base, oid2.Hex(), base, "image/png", meta2.Length)
												log.Printf("assets: konvertiert(soffice) %s -> %s (%d bytes)", rel, base, meta2.Length)
												converted = append(converted, item{Rel: base})
											} else {
												_ = pf.Close()
												log.Printf("assets: upload png fehlgeschlagen (soffice): %v", errUp)
											}
										} else {
											log.Printf("assets: open png fehlgeschlagen (soffice): %v", errOpen)
										}
									} else {
										log.Printf("assets: konvertierung fehlgeschlagen (soffice png): %v", err2)
									}
								}
								if profDir != "" {
									_ = os.RemoveAll(profDir)
								}
							} else {
								log.Printf("assets: kein Konvertierungstool gefunden (inkscape/convert/magick/soffice)")
							}
						}
						if emfFile != nil {
							_ = os.Remove(emfFile.Name())
						}
						if pngFile != nil {
							_ = os.Remove(pngFile.Name())
						}
					}
				}
			}
			if req.URL.Query().Get("summary") == "1" {
				writeJSON(w, http.StatusOK, map[string]any{"saved": saved, "converted": converted, "skipped": skipped})
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
		})
		r.With(requirePermission("projects.read")).Get("/{id}/assets/list", func(w http.ResponseWriter, req *http.Request) {
			pid := chi.URLParam(req, "id")
			list, err := projSvc.ListProjectAssets(req.Context(), pid)
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.With(requirePermission("projects.read")).Get("/{id}/assets", func(w http.ResponseWriter, req *http.Request) {
			pid := chi.URLParam(req, "id")
			rel := strings.TrimSpace(req.URL.Query().Get("path"))
			if rel == "" {
				writeHTTPError(w, req, http.StatusBadRequest, "path erforderlich", nil)
				return
			}
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
			if err != nil {
				writeHTTPError(w, req, http.StatusNotFound, "Asset nicht gefunden", err)
				return
			}
			// GridFS stream öffnen
			db := mg.Database(cfg.MongoDB)
			bucket, err := gridfs.NewBucket(db)
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, "GridFS nicht verfügbar", err)
				return
			}
			oid, err := primitive.ObjectIDFromHex(gridID)
			if err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültige ID", err)
				return
			}
			rc, err := bucket.OpenDownloadStream(oid)
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, "Stream fehlgeschlagen", err)
				return
			}
			defer rc.Close()
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			w.Header().Set("Content-Type", contentType)
			if filename != "" {
				w.Header().Set("Content-Disposition", "inline; filename=\""+filename+"\"")
			}
			if length > 0 {
				w.Header().Set("Content-Length", fmt.Sprintf("%d", length))
			}
			_, _ = io.Copy(w, rc)
		})
		// Link eines Materials aus der Varianten-Materialliste zu Stammmaterial setzen
		r.With(requirePermission("projects.write")).Patch("/{id}/single-elevations/{sid}/materials/{kind}/{itemID}", func(w http.ResponseWriter, req *http.Request) {
			kind := chi.URLParam(req, "kind")
			itemID := chi.URLParam(req, "itemID")
			var in struct {
				MaterialID string `json:"material_id"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültige Eingabe", err)
				return
			}
			if err := projSvc.LinkVariantMaterial(req.Context(), kind, itemID, in.MaterialID); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})

	// Einstellungen – Nummernkreise
	protected.With(requirePermission("settings.manage")).Route("/settings/numbering", func(r chi.Router) {
		r.Get("/{entity}", func(w http.ResponseWriter, req *http.Request) {
			entity := chi.URLParam(req, "entity")
			cfg, err := numSvc.Get(req.Context(), entity)
			if err != nil {
				writeHTTPError(w, req, http.StatusNotFound, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusOK, cfg)
		})
		r.Get("/{entity}/preview", func(w http.ResponseWriter, req *http.Request) {
			entity := chi.URLParam(req, "entity")
			s, err := numSvc.Preview(req.Context(), entity)
			if err != nil {
				writeHTTPError(w, req, http.StatusNotFound, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"preview": s})
		})
		r.Put("/{entity}", func(w http.ResponseWriter, req *http.Request) {
			entity := chi.URLParam(req, "entity")
			var in struct {
				Pattern string `json:"pattern"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültige Eingabe", err)
				return
			}
			if strings.TrimSpace(in.Pattern) == "" {
				writeHTTPError(w, req, http.StatusBadRequest, "Pattern erforderlich", nil)
				return
			}
			if err := numSvc.UpdatePattern(req.Context(), entity, in.Pattern); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})

	// Einstellungen – PDF Templates
	protected.With(requirePermission("settings.manage")).Route("/settings/pdf", func(r chi.Router) {
		r.Get("/{entity}", func(w http.ResponseWriter, req *http.Request) {
			entity := chi.URLParam(req, "entity")
			t, err := pdfSvc.Get(req.Context(), entity)
			if err != nil {
				writeHTTPError(w, req, http.StatusNotFound, err.Error(), err)
				return
			}
			effectiveTemplate := *t
			effectiveDisplayName := ""
			effectiveClaim := ""
			effectivePrimaryColor := ""
			effectiveAccentColor := ""
			if branding, berr := brandingSvc.Get(req.Context()); berr == nil {
				effectiveTemplate = settings.ApplyBrandingDefaults(effectiveTemplate, branding)
				effectiveDisplayName = branding.DisplayName
				effectiveClaim = branding.Claim
				effectivePrimaryColor = branding.PrimaryColor
				effectiveAccentColor = branding.AccentColor
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"entity":                  t.Entity,
				"header_text":             t.HeaderText,
				"footer_text":             t.FooterText,
				"top_first_mm":            t.TopFirstMM,
				"top_other_mm":            t.TopOtherMM,
				"logo_doc_id":             t.LogoDocID,
				"bg_first_doc_id":         t.BgFirstDocID,
				"bg_other_doc_id":         t.BgOtherDocID,
				"effective_header_text":   effectiveTemplate.HeaderText,
				"effective_footer_text":   effectiveTemplate.FooterText,
				"effective_display_name":  effectiveDisplayName,
				"effective_claim":         effectiveClaim,
				"effective_primary_color": effectivePrimaryColor,
				"effective_accent_color":  effectiveAccentColor,
			})
		})
		r.Put("/{entity}", func(w http.ResponseWriter, req *http.Request) {
			entity := chi.URLParam(req, "entity")
			var in struct {
				HeaderText string  `json:"header_text"`
				FooterText string  `json:"footer_text"`
				TopFirstMM float64 `json:"top_first_mm"`
				TopOtherMM float64 `json:"top_other_mm"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültige Eingabe", err)
				return
			}
			if in.TopFirstMM <= 0 {
				in.TopFirstMM = 30
			}
			if in.TopOtherMM <= 0 {
				in.TopOtherMM = 20
			}
			if err := pdfSvc.Upsert(req.Context(), settings.PDFTemplate{Entity: entity, HeaderText: in.HeaderText, FooterText: in.FooterText, TopFirstMM: in.TopFirstMM, TopOtherMM: in.TopOtherMM}); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		// Uploads: logo, bg-first, bg-other
		r.Post("/{entity}/upload/{kind}", func(w http.ResponseWriter, req *http.Request) {
			entity := chi.URLParam(req, "entity")
			kind := chi.URLParam(req, "kind")
			if err := req.ParseMultipartForm(16 << 20); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültiges Formular", err)
				return
			}
			file, header, err := req.FormFile("file")
			if err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Datei fehlt (file)", err)
				return
			}
			defer file.Close()
			// Upload to GridFS
			db := mg.Database(cfg.MongoDB)
			bucket, err := gridfs.NewBucket(db)
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}
			oid, err := bucket.UploadFromStream(header.Filename, file)
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}
			hex := oid.Hex()
			if err := pdfSvc.SetImage(req.Context(), entity, mapPDFKind(kind), &hex); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"document_id": hex})
		})
		r.Delete("/{entity}/upload/{kind}", func(w http.ResponseWriter, req *http.Request) {
			entity := chi.URLParam(req, "entity")
			kind := chi.URLParam(req, "kind")
			if err := pdfSvc.SetImage(req.Context(), entity, mapPDFKind(kind), nil); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})

	// Einstellungen – Einheiten (für Materialeinheiten, Dimensionen etc.)
	protected.With(requirePermission("settings.manage")).Route("/settings/units", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			list, err := unitSvc.List(req.Context())
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in struct{ Code, Name string }
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültige Eingabe", err)
				return
			}
			if err := unitSvc.Upsert(req.Context(), in.Code, in.Name); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		r.Delete("/{code}", func(w http.ResponseWriter, req *http.Request) {
			code := chi.URLParam(req, "code")
			if err := unitSvc.Delete(req.Context(), code); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})

	protected.With(requirePermission("settings.manage")).Route("/settings/material-groups", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			list, err := materialGroupSvc.List(req.Context())
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in settings.MaterialGroup
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültige Eingabe", err)
				return
			}
			if err := materialGroupSvc.Upsert(req.Context(), in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		r.Delete("/{code}", func(w http.ResponseWriter, req *http.Request) {
			code := chi.URLParam(req, "code")
			if err := materialGroupSvc.Delete(req.Context(), code); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})

	protected.With(requirePermission("settings.manage")).Route("/settings/quote-text-blocks", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			list, err := quoteTextBlockSvc.List(req.Context())
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		})
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in settings.QuoteTextBlock
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültige Eingabe", err)
				return
			}
			if err := quoteTextBlockSvc.Upsert(req.Context(), in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		r.Delete("/{id}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			if err := quoteTextBlockSvc.Delete(req.Context(), id); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})

	// Einstellungen – Firmenprofil / Bankdaten
	protected.With(requirePermission("settings.manage")).Route("/settings/company", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			profile, err := companySvc.Get(req.Context())
			if err != nil {
				writeHTTPError(w, req, http.StatusNotFound, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusOK, profile)
		})
		r.Put("/", func(w http.ResponseWriter, req *http.Request) {
			var in settings.CompanyProfile
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültige Eingabe", err)
				return
			}
			if err := companySvc.Upsert(req.Context(), in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		r.Get("/branches", func(w http.ResponseWriter, req *http.Request) {
			items, err := companySvc.ListBranches(req.Context())
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusOK, items)
		})
		r.Post("/branches", func(w http.ResponseWriter, req *http.Request) {
			var in settings.CompanyBranch
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültige Eingabe", err)
				return
			}
			item, err := companySvc.CreateBranch(req.Context(), in)
			if err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusCreated, item)
		})
		r.Patch("/branches/{branchID}", func(w http.ResponseWriter, req *http.Request) {
			branchID := chi.URLParam(req, "branchID")
			var in settings.CompanyBranch
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültige Eingabe", err)
				return
			}
			item, err := companySvc.UpdateBranch(req.Context(), branchID, in)
			if err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "nicht gefunden") {
					writeHTTPError(w, req, http.StatusNotFound, err.Error(), err)
					return
				}
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusOK, item)
		})
		r.Delete("/branches/{branchID}", func(w http.ResponseWriter, req *http.Request) {
			branchID := chi.URLParam(req, "branchID")
			if err := companySvc.DeleteBranch(req.Context(), branchID); err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "nicht gefunden") {
					writeHTTPError(w, req, http.StatusNotFound, err.Error(), err)
					return
				}
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		r.Get("/localization", func(w http.ResponseWriter, req *http.Request) {
			cfg, err := locSvc.Get(req.Context())
			if err != nil {
				writeHTTPError(w, req, http.StatusNotFound, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusOK, cfg)
		})
		r.Put("/localization", func(w http.ResponseWriter, req *http.Request) {
			var in settings.LocalizationSettings
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültige Eingabe", err)
				return
			}
			if err := locSvc.Upsert(req.Context(), in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		r.Get("/branding", func(w http.ResponseWriter, req *http.Request) {
			cfg, err := brandingSvc.Get(req.Context())
			if err != nil {
				writeHTTPError(w, req, http.StatusNotFound, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusOK, cfg)
		})
		r.Put("/branding", func(w http.ResponseWriter, req *http.Request) {
			var in settings.BrandingSettings
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültige Eingabe", err)
				return
			}
			if err := brandingSvc.Upsert(req.Context(), in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})

	// Download eines Dokuments über DocumentID (GridFS ObjectID Hex)
	protected.With(requirePermission("documents.read")).Get("/documents/{docID}", func(w http.ResponseWriter, req *http.Request) {
		docID := chi.URLParam(req, "docID")
		rc, filename, contentType, length, err := matSvc.OpenDocumentStream(req.Context(), docID)
		if err != nil {
			rc, filename, contentType, length, err = conSvc.OpenDocumentStream(req.Context(), docID)
			if err != nil {
				writeHTTPError(w, req, http.StatusNotFound, err.Error(), err)
				return
			}
		}
		defer rc.Close()
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)
		if filename != "" {
			w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
		}
		if length > 0 {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", length))
		}
		if _, err := io.Copy(w, rc); err != nil {
			// can't write header after body; just stop
			return
		}
	})

	protected.Route("/workflow", func(r chi.Router) {
		r.With(requirePermission("quotes.read"), requirePermission("sales_orders.read")).Get("/commercial", func(w http.ResponseWriter, req *http.Request) {
			out, err := buildCommercialWorkflow(req.Context(), pg, quoteSvc, salesSvc, commercialWorkflowFilter{
				ProjectID: strings.TrimSpace(req.URL.Query().Get("project_id")),
				ContactID: strings.TrimSpace(req.URL.Query().Get("contact_id")),
				Kind:      strings.TrimSpace(req.URL.Query().Get("kind")),
			})
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})

	protected.Route("/stock-movements", func(r chi.Router) {
		r.With(requirePermission("stock_movements.write")).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in materials.StockMovementCreate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültige Eingabe", err)
				return
			}
			out, err := matSvc.CreateMovement(req.Context(), in)
			if err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
	})

	protected.Route("/warehouses", func(r chi.Router) {
		r.With(requirePermission("warehouses.write")).Post("/", func(w http.ResponseWriter, req *http.Request) {
			var in materials.WarehouseCreate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültige Eingabe", err)
				return
			}
			out, err := matSvc.CreateWarehouse(req.Context(), in)
			if err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("warehouses.read")).Get("/", func(w http.ResponseWriter, req *http.Request) {
			out, err := matSvc.ListWarehouses(req.Context())
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		r.With(requirePermission("warehouses.write")).Post("/{id}/locations", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			var in materials.LocationCreate
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, "Ungültige Eingabe", err)
				return
			}
			out, err := matSvc.CreateLocation(req.Context(), id, in)
			if err != nil {
				writeHTTPError(w, req, http.StatusBadRequest, err.Error(), err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		})
		r.With(requirePermission("warehouses.read")).Get("/{id}/locations", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "id")
			out, err := matSvc.ListLocations(req.Context(), id)
			if err != nil {
				writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
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

func writeHTTPError(w http.ResponseWriter, req *http.Request, code int, msg string, err error) {
	logAPIError(req, code, "http_error", msg, err)
	http.Error(w, msg, code)
}

func writeDomainError(w http.ResponseWriter, req *http.Request, err error) {
	if err == nil {
		writeAPIError(w, req, http.StatusInternalServerError, "internal_error", "Interner Fehler")
		return
	}
	status, code := classifyDomainError(err)
	logAPIError(req, status, code, err.Error(), err)
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": err.Error(),
		},
	})
}

func classifyDomainError(err error) (int, string) {
	if err == nil {
		return http.StatusInternalServerError, "internal_error"
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "nicht gefunden"):
		return http.StatusNotFound, "not_found"
	case strings.Contains(msg, "nicht konfiguriert"),
		strings.Contains(msg, "gridfs nicht verfügbar"),
		strings.Contains(msg, "postgres nicht konfiguriert"),
		strings.Contains(msg, "mongodb nicht konfiguriert"):
		return http.StatusInternalServerError, "internal_error"
	case strings.Contains(msg, "erforderlich"),
		strings.Contains(msg, "ungültig"),
		strings.Contains(msg, "bereits vorhanden"),
		strings.Contains(msg, "darf nicht"),
		strings.Contains(msg, "fehlt"),
		strings.Contains(msg, "nicht im status"),
		strings.Contains(msg, "nicht gebucht"),
		strings.Contains(msg, "übersteigt"),
		strings.Contains(msg, "stimmt nicht"):
		return http.StatusBadRequest, "validation_error"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

func writeAPIError(w http.ResponseWriter, req *http.Request, code int, errCode, msg string) {
	logAPIError(req, code, errCode, msg, nil)
	writeJSON(w, code, map[string]any{
		"error": map[string]any{
			"code":    errCode,
			"message": msg,
		},
	})
}

type authContextKey string

const (
	authUserKey        authContextKey = "auth_user"
	authRolesKey       authContextKey = "auth_roles"
	authPermissionsKey authContextKey = "auth_permissions"
	authSessionKey     authContextKey = "auth_session"
)

func requireAuth(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			header := strings.TrimSpace(req.Header.Get("Authorization"))
			if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
				writeAPIError(w, req, http.StatusUnauthorized, "unauthorized", "Authorization fehlt")
				return
			}
			token := strings.TrimSpace(header[len("Bearer "):])
			sess, user, permissions, err := authSvc.AuthenticateAccessToken(req.Context(), token)
			if err != nil {
				writeAPIError(w, req, http.StatusUnauthorized, "unauthorized", "Token ungültig oder abgelaufen")
				return
			}
			ctx := context.WithValue(req.Context(), authUserKey, *user)
			ctx = context.WithValue(ctx, authRolesKey, sess.Roles)
			ctx = context.WithValue(ctx, authPermissionsKey, permissions)
			ctx = context.WithValue(ctx, authSessionKey, *sess)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}

func authUserFromContext(ctx context.Context) (auth.User, bool) {
	v, ok := ctx.Value(authUserKey).(auth.User)
	return v, ok
}

func authRolesFromContext(ctx context.Context) ([]string, bool) {
	v, ok := ctx.Value(authRolesKey).([]string)
	return v, ok
}

func authPermissionsFromContext(ctx context.Context) ([]string, bool) {
	v, ok := ctx.Value(authPermissionsKey).([]string)
	return v, ok
}

func requirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			permissions, ok := authPermissionsFromContext(req.Context())
			if !ok {
				writeAPIError(w, req, http.StatusForbidden, "forbidden", "Berechtigungen fehlen")
				return
			}
			for _, p := range permissions {
				if p == permission || p == "users.manage" {
					next.ServeHTTP(w, req)
					return
				}
			}
			writeAPIError(w, req, http.StatusForbidden, "forbidden", "Keine Berechtigung")
		})
	}
}

// helper mapping for upload kinds for PDF settings
func mapPDFKind(k string) string {
	switch strings.ToLower(k) {
	case "logo":
		return "logo"
	case "bg-first", "bg_first":
		return "bg_first"
	case "bg-other", "bg_other":
		return "bg_other"
	default:
		return k
	}
}

// sanitizeFilename entfernt problematische Zeichen für Dateinamen in Content-Disposition
func sanitizeFilename(s string) string {
	if s == "" {
		return time.Now().Format("20060102")
	}
	repl := []string{"/", "-", "\\", "-", ":", "-", "\"", "'", "\n", " ", "\r", " "}
	r := strings.NewReplacer(repl...)
	out := r.Replace(s)
	if strings.TrimSpace(out) == "" {
		return time.Now().Format("20060102")
	}
	return out
}

func projectQuoteDimensionsLabel(widthMM, heightMM *float64) string {
	if widthMM == nil && heightMM == nil {
		return ""
	}
	if widthMM != nil && heightMM != nil {
		return fmt.Sprintf("%s x %s mm", strconv.FormatFloat(*widthMM, 'f', -1, 64), strconv.FormatFloat(*heightMM, 'f', -1, 64))
	}
	if widthMM != nil {
		return fmt.Sprintf("B %s mm", strconv.FormatFloat(*widthMM, 'f', -1, 64))
	}
	return fmt.Sprintf("H %s mm", strconv.FormatFloat(*heightMM, 'f', -1, 64))
}
