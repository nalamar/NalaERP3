package llm

import (
	"context"
	"sync"
)

// Fake ist ein Test-Anbieter ohne Netz: Respond bestimmt je Aufruf die
// Antwort; Requests haelt alle Anfragen fest. Fuer Tests in diesem und in
// abhaengigen Paketen (I.4).
type Fake struct {
	Respond func(ctx context.Context, call int, req Request) (Response, error)

	mu       sync.Mutex
	Requests []Request
}

func (*Fake) Name() string { return "fake" }

func (f *Fake) Complete(ctx context.Context, req Request) (Response, error) {
	f.mu.Lock()
	f.Requests = append(f.Requests, req)
	call := len(f.Requests)
	f.mu.Unlock()
	if f.Respond == nil {
		return Response{Text: "{}", Model: "fake"}, nil
	}
	return f.Respond(ctx, call, req)
}
