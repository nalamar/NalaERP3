package hr

import (
	"errors"
	"sort"
	"time"
	_ "time/tzdata" // Zeitzonendaten einbetten, siehe workLocation

	"github.com/google/uuid"
)

// Fachregeln der Zeiterfassung ohne Datenbankzugriff (ADR 0024, Backlog
// F.1.3). Zwei Dinge, die bewusst getrennt bleiben:
//
//   - ValidateTimeEntry lehnt ab, was nicht stimmen KANN.
//   - EvaluateWorkDays weist aus, was gegen das ArbZG verstoesst.
//
// Ein ArbZG-Verstoss ist nie ein Validierungsfehler: aufzuzeichnen ist die
// tatsaechlich geleistete Arbeitszeit. Ein System, das einen 11-Stunden-Tag
// nicht annimmt, erzwingt eine falsche Aufzeichnung.

const (
	TimeEntryStatusActive    = "active"
	TimeEntryStatusStorniert = "storniert"
)

// TimeEntry entspricht einer Zeile in hr_time_entries.
type TimeEntry struct {
	ID                uuid.UUID  `json:"id"`
	EmployeeID        uuid.UUID  `json:"employee_id"`
	WorkDate          time.Time  `json:"work_date"`
	StartedAt         time.Time  `json:"started_at"`
	EndedAt           *time.Time `json:"ended_at,omitempty"`
	BreakMinutes      int        `json:"break_minutes"`
	ProjectID         *uuid.UUID `json:"project_id,omitempty"`
	Note              string     `json:"note"`
	Status            string     `json:"status"`
	StornoReason      string     `json:"storno_reason,omitempty"`
	StorniertAt       *time.Time `json:"storniert_at,omitempty"`
	StorniertByUserID *string    `json:"storniert_by_user_id,omitempty"`
	ReplacesEntryID   *uuid.UUID `json:"replaces_entry_id,omitempty"`
	CreatedByUserID   *string    `json:"created_by_user_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

// workLocation ist die eine Zeitzone, in der ein Eintrag seinem Arbeitstag
// zugeordnet wird (Annahme aus ADR 0024: eine Zeitzone fuer das ganze
// System). Die Zeitzonendaten sind ueber time/tzdata in das Binary
// eingebettet, damit die Zuordnung nicht davon abhaengt, ob das
// Laufzeit-Image ein tzdata-Paket mitbringt - ohne sie wuerde LoadLocation
// dort fehlschlagen und jeder Eintrag kurz nach Mitternacht dem falschen
// Tag zugeordnet.
var workLocation = mustLoadLocation("Europe/Berlin")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic("hr: Zeitzone " + name + " nicht ladbar: " + err.Error())
	}
	return loc
}

// WorkDateOf liefert den Arbeitstag eines Eintrags: den Kalendertag des
// Beginns in Europe/Berlin, als Mitternacht UTC (so, wie Postgres eine
// date-Spalte liefert). Eine Nachtschicht gehoert zu dem Tag, an dem sie
// beginnt.
func WorkDateOf(startedAt time.Time) time.Time {
	y, m, d := startedAt.In(workLocation).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ValidateTimeEntry prueft einen einzelnen Eintrag fuer sich. now ist der
// Vergleichszeitpunkt fuer "in der Zukunft" und wird uebergeben statt
// intern ermittelt, damit die Regel testbar ist.
//
// Nicht Teil dieser Funktion, weil dafuer andere Eintraege noetig sind:
// Ueberschneidung mit einem anderen Eintrag und "hoechstens ein laufender
// Eintrag" (beides im Service, F.1.4).
func ValidateTimeEntry(e TimeEntry, now time.Time) error {
	if e.StartedAt.IsZero() {
		return errors.New("Beginn erforderlich")
	}
	if e.StartedAt.After(now) {
		return errors.New("Beginn darf nicht in der Zukunft liegen")
	}
	if e.BreakMinutes < 0 {
		return errors.New("Pause darf nicht negativ sein")
	}
	if e.EndedAt == nil {
		// Bei einem laufenden Eintrag ist die Pause noch nicht gegen die
		// Anwesenheit pruefbar; sie wird beim Beenden angegeben.
		if e.BreakMinutes != 0 {
			return errors.New("Pause kann erst beim Beenden angegeben werden")
		}
		return nil
	}
	if !e.EndedAt.After(e.StartedAt) {
		return errors.New("Ende muss nach dem Beginn liegen")
	}
	if e.EndedAt.After(now) {
		return errors.New("Ende darf nicht in der Zukunft liegen")
	}
	if time.Duration(e.BreakMinutes)*time.Minute >= e.EndedAt.Sub(e.StartedAt) {
		return errors.New("Pause muss kürzer sein als die Anwesenheit")
	}
	return nil
}

// Hinweise der Tagesauswertung. Gesetzliche Grundwerte des ArbZG; der
// Ausgleichszeitraum nach §3 Satz 2, tarifliche Abweichungen (§7),
// Sonn-/Feiertagsarbeit (§9) und das JArbSchG sind nicht abgebildet
// (Backlog F.6).
const (
	HinweisUeber8h                = "ueber_8h"                // §3 Satz 1
	HinweisUeber10h               = "ueber_10h"               // §3 Satz 2
	HinweisPauseZuKurz            = "pause_zu_kurz"           // §4 Satz 1
	HinweisRuhezeitUnterschritten = "ruhezeit_unterschritten" // §5 Abs. 1
)

const (
	arbzgRegelArbeitszeit   = 8 * time.Hour
	arbzgHoechstArbeitszeit = 10 * time.Hour
	arbzgPauseAb            = 6 * time.Hour
	arbzgLangePauseAb       = 9 * time.Hour
	arbzgPause              = 30 * time.Minute
	arbzgLangePause         = 45 * time.Minute
	arbzgPausenabschnitt    = 15 * time.Minute
	arbzgRuhezeit           = 11 * time.Hour
)

// DayEvaluation ist die Auswertung eines Arbeitstags eines Mitarbeiters.
// Sie wird bei jedem Lesen berechnet und nie gespeichert.
type DayEvaluation struct {
	WorkDate     string `json:"work_date"`
	NetMinutes   int    `json:"net_minutes"`
	BreakMinutes int    `json:"break_minutes"`
	// Open ist true, solange an dem Tag ein Eintrag laeuft. Der laufende
	// Eintrag zaehlt in keiner Summe mit; die Hinweise des Tages sind dann
	// vorlaeufig.
	Open     bool     `json:"open"`
	Hinweise []string `json:"hinweise"`
}

// EvaluateWorkDays wertet die Eintraege EINES Mitarbeiters je Arbeitstag
// aus, aufsteigend nach Tag. Stornierte Eintraege werden uebergangen.
//
// Arbeitszeit ist die Anwesenheit ohne Ruhepausen (§2 Abs. 1 ArbZG). Als
// Ruhepause zaehlen die am Eintrag angegebenen Pausenminuten sowie Luecken
// zwischen zwei Eintraegen desselben Tages - Luecken aber nur ab 15
// Minuten, weil kuerzere Unterbrechungen nach §4 Satz 2 keine Ruhepause
// sind. Fuer die Pausenminuten am Eintrag ist diese Abschnittsregel nicht
// pruefbar: dort liegt nur die Summe vor.
//
// Die Ruhezeit wird zwischen dem letzten Ende eines Tages und dem ersten
// Beginn des naechsten in der Eingabe enthaltenen Tages geprueft; der
// Hinweis steht am spaeteren Tag. Wer einen Zeitraum auswertet, muss
// deshalb den Tag davor mit uebergeben, sonst fehlt dem ersten Tag der
// Vergleich.
func EvaluateWorkDays(entries []TimeEntry) []DayEvaluation {
	byDay := make(map[string][]TimeEntry)
	for _, e := range entries {
		if e.Status == TimeEntryStatusStorniert {
			continue
		}
		key := e.WorkDate.Format("2006-01-02")
		byDay[key] = append(byDay[key], e)
	}
	days := make([]string, 0, len(byDay))
	for k := range byDay {
		days = append(days, k)
	}
	sort.Strings(days)

	out := make([]DayEvaluation, 0, len(days))
	var prevEnd *time.Time
	for _, day := range days {
		list := byDay[day]
		sort.Slice(list, func(i, j int) bool { return list[i].StartedAt.Before(list[j].StartedAt) })

		ev := DayEvaluation{WorkDate: day, Hinweise: []string{}}
		var net, pause time.Duration
		var lastEnd *time.Time
		for _, e := range list {
			if e.EndedAt == nil {
				ev.Open = true
				continue
			}
			if lastEnd != nil {
				if gap := e.StartedAt.Sub(*lastEnd); gap >= arbzgPausenabschnitt {
					pause += gap
				}
			}
			own := time.Duration(e.BreakMinutes) * time.Minute
			pause += own
			net += e.EndedAt.Sub(e.StartedAt) - own
			if lastEnd == nil || e.EndedAt.After(*lastEnd) {
				lastEnd = e.EndedAt
			}
		}
		ev.NetMinutes = int(net / time.Minute)
		ev.BreakMinutes = int(pause / time.Minute)

		if net > arbzgRegelArbeitszeit {
			ev.Hinweise = append(ev.Hinweise, HinweisUeber8h)
		}
		if net > arbzgHoechstArbeitszeit {
			ev.Hinweise = append(ev.Hinweise, HinweisUeber10h)
		}
		if (net > arbzgLangePauseAb && pause < arbzgLangePause) || (net > arbzgPauseAb && pause < arbzgPause) {
			ev.Hinweise = append(ev.Hinweise, HinweisPauseZuKurz)
		}
		if prevEnd != nil && list[0].StartedAt.Sub(*prevEnd) < arbzgRuhezeit {
			ev.Hinweise = append(ev.Hinweise, HinweisRuhezeitUnterschritten)
		}
		if lastEnd != nil {
			prevEnd = lastEnd
		}
		out = append(out, ev)
	}
	return out
}
