// Package llm kapselt den Zugriff auf Sprachmodelle hinter einer schmalen,
// anbieterneutralen Schnittstelle (ADR 0043, Backlog I.3). Fachliche Regeln
// (nur Zuordnung und Textarbeit, nie ein Preis vom Modell, Score und
// Begruendung, Freigabe durch einen Menschen; aufgabe.md §4) stehen nicht
// hier, sondern bei den Aufrufern. Dieses Paket sorgt fuer Zeitlimit,
// Wiederholung bei voruebergehenden Fehlern, Pruefung der JSON-Antwort und
// ein Protokoll ohne Inhalte.
package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"
)

// Message ist eine Nachricht im Gespraech; Role ist "user" oder "assistant".
type Message struct {
	Role    string
	Content string
}

// Request ist eine Anfrage an ein Modell. JSONSchema leer = freier Text;
// sonst muss die Antwort ein JSON-Wert sein, der dem Schema entspricht
// (unterstuetzte Teilmenge siehe ValidateJSON). Purpose erscheint im
// Protokoll (z. B. "matching").
type Request struct {
	System      string
	Messages    []Message
	JSONSchema  map[string]any
	MaxTokens   int
	Temperature float64
	Purpose     string
}

// Usage ist der Verbrauch eines Aufrufs.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Response ist die Antwort eines Modells.
type Response struct {
	Text  string
	Model string
	Usage Usage
}

// Provider ist eine Anbindung an einen Dienst (OpenAI-kompatibel, Anthropic,
// Mistral, Test-Anbieter). Voruebergehende Fehler (Ueberlast, 5xx,
// Netzwerk) meldet sie als *TransientError.
type Provider interface {
	Name() string
	Complete(ctx context.Context, req Request) (Response, error)
}

// TransientError markiert einen Fehler, bei dem eine Wiederholung sinnvoll
// ist.
type TransientError struct{ Err error }

func (e *TransientError) Error() string { return "vorübergehender Fehler: " + e.Err.Error() }
func (e *TransientError) Unwrap() error { return e.Err }

var (
	// ErrTimeout: das Zeitlimit eines Aufrufs ist abgelaufen.
	ErrTimeout = errors.New("Zeitlimit des Sprachmodells überschritten")
	// ErrInvalidResponse: die Antwort entspricht nicht dem verlangten Schema.
	ErrInvalidResponse = errors.New("Antwort des Sprachmodells ist ungültig")
)

// Grenzen (ADR 0043 Frage 6): Standard 300 s je Aufruf (je Mandant
// einstellbar, I.3.5) und hoechstens 2 Wiederholungen.
const (
	DefaultTimeout    = 300 * time.Second
	DefaultMaxRetries = 2
	MaxTimeout        = 30 * time.Minute
)

// Limits sind die Grenzen eines Clients; Nullwerte bedeuten Standard.
type Limits struct {
	Timeout    time.Duration
	MaxRetries int
}

func (l Limits) normalized() Limits {
	if l.Timeout <= 0 {
		l.Timeout = DefaultTimeout
	}
	if l.Timeout > MaxTimeout {
		l.Timeout = MaxTimeout
	}
	if l.MaxRetries < 0 {
		l.MaxRetries = 0
	}
	if l.MaxRetries > DefaultMaxRetries {
		l.MaxRetries = DefaultMaxRetries
	}
	return l
}

// Ergebnis eines Aufrufs im Protokoll.
const (
	OutcomeOK              = "ok"
	OutcomeError           = "error"
	OutcomeTimeout         = "timeout"
	OutcomeInvalidResponse = "invalid_response"
)

// CallRecord ist der Protokolleintrag eines Aufrufs. Er enthaelt bewusst
// keine Inhalte (ADR 0043 Frage 5), nur eine Pruefsumme der Eingabe.
type CallRecord struct {
	Provider  string
	Model     string
	Purpose   string
	Outcome   string
	Attempts  int
	Duration  time.Duration
	Usage     Usage
	InputHash string
}

