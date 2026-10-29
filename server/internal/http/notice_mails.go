package apihttp

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"nalaerp3/internal/noticemail"
)

// Hinweis-Mails (Backlog F.14.4c):
//   - GET/PUT /notice-mails/me: eigene Abbestellung - jeder angemeldete
//     Benutzer fuer sich, ohne eigenes Recht.
//   - GET /notice-mails/log: Versandprotokoll des Mandanten (ohne Inhalt,
//     ohne Adressen) mit settings.manage.

type noticeMailPref struct {
	Abbestellt bool `json:"abbestellt"`
}

func mountNoticeMailRoutes(protected chi.Router, prefs *noticemail.Prefs) {
	protected.Route("/notice-mails", func(r chi.Router) {
		r.Get("/me", func(w http.ResponseWriter, req *http.Request) {
			out, err := prefs.OptedOut(req.Context(), actorUserIDFromContext(req.Context()))
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, noticeMailPref{Abbestellt: out})
		})

		r.Put("/me", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				Abbestellt *bool `json:"abbestellt"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil || in.Abbestellt == nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "abbestellt (true/false) erforderlich")
				return
			}
			userID := actorUserIDFromContext(req.Context())
			if err := prefs.SetOptedOut(req.Context(), userID, *in.Abbestellt); err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, noticeMailPref{Abbestellt: *in.Abbestellt})
		})

		// Juengste zuerst; limit (Standard 100, hoechstens 500).
		r.With(requirePermission("settings.manage")).Get("/log", func(w http.ResponseWriter, req *http.Request) {
			limit := 0
			if v := req.URL.Query().Get("limit"); v != "" {
				n, err := strconv.Atoi(v)
				if err != nil || n < 1 {
					writeAPIError(w, req, http.StatusBadRequest, "validation_error", "limit muss eine positive Zahl sein")
					return
				}
				limit = n
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := prefs.ListLog(req.Context(), companyID, limit)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	})
}
