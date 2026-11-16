package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAICompatible spricht die Chat-Completions-Schnittstelle, die OpenAI
// und viele andere Dienste anbieten, auch lokale Server wie Ollama, vLLM
// oder LM Studio (ADR 0043, Backlog I.3.2).
//
// BaseURL ist die Basis bis einschliesslich Versionspfad, z. B.
// "https://api.openai.com/v1" oder "http://localhost:11434/v1". APIKey ist
// optional (lokale Server brauchen keinen). SchemaMode steuert, wie ein
// verlangtes JSON-Schema uebergeben wird: "json_schema" (Standard) oder
// "json_object" fuer Dienste, die nur JSON ohne Schema kennen; geprueft wird
// die Antwort in beiden Faellen vom Client.
type OpenAICompatible struct {
	BaseURL    string
	APIKey     string
	Model      string
	SchemaMode string
	HTTPClient *http.Client
}

// Hoechstgroesse einer Antwort des Dienstes.
const maxProviderResponseBytes = 10 << 20

func (*OpenAICompatible) Name() string { return "openai_compatible" }

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model          string          `json:"model"`
	Messages       []openAIMessage `json:"messages"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat map[string]any  `json:"response_format,omitempty"`
}

type openAIResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content *string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (p *OpenAICompatible) Complete(ctx context.Context, req Request) (Response, error) {
	base := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if base == "" || strings.TrimSpace(p.Model) == "" {
		return Response{}, errors.New("Sprachmodell nicht vollständig eingerichtet (Endpunkt und Modell)")
	}
	body := openAIRequest{Model: p.Model, MaxTokens: req.MaxTokens, Temperature: req.Temperature}
	if req.System != "" {
		body.Messages = append(body.Messages, openAIMessage{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		body.Messages = append(body.Messages, openAIMessage{Role: m.Role, Content: m.Content})
	}
	if req.JSONSchema != nil {
		if p.SchemaMode == "json_object" {
			body.ResponseFormat = map[string]any{"type": "json_object"}
		} else {
			body.ResponseFormat = map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "antwort", "schema": req.JSONSchema}}
		}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	var parsed openAIResponse
	if err := postJSON(ctx, p.HTTPClient, base+"/chat/completions", raw, func(h http.Header) {
		if p.APIKey != "" {
			h.Set("Authorization", "Bearer "+p.APIKey)
		}
	}, []string{p.APIKey}, &parsed); err != nil {
		return Response{}, err
	}
	if len(parsed.Choices) == 0 || parsed.Choices[0].Message.Content == nil {
		return Response{}, errors.New("Antwort des Sprachmodells ohne Inhalt")
	}
	return Response{
		Text:  *parsed.Choices[0].Message.Content,
		Model: parsed.Model,
		Usage: Usage{InputTokens: parsed.Usage.PromptTokens, OutputTokens: parsed.Usage.CompletionTokens},
	}, nil
}

// postJSON sendet eine Anfrage an einen Dienst und dekodiert die Antwort.
// Netzfehler, 429 und 5xx gelten als voruebergehend; andere Fehlerstatus
// als dauerhaft. Fehlermeldungen enthalten hoechstens 300 Zeichen der
// Antwort, Geheimnisse aus secrets sind darin ersetzt.
func postJSON(ctx context.Context, client *http.Client, url string, body []byte, auth func(http.Header), secrets []string, out any) error {
	if client == nil {
		client = http.DefaultClient
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	auth(httpReq.Header)
	resp, err := client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return &TransientError{Err: redact(err.Error(), secrets)}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderResponseBytes))
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return &TransientError{Err: redact(err.Error(), secrets)}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := strings.TrimSpace(string(data))
		if r := []rune(msg); len(r) > 300 {
			msg = string(r[:300]) + "…"
		}
		err := fmt.Errorf("Sprachmodell antwortet mit Status %d: %s", resp.StatusCode, redact(msg, secrets).Error())
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return &TransientError{Err: err}
		}
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("Antwort des Sprachmodells nicht lesbar: %v", err)
	}
	return nil
}

func redact(msg string, secrets []string) error {
	for _, s := range secrets {
		if s != "" {
			msg = strings.ReplaceAll(msg, s, "***")
		}
	}
	return errors.New(msg)
}
