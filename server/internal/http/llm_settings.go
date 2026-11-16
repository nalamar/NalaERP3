package apihttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"nalaerp3/internal/llm"
)

// HTTP-Anbindung der Einstellungen fuer das Sprachmodell (ADR 0043, Backlog
// I.3.5a2), alles mit settings.manage. Der API-Schluessel wird nur
// angenommen, nie ausgegeben.

// llmTestRequest ist der Verbindungstest: kurze Anfrage mit kleinem
// JSON-Schema, ohne Daten aus dem ERP. 1024 Ausgabe-Tokens, weil Modelle mit
// Denken einen Teil davon fuer das Denken verbrauchen.
func llmTestRequest() llm.Request {
	return llm.Request{
		System:   "Du bist ein Verbindungstest. Antworte nur mit dem verlangten JSON.",
		Messages: []llm.Message{{Role: "user", Content: `Antworte mit {"ok": true}.`}},
		JSONSchema: map[string]any{
			"type":       "object",
			"required":   []any{"ok"},
			"properties": map[string]any{"ok": map[string]any{"type": "boolean"}},
		},
		MaxTokens: 1024,
		Purpose:   "verbindungstest",
	}
}

func mountLLMSettingsRoutes(protected chi.Router, svc *llm.SettingsService) {
	protected.With(requirePermission("settings.manage")).Route("/settings/llm", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			out, err := svc.Get(req.Context(), companyID)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})

		// Speichern. Leerer "api_key" laesst den gespeicherten stehen,
		// "api_key_loeschen": true entfernt ihn.
		r.Put("/", func(w http.ResponseWriter, req *http.Request) {
			var in llm.SettingsInput
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

		// Verbindungstest mit der gespeicherten Einstellung - auch wenn das
		// Sprachmodell noch nicht eingeschaltet ist. Scheitert der Dienst,
		// kommt 502 mit der zusammengefassten Meldung; ist das Tageslimit
		// erreicht, 429.
		r.Post("/test", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			client, err := svc.TestClient(req.Context(), companyID, nil)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			start := time.Now()
			resp, err := client.Complete(req.Context(), llmTestRequest())
			switch {
			case errors.Is(err, llm.ErrDailyLimit):
				writeAPIError(w, req, http.StatusTooManyRequests, "llm_daily_limit", err.Error())
				return
			case err != nil:
				writeAPIError(w, req, http.StatusBadGateway, "llm_test_failed", "Test des Sprachmodells fehlgeschlagen: "+err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":             true,
				"modell":         resp.Model,
				"dauer_ms":       time.Since(start).Milliseconds(),
				"eingabe_tokens": resp.Usage.InputTokens,
				"ausgabe_tokens": resp.Usage.OutputTokens,
			})
		})
	})
}