// Recorder nimmt Protokolleintraege entgegen.
type Recorder interface {
	Record(ctx context.Context, rec CallRecord)
}

// LogRecorder schreibt Eintraege ins Server-Log.
type LogRecorder struct{}

func (LogRecorder) Record(_ context.Context, r CallRecord) {
	log.Printf("llm_call provider=%s model=%s purpose=%s outcome=%s attempts=%d duration_ms=%d input_tokens=%d output_tokens=%d input_hash=%s",
		r.Provider, r.Model, r.Purpose, r.Outcome, r.Attempts, r.Duration.Milliseconds(), r.Usage.InputTokens, r.Usage.OutputTokens, r.InputHash)
}

// InputHash ist eine Pruefsumme ueber Systemtext und Nachrichten (16
// Hex-Zeichen); sie erlaubt, gleiche Eingaben im Protokoll zu erkennen, ohne
// den Inhalt zu speichern.
func InputHash(req Request) string {
	h := sha256.New()
	write := func(s string) {
		fmt.Fprintf(h, "%d:%s|", len(s), s)
	}
	write(req.System)
	for _, m := range req.Messages {
		write(m.Role)
		write(m.Content)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Client legt Zeitlimit, Wiederholung, Schemapruefung und Protokoll um
// einen Provider.
type Client struct {
	provider Provider
	limits   Limits
	recorder Recorder
	sleep    func(time.Duration)
	now      func() time.Time
}

// NewClient erzeugt einen Client; recorder nil = LogRecorder.
func NewClient(p Provider, limits Limits, recorder Recorder) *Client {
	if recorder == nil {
		recorder = LogRecorder{}
	}
	return &Client{provider: p, limits: limits.normalized(), recorder: recorder, sleep: time.Sleep, now: time.Now}
}

// Limits liefert die wirksamen Grenzen.
func (c *Client) Limits() Limits { return c.limits }

// backoff zwischen Wiederholungen: 1 s, 2 s.
func backoff(attempt int) time.Duration { return time.Duration(attempt) * time.Second }

// Complete fuehrt die Anfrage aus. Voruebergehende Fehler werden bis zu
// MaxRetries-mal wiederholt; ein abgelaufenes Zeitlimit und eine ungueltige
// Antwort nicht.
func (c *Client) Complete(ctx context.Context, req Request) (Response, error) {
	rec := CallRecord{Provider: c.provider.Name(), Purpose: req.Purpose, InputHash: InputHash(req)}
	start := c.now()
	defer func() {
		rec.Duration = c.now().Sub(start)
		c.recorder.Record(ctx, rec)
	}()
	for {
		rec.Attempts++
		attemptCtx, cancel := context.WithTimeout(ctx, c.limits.Timeout)
		resp, err := c.provider.Complete(attemptCtx, req)
		timedOut := errors.Is(attemptCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil
		cancel()
		if err == nil {
			rec.Model = resp.Model
			rec.Usage.InputTokens += resp.Usage.InputTokens
			rec.Usage.OutputTokens += resp.Usage.OutputTokens
			if req.JSONSchema != nil {
				if verr := checkJSON(resp.Text, req.JSONSchema); verr != nil {
					rec.Outcome = OutcomeInvalidResponse
					return Response{}, fmt.Errorf("%w: %v", ErrInvalidResponse, verr)
				}
			}
			rec.Outcome = OutcomeOK
			return resp, nil
		}
		if timedOut {
			rec.Outcome = OutcomeTimeout
			return Response{}, fmt.Errorf("%w (%s)", ErrTimeout, c.limits.Timeout)
		}
		var transient *TransientError
		if !errors.As(err, &transient) || rec.Attempts > c.limits.MaxRetries || ctx.Err() != nil {
			rec.Outcome = OutcomeError
			return Response{}, err
		}
		c.sleep(backoff(rec.Attempts))
	}
}

func checkJSON(text string, schema map[string]any) error {
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return fmt.Errorf("kein JSON: %v", err)
	}
	return ValidateJSON(v, schema)
}
