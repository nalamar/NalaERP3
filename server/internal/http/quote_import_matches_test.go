package apihttp

import (
	"context"
	"errors"
	"testing"
	"time"
)

// backgroundMatchRunner (Backlog I.5.3): kehrt sofort zurueck, der Lauf
// bekommt einen Context ohne Abbruch, Fehler und Panic beenden nur ihn.
func TestBackgroundMatchRunner(t *testing.T) {
	release := make(chan struct{})
	done := make(chan error, 3)
	backgroundMatchRunner(func(ctx context.Context) error {
		<-release
		done <- ctx.Err()
		return errors.New("Fehler des Laufs")
	})
	// kehrt zurueck, bevor der Lauf endet
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Context des Laufs: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Lauf nicht ausgeführt")
	}
	backgroundMatchRunner(func(context.Context) error {
		defer func() { done <- nil }()
		panic("Panic im Lauf")
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Lauf mit Panic nicht ausgeführt")
	}
	// der Testprozess laeuft weiter: die Panic wurde aufgefangen
	time.Sleep(50 * time.Millisecond)
}
