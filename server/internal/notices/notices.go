// Package notices berechnet die Hinweise fuer einen Benutzer (Backlog
// F.10.4.1): was im Betrieb faellig ist und was ihn selbst betrifft, je
// nach seinen Rechten.
//
// Die Hinweise entstehen an EINER Stelle (Service.For) - der Kanal "im
// System" (GET /notices, Dashboard) liest sie, und ein spaeterer
// E-Mail-Versand (F.14) verschickt dieselben ueber einen Sender.
//
// DSGVO: ein Hinweis enthaelt nur Arten und Anzahlen, nie Namen anderer
// Mitarbeiter - auch nicht fuer den spaeteren Mailtext. Die Namen stehen in
// den Faelligkeitslisten, die der Verweis oeffnet und die ihre Rechte
// selbst pruefen.
package notices

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Arten von Hinweisen. Der Client oeffnet je Art die passende Liste.
const (
	KindQualificationsDue  = "qualifications_due"    // betriebsweit, qualifications.read
	KindOwnQualifications  = "own_qualifications"    // eigene, qualifications.self
	KindAssetInspectionDue = "asset_inspections_due" // assets.read
	KindFleetSchedulesDue  = "fleet_schedules_due"   // fleet.read
)

// Dringlichkeit eines Hinweises.
const (
	SeverityKritisch = "kritisch" // etwas ist abgelaufen, fehlt, ueberfaellig oder gesperrt
	SeverityHinweis  = "hinweis"  // nur bald faellig
)

// Notice ist ein Hinweis: eine Art mit Anzahlen je Stand.
type Notice struct {
	Kind     string         `json:"kind"`
	Title    string         `json:"title"`
	Severity string         `json:"severity"`
	Total    int            `json:"total"`
	Counts   map[string]int `json:"counts"`
}

// Recipient ist der Empfaenger: der Benutzer mit seinen Rechten und, falls
// verknuepft, seinem Mitarbeiter.
type Recipient struct {
	UserID     string
	CompanyID  string
	EmployeeID *uuid.UUID
	// Can prueft ein Recht (Superuser eingeschlossen).
	Can func(permission string) bool
}

// Source liefert Hinweise eines Bereichs; keine, wenn der Empfaenger das
// Recht nicht hat oder nichts faellig ist.
type Source interface {
	Notices(ctx context.Context, r Recipient, today time.Time) ([]Notice, error)
}

// Sender verschickt Hinweise ueber einen weiteren Kanal. Vorbereitet fuer
// den E-Mail-Versand (F.14, wartet auf den SMTP-Zugang); noch ohne
// Umsetzung - der Kanal "im System" braucht keinen Sender, er liest
// Service.For.
type Sender interface {
	Send(ctx context.Context, r Recipient, notices []Notice) error
}

type Service struct{ sources []Source }

func NewService(sources ...Source) *Service { return &Service{sources: sources} }

// For liefert alle Hinweise fuer den Empfaenger, die kritischen zuerst,
// darin in der Reihenfolge der Quellen.
func (s *Service) For(ctx context.Context, r Recipient, today time.Time) ([]Notice, error) {
	out := make([]Notice, 0)
	for _, src := range s.sources {
		ns, err := src.Notices(ctx, r, today)
		if err != nil {
			return nil, err
		}
		out = append(out, ns...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Severity == SeverityKritisch && out[j].Severity != SeverityKritisch
	})
	return out, nil
}

// countNotice zaehlt die Staende; ohne Eintraege gibt es keinen Hinweis.
// Kritisch ist er, sobald ein Stand aus critical vorkommt.
func countNotice(kind, title string, statuses []string, critical map[string]bool) []Notice {
	if len(statuses) == 0 {
		return nil
	}
	n := Notice{Kind: kind, Title: title, Severity: SeverityHinweis, Counts: map[string]int{}}
	for _, st := range statuses {
		n.Counts[st]++
		n.Total++
		if critical[st] {
			n.Severity = SeverityKritisch
		}
	}
	return []Notice{n}
}
