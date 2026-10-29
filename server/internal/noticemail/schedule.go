package noticemail

import (
	"context"
	"log"
	"sync"
	"time"
)

// Zeitsteuerung der Sammelmails (Backlog F.14.4b): werktags (Montag bis
// Freitag) ab 7:00 Uhr deutscher Zeit, einmal je Tag. Laeuft im
// API-Prozess, nicht in den Tests (app.New startet sie). Startet der
// Prozess erst spaeter am Tag, wird nachgeholt. Feiertage werden nicht
// ausgenommen (ANNAHME).
//
// Mehrere Instanzen sind unschaedlich: jede laeuft einmal am Tag, und das
// Protokoll (eindeutig je Benutzer und Tag, vor dem Versand angelegt)
// verhindert doppelten Versand.

const (
	SendHour        = 7
	DefaultInterval = 5 * time.Minute
)

// Berlin ist die Zeitzone des Versands. Ohne Zeitzonendaten im System
// (minimales Container-Image) gilt ersatzweise MEZ ohne Sommerzeit.
func Berlin() *time.Location { return berlin(time.LoadLocation) }

func berlin(load func(string) (*time.Location, error)) *time.Location {
	if loc, err := load("Europe/Berlin"); err == nil {
		return loc
	}
	return time.FixedZone("MEZ", 3600)
}

// DueDay liefert den Versandtag (Datum in UTC, 00:00), wenn now in loc ein
// Werktag ab SendHour ist.
func DueDay(now time.Time, loc *time.Location) (time.Time, bool) {
	l := now.In(loc)
	if wd := l.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return time.Time{}, false
	}
	if l.Hour() < SendHour {
		return time.Time{}, false
	}
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.UTC), true
}

// dayRunner ist Runner.RunDay; Tests setzen eine Attrappe ein.
type dayRunner interface {
	RunDay(ctx context.Context, day time.Time) (Result, error)
}

type Scheduler struct {
	Runner   dayRunner
	Location *time.Location   // nil: Berlin()
	Interval time.Duration    // 0: DefaultInterval
	Now      func() time.Time // nil: time.Now

	mu      sync.Mutex
	lastDay time.Time
}

// Tick prueft einmal, ob heute versandt werden muss, und tut es. Ein
// Fehler wird geloggt (ohne Inhalt) und beim naechsten Tick wiederholt;
// erst ein erfolgreicher Lauf hakt den Tag ab.
func (s *Scheduler) Tick(ctx context.Context) {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	loc := s.Location
	if loc == nil {
		loc = Berlin()
	}
	day, ok := DueDay(now(), loc)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastDay.Equal(day) {
		return
	}
	res, err := s.Runner.RunDay(ctx, day)
	if err != nil {
		log.Printf("[Hinweis-Mails] Versand %s abgebrochen, neuer Versuch beim nächsten Durchlauf", day.Format("2006-01-02"))
		return
	}
	s.lastDay = day
	if res.Gesendet > 0 || res.Fehler > 0 {
		log.Printf("[Hinweis-Mails] Versand %s: %d gesendet, %d fehlgeschlagen", day.Format("2006-01-02"), res.Gesendet, res.Fehler)
	}
}

func (s *Scheduler) interval() time.Duration {
	if s.Interval <= 0 {
		return DefaultInterval
	}
	return s.Interval
}

// Start laeuft bis ctx endet: sofort ein Tick, dann je Intervall.
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(s.interval())
		defer t.Stop()
		for {
			s.Tick(ctx)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}
