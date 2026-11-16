package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Anthropic spricht die Messages-API ueber das offizielle Go-SDK (ADR 0043,
// Backlog I.3.3). Wiederholungen des SDK sind abgeschaltet; Zeitlimit und
// Wiederholung uebernimmt der Client dieses Pakets.
//
// BaseURL ist optional (Standard des SDK). Ohne Modell gilt
// DefaultAnthropicModel. Request.Temperature wird nicht gesendet: die API
// lehnt Sampling-Parameter seit 2026-10-21 ab. Ein verlangtes JSON-Schema
// geht als output_config.format; Schluesselwoerter, die die API dort nicht
// annimmt (z. B. minimum/maximum), werden entfernt - der Client prueft die
// Antwort gegen das vollstaendige Schema.
type Anthropic struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient option.HTTPClient
}

const (
	DefaultAnthropicModel     = "claude-opus-5-5"
	defaultAnthropicMaxTokens = 16000
)

func (*Anthropic) Name() string { return "anthropic" }

func (p *Anthropic) Complete(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return Response{}, errors.New("Sprachmodell nicht vollständig eingerichtet (API-Schlüssel)")
	}
	model := strings.TrimSpace(p.Model)
	if model == "" {
		model = DefaultAnthropicModel
	}
	maxTokens := int64(req.MaxTokens)
	if maxTokens <= 0 {
		maxTokens = defaultAnthropicMaxTokens
	}
	opts := []option.RequestOption{option.WithAPIKey(p.APIKey), option.WithMaxRetries(0)}
	if base := strings.TrimSpace(p.BaseURL); base != "" {
		opts = append(opts, option.WithBaseURL(base))
	}
	if p.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(p.HTTPClient))
	}
	client := anthropic.NewClient(opts...)

	params := anthropic.MessageNewParams{Model: model, MaxTokens: maxTokens}
	if req.System != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.System}}
	}
	for _, m := range req.Messages {
		if m.Role == "assistant" {
			params.Messages = append(params.Messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		} else {
			params.Messages = append(params.Messages, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		}
	}
	if req.JSONSchema != nil {
		params.OutputConfig = anthropic.OutputConfigParam{Format: anthropic.JSONOutputFormatParam{Schema: apiSchema(req.JSONSchema)}}
	}

	msg, err := client.Messages.New(ctx, params)
	if err != nil {
		if ctx.Err() != nil {
			return Response{}, ctx.Err()
		}
		var apierr *anthropic.Error
		if errors.As(err, &apierr) {
			e := fmt.Errorf("Sprachmodell antwortet mit Status %d (%s)", apierr.StatusCode, apierr.Type())
			// 429 Ratenlimit, 5xx einschliesslich 529 Ueberlastung
			if apierr.StatusCode == 429 || apierr.StatusCode >= 500 {
				return Response{}, &TransientError{Err: e}
			}
			return Response{}, e
		}
		// Transportfehler (Meldung nennt nur die URL, nie den Schluessel)
		return Response{}, &TransientError{Err: err}
	}
	if msg.StopReason == anthropic.StopReasonRefusal {
		return Response{}, errors.New("Anfrage vom Sprachmodell abgelehnt")
	}
	var text strings.Builder
	for _, block := range msg.Content {
		if b, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(b.Text)
		}
	}
	if text.Len() == 0 {
		return Response{}, errors.New("Antwort des Sprachmodells ohne Inhalt")
	}
	return Response{
		Text:  text.String(),
		Model: string(msg.Model),
		Usage: Usage{InputTokens: int(msg.Usage.InputTokens), OutputTokens: int(msg.Usage.OutputTokens)},
	}, nil
}

// apiSchema kopiert ein Schema ohne die Schluesselwoerter, die structured
// outputs nicht annimmt, und setzt additionalProperties false bei Objekten
// (von der API verlangt).
func apiSchema(schema map[string]any) map[string]any {
	out := make(map[string]any, len(schema))
	for k, v := range schema {
		switch k {
		case "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf", "minLength", "maxLength", "minItems", "maxItems":
			continue
		case "properties", "$defs":
			// Feldnamen bleiben, die Teilschemata werden bereinigt
			if props, ok := v.(map[string]any); ok {
				clean := make(map[string]any, len(props))
				for name, sub := range props {
					clean[name] = subSchema(sub)
				}
				out[k] = clean
				continue
			}
		case "items":
			out[k] = subSchema(v)
			continue
		case "anyOf", "allOf":
			if list, ok := v.([]any); ok {
				clean := make([]any, len(list))
				for i, sub := range list {
					clean[i] = subSchema(sub)
				}
				out[k] = clean
				continue
			}
		}
		out[k] = v
	}
	if out["type"] == "object" {
		out["additionalProperties"] = false
	}
	return out
}

func subSchema(v any) any {
	if m, ok := v.(map[string]any); ok {
		return apiSchema(m)
	}
	return v
}
