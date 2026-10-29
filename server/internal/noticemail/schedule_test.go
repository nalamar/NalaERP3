package noticemail

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// Zeitsteuerung ohne Datenbank (Backlog F.14.4b).

type fakeDayRunner struct {
	mu   sync.Mutex
	days []time.Time
	err  error
}

func (f *fakeDayRunner) RunDay(_ context.Context, day time.Time) (Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.days = append(f.days, day)
	return Result{Gesendet: 1}, f.err
}

func (f *fakeDayRunner) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.days)
}

func TestDueDay(t *testing.T) {
	loc := Berlin()
	at := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, loc) }
	utcDay := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
	for name, tc := range map[string]struct {
		now  time.Time
		want time.Time
		ok   bool
	}{
		"Donnerstag 7:00":          {at(2026, 10, 29, 7, 0), utcDay(2026, 10, 29), true},
		"Donnerstag 6:59":          {at(2026, 10, 29, 6, 59), time.Time{}, false},
		"Donnerstag 23:59":         {at(2026, 10, 29, 23, 59), utcDay(2026, 10, 29), true},
		"Freitag 9:00":             {at(2026, 10, 30, 9, 0), utcDay(2026, 10, 30), true},
		"Samstag 8:00":             {at(2026, 10, 31, 8, 0), time.Time{}, false},
		"Sonntag 8:00":             {at(2026, 11, 1, 8, 0), time.Time{}, false},
		"Montag 7:00":              {at(2026, 11, 2, 7, 0), utcDay(2026, 11, 2), true},
		"Sommerzeit 7:00 = 5 UTC":  {time.Date(2026, 7, 1, 5, 0, 0, 0, time.UTC), utcDay(2026, 7, 1), true},
		"Sommerzeit 6:59 = 4:59":   {time.Date(2026, 7, 1, 4, 59, 0, 0, time.UTC), time.Time{}, false},
		"Winterzeit 7:00 = 6 UTC":  {time.Date(2026, 12, 1, 6, 0, 0, 0, time.UTC), utcDay(2026, 12, 1), true},
		"Winterzeit 6:59 = 5:59":   {time.Date(2026, 12, 1, 5, 59, 0, 0, time.UTC), time.Time{}, false},
		"UTC-Freitag spaet = Sa.":  {time.Date(2026, 10, 30, 23, 30, 0, 0, time.UTC), time.Time{}, false},
		"UTC-Sonntag spaet = Mo.":  {time.Date(2026, 11, 1, 23, 30, 0, 0, time.UTC), time.Time{}, false},
		"Montag 7 Uhr ab UTC-Zeit": {time.Date(2026, 11, 2, 6, 0, 0, 0, time.UTC), utcDay(2026, 11, 2), true},
	} {
		got, ok := DueDay(tc.now, loc)
		if ok != tc.ok || !got.Equal(tc.want) {
			t.Errorf("%s: %v %v", name, got, ok)
		}
	}
}

func TestSchedulerRunsOncePerDayAndRetriesErrors(t *testing.T) {
	loc := Berlin()
	now := time.Date(2026, 10, 29, 6, 50, 0, 0, loc)
	fr := &fakeDayRunner{}
	s := &Scheduler{Runner: fr, Location: loc, Now: func() time.Time { return now }}
	ctx := context.Background()

	s.Tick(ctx)
	if fr.count() != 0 {
		t.Fatal("vor 7 Uhr versandt")
	}
	now = now.Add(15 * time.Minute) // 7:05
	s.Tick(ctx)
	s.Tick(ctx)
	if fr.count() != 1 || !fr.days[0].Equal(time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("Laeufe: %v", fr.days)
	}
	// Naechster Tag mit Fehler: wird wiederholt, bis er gelingt.
	now = now.AddDate(0, 0, 1)
	fr.err = errors.New("Datenbank weg")
	s.Tick(ctx)
	s.Tick(ctx)
	if fr.count() != 3 {
		t.Fatalf("Fehler nicht wiederholt: %d", fr.count())
	}
	fr.err = nil
	s.Tick(ctx)
	s.Tick(ctx)
	if fr.count() != 4 {
		t.Fatalf("nach Erfolg weiter versandt: %d", fr.count())
	}
	// Wochenende: nichts.
	now = now.AddDate(0, 0, 1) // Samstag
	s.Tick(ctx)
	if fr.count() != 4 {
		t.Fatalf("am Samstag versandt: %d", fr.count())
	}
}

func TestSchedulerStartTicksAndStops(t *testing.T) {
	loc := Berlin()
	var mu sync.Mutex
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, loc)
	fr := &fakeDayRunner{}
	s := &Scheduler{Runner: fr, Location: loc, Interval: 5 * time.Millisecond, Now: func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		now = now.AddDate(0, 0, 1) // jeder Tick ein neuer Tag
		return now
	}}
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	deadline := time.Now().Add(2 * time.Second)
	for fr.count() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if fr.count() < 3 {
		t.Fatalf("Ticks: %d", fr.count())
	}
	cancel()
	time.Sleep(30 * time.Millisecond)
	n := fr.count()
	time.Sleep(50 * time.Millisecond)
	if fr.count() != n {
		t.Fatal("nach dem Ende weiter getickt")
	}
}

// Ohne Zeitzonendaten bleibt der Versand bei 7 Uhr MEZ; mit ihnen ist es
// Europe/Berlin.
func TestBerlin(t *testing.T) {
	if Berlin().String() != "Europe/Berlin" && Berlin().String() != "MEZ" {
		t.Fatalf("Zeitzone: %s", Berlin())
	}
	if _, off := time.Date(2026, 1, 1, 12, 0, 0, 0, Berlin()).Zone(); off != 3600 {
		t.Fatalf("Winter: %d", off)
	}
}

func TestBerlinFallbackAndInterval(t *testing.T) {
	loc := berlin(func(string) (*time.Location, error) { return nil, errors.New("keine Zeitzonendaten") })
	if _, off := time.Date(2026, 7, 1, 12, 0, 0, 0, loc).Zone(); off != 3600 {
		t.Fatalf("Ersatzzone: %d", off)
	}
	if got := (&Scheduler{}).interval(); got != 5*time.Minute {
		t.Fatalf("Standardintervall: %v", got)
	}
	if got := (&Scheduler{Interval: time.Second}).interval(); got != time.Second {
		t.Fatalf("Intervall: %v", got)
	}
}
