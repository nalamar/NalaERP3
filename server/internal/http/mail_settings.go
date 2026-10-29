package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"nalaerp3/internal/mail"
)

// HTTP-Anbindung des Mail-Zugangs (ADR 0033, Backlog F.14.2c), alles mit
// settings.manage. Das Passwort wird nur angenommen, nie ausgegeben.

// MailTransport verschickt eine Nachricht (mail.Sender; im Test ersetzbar).
type MailTransport interface {
	Send(ctx context.Context, cfg mail.SMTPConfig, msg mail.Message) error
}

const (
	mailTestSubject = "Nala ERP: Testnachricht"
	mailTestText    = "Diese Testnachricht bestätigt, dass der E-Mail-Versand von Nala ERP eingerichtet ist.\n\nBitte nicht antworten."
)

func mountMailSettingsRoutes(protected chi.Router, svc *mail.SettingsService, transport MailTransport, redirected bool) {
	protected.With(requirePermission("settings.manage")).Route("/settings/mail", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Get(req.Context(), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Speichern. Leeres "passwort" laesst das gespeicherte stehen,
		// "passwort_loeschen": true entfernt es.
		r.Put("/", func(w http.ResponseWriter, req *http.Request) {
			var in mail.Input
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Save(req.Context(), in, actorUserIDFromContext(req.Context()), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Testversand an eine Adresse - mit dem gespeicherten Zugang, auch
		// wenn der Versand noch nicht eingeschaltet ist. Scheitert der
		// Mail-Server, kommt 502 mit der zusammengefassten Meldung.
		r.Post("/test", func(w http.ResponseWriter, req *http.Request) {
			var in struct {
				An string `json:"an"`
			}
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
				return
			}
			to := strings.TrimSpace(in.An)
			if to == "" {
				writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Empfänger erforderlich")
				return
			}
			companyID, _ := companyIDFromContext(req.Context())
			cfg, err := svc.Config(req.Context(), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			msg := mail.Message{To: []string{to}, Subject: mailTestSubject, Text: mailTestText}
			if err := mail.ValidateMessage(*cfg, msg); err != nil {
				writeDomainError(w, req, err)
				return
			}
			err = transport.Send(req.Context(), *cfg, msg)
			var se *mail.SendError
			if errors.As(err, &se) {
				writeAPIError(w, req, http.StatusBadGateway, "mail_send_failed", se.Error())
				return
			}
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"gesendet": true, "umgeleitet": redirected})
		})
	})
}
