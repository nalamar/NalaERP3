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
		// Sprachmodell noch nicht eingeschaltet ist (siehe runLLMTest).
		r.Post("/test", func(w http.ResponseWriter, req *http.Request) {
			companyID, _ := companyIDFromContext(req.Context())
			client, err := svc.TestClient(req.Context(), companyID, nil)
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			runLLMTest(w, req, client)
		})

		mountLLMProfileRoutes(r, svc)
	})
}

// runLLMTest schickt die Testanfrage und schreibt das Ergebnis: 200 mit
// Modell, Dauer und Tokens (ohne Inhalt der Antwort), 429 beim Tageslimit,
// 502 mit zusammengefasster Meldung, wenn der Dienst scheitert.
func runLLMTest(w http.ResponseWriter, req *http.Request, client *llm.Client) {
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
}

// mountLLMProfileRoutes: Modellprofile sowie Hauptschalter und Tageslimit
// (ADR 0046, Backlog I.8.1b) unter /settings/llm, Recht wie oben.
func mountLLMProfileRoutes(r chi.Router, svc *llm.SettingsService) {
	r.Get("/mandant", func(w http.ResponseWriter, req *http.Request) {
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.GetMandant(req.Context(), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Put("/mandant", func(w http.ResponseWriter, req *http.Request) {
		var in llm.MandantInput
		if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
			writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.SaveMandant(req.Context(), in, actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})

	r.Get("/profile", func(w http.ResponseWriter, req *http.Request) {
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.ListProfiles(req.Context(), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	// Anlegen: das erste Profil des Mandanten wird Standard.
	r.Post("/profile", func(w http.ResponseWriter, req *http.Request) {
		var in llm.ProfileInput
		if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
			writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.CreateProfile(req.Context(), in, actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	})
	r.Get("/profile/{id}", func(w http.ResponseWriter, req *http.Request) {
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.GetProfile(req.Context(), companyID, chi.URLParam(req, "id"))
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	// Ersetzen. Leerer "api_key" laesst den gespeicherten stehen,
	// "api_key_loeschen": true entfernt ihn.
	r.Put("/profile/{id}", func(w http.ResponseWriter, req *http.Request) {
		var in llm.ProfileInput
		if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
			writeAPIError(w, req, http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.UpdateProfile(req.Context(), chi.URLParam(req, "id"), in, actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Post("/profile/{id}/standard", func(w http.ResponseWriter, req *http.Request) {
		companyID, _ := companyIDFromContext(req.Context())
		out, err := svc.SetStandardProfile(req.Context(), chi.URLParam(req, "id"), actorUserIDFromContext(req.Context()), companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	// Verbindungstest je Profil, auch ausgeschaltet; zaehlt zum Tageslimit.
	r.Post("/profile/{id}/test", func(w http.ResponseWriter, req *http.Request) {
		companyID, _ := companyIDFromContext(req.Context())
		client, _, err := svc.TestProfileClient(req.Context(), companyID, chi.URLParam(req, "id"), nil)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		runLLMTest(w, req, client)
	})
}
