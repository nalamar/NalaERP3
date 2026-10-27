package hr

import (
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Fachregeln der Qualifikationen ohne Datenbankzugriff (ADR 0027, Backlog
// F.2.4.1): wie lange ein Nachweis gilt, und wie der Stand eines
// Mitarbeiters daraus folgt. Der Stand wird bei jedem Lesen berechnet und
// nie gespeichert - er haengt vom heutigen Tag ab.

// EmployeeQualification entspricht einer Zeile in
// hr_employee_qualifications: ein Nachweis.
type EmployeeQualification struct {
	ID         uuid.UUID `json:"id"`
	EmployeeID uuid.UUID `json:"employee_id"`
	TypeID     uuid.UUID `json:"type_id"`
	ErworbenAm time.Time `json:"erworben_am"`
	// GueltigBis ist der letzte Tag der Gueltigkeit (einschliesslich).
	// nil = unbefristet.
	GueltigBis        *time.Time `json:"gueltig_bis"`
	Aussteller        string     `json:"aussteller"`
	NachweisNr        string     `json:"nachweis_nr"`
	Note              string     `json:"note"`
	Status            string     `json:"status"`
	StornoReason      string     `json:"storno_reason,omitempty"`
	StorniertAt       *time.Time `json:"storniert_at,omitempty"`
	StorniertByUserID *string    `json:"storniert_by_user_id,omitempty"`
	CreatedByUserID   *string    `json:"created_by_user_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

const (
	QualificationRecordActive    = "active"
	QualificationRecordStorniert = "storniert"
)

// Stand einer Art bei einem Mitarbeiter.
const (
	QualificationStandGueltig    = "gueltig"    // gilt laenger als die Vorwarnzeit
	QualificationStandLaeuftAb   = "laeuft_ab"  // gilt noch, Ablauf innerhalb der Vorwarnzeit
	QualificationStandAbgelaufen = "abgelaufen" // letzter gueltiger Tag ist vorbei
	QualificationStandFehlt      = "fehlt"      // erforderlich, aber kein aktiver Nachweis
)

// DefaultQualificationWarnDays ist die Vorwarnzeit, ab der eine anstehende
// Erneuerung angemahnt wird (Annahme aus ADR 0027; je Abfrage aenderbar).
const DefaultQualificationWarnDays = 60

// QualificationValidUntil berechnet den letzten gueltigen Tag eines
// Nachweises aus dem Erwerbsdatum und der Frist der Art. Ohne Frist (nil)
// ist der Nachweis unbefristet.
//
// Der Tag gleichen Datums nach Ablauf der Frist zaehlt noch als gueltig:
// eine am 15.03.2026 erfolgte jaehrliche Unterweisung gilt bis
// einschliesslich 15.03.2027 ("mindestens einmal jaehrlich" - Annahme, die
// Auslegung "bis zum Vortag" waere einen Tag strenger).
//
// Gibt es den Tag im Zielmonat nicht, gilt dessen letzter Tag: 31.01. plus
// ein Monat ist der 28.02. (bzw. 29.02.). time.AddDate wuerde hier in den
// Maerz ueberlaufen und einen Nachweis damit je nach Erwerbstag um bis zu
// drei Tage laenger gelten lassen.
func QualificationValidUntil(erworbenAm time.Time, gueltigkeitMonate *int) *time.Time {
	if gueltigkeitMonate == nil {
		return nil
	}
	y, m, d := erworbenAm.Date()
	first := time.Date(y, m+time.Month(*gueltigkeitMonate), 1, 0, 0, 0, 0, time.UTC)
	if last := first.AddDate(0, 1, -1).Day(); d > last {
		d = last
	}
	until := time.Date(first.Year(), first.Month(), d, 0, 0, 0, 0, time.UTC)
	return &until
}

// ValidateQualificationDates prueft die Datumsangaben eines Nachweises.
// today wird uebergeben statt intern ermittelt, damit die Regel testbar
// ist; verglichen werden Kalendertage.
func ValidateQualificationDates(erworbenAm time.Time, gueltigBis *time.Time, today time.Time) error {
	if erworbenAm.IsZero() {
		return errors.New("Erwerbsdatum erforderlich")
	}
	// Erfasst wird, was erreicht IST (Vorgabe des Nutzers vom 2026-10-13),
	// nicht was geplant ist.
	if dateOnly(erworbenAm).After(dateOnly(today)) {
		return errors.New("Erwerbsdatum darf nicht in der Zukunft liegen")
	}
	if gueltigBis != nil && dateOnly(*gueltigBis).Before(dateOnly(erworbenAm)) {
		return errors.New("Gültig-bis darf nicht vor dem Erwerbsdatum liegen")
	}
	return nil
}

// QualificationStanding ist der Stand EINER Art bei einem Mitarbeiter.
type QualificationStanding struct {
	TypeID uuid.UUID `json:"type_id"`
	Status string    `json:"status"`
	// Erforderlich: die Art ist dem Mitarbeiter als Anforderung zugeordnet
	// oder fuer alle Pflicht.
	Erforderlich bool `json:"erforderlich"`
	// Die folgenden Angaben beschreiben den massgeblichen Nachweis und
	// fehlen beim Stand "fehlt".
	QualificationID *uuid.UUID `json:"qualification_id,omitempty"`
	ErworbenAm      *time.Time `json:"erworben_am,omitempty"`
	GueltigBis      *time.Time `json:"gueltig_bis,omitempty"`
	// TageBisAblauf: verbleibende Tage bis einschliesslich GueltigBis bzw.
	// BestaetigungFaelligAm, je nachdem was frueher liegt; negativ, wenn
	// abgelaufen. nil bei unbefristeten Nachweisen und bei "fehlt".
	TageBisAblauf *int `json:"tage_bis_ablauf,omitempty"`
	// BestaetigungFaelligAm: letzter Tag, bis zu dem die naechste
	// Zwischenbestaetigung vorliegen muss (F.10.3). Nur bei Arten mit
	// Bestaetigungsintervall und nur, solange er nicht nach GueltigBis liegt.
	BestaetigungFaelligAm *time.Time `json:"bestaetigung_faellig_am,omitempty"`
	// AblaufGrund ist "bestaetigung", wenn der Stand an der faelligen
	// Bestaetigung haengt statt am Gueltig-bis des Nachweises.
	AblaufGrund string `json:"ablauf_grund,omitempty"`
}

// QualificationAblaufBestaetigung: der Stand haengt an der Bestaetigung.
const QualificationAblaufBestaetigung = "bestaetigung"

// QualificationConfirmationDue berechnet, bis wann ein Nachweis zuletzt
// bestaetigt sein muss (F.10.3, DIN EN ISO 9606-1): monate nach dem Erwerb,
// jede rechtzeitige Bestaetigung verschiebt die Frist auf monate nach
// IHREM Datum.
//
// Eine Bestaetigung NACH Ablauf der Frist rettet den Nachweis nicht - ohne
// rechtzeitige Bestaetigung verliert das Zertifikat seine Gueltigkeit. Die
// Kette endet deshalb an der ersten verspaeteten Bestaetigung; diese und
// alle spaeteren zaehlen nicht. Bestaetigungen vor dem Erwerb zaehlen
// ebenfalls nicht. Die Reihenfolge von confirmations ist beliebig.
func QualificationConfirmationDue(erworbenAm time.Time, confirmations []time.Time, monate int) time.Time {
	due := *QualificationValidUntil(dateOnly(erworbenAm), &monate)
	sorted := make([]time.Time, 0, len(confirmations))
	for _, c := range confirmations {
		sorted = append(sorted, dateOnly(c))
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })
	for _, c := range sorted {
		if c.Before(dateOnly(erworbenAm)) {
			continue
		}
		if c.After(due) {
			break
		}
		due = *QualificationValidUntil(c, &monate)
	}
	return due
}

// QualificationConfirmations sind die Angaben zur Zwischenbestaetigung fuer
// EvaluateQualificationsWithConfirmations: die Daten der nicht stornierten
// Bestaetigungen je Nachweis und das Intervall je Art (nur Arten mit
// Intervall).
type QualificationConfirmations struct {
	Dates  map[uuid.UUID][]time.Time
	Months map[uuid.UUID]int
}

// EvaluateQualifications berechnet den Stand EINES Mitarbeiters am Tag
// today: je Art, fuer die er einen aktiven Nachweis hat oder die fuer ihn
// erforderlich ist, ein Eintrag. Sortiert nach TypeID, damit das Ergebnis
// stabil ist; eine fachliche Sortierung macht der Aufrufer, der die Namen
// der Arten kennt.
//
// records sind die Nachweise des Mitarbeiters; stornierte werden
// uebergangen. requiredTypeIDs sind die Arten, die fuer ihn erforderlich
// sind - seine eigenen Anforderungen und die, die fuer alle Pflicht sind.
// Stillgelegte Arten kennt diese Funktion nicht: der Aufrufer nimmt sie
// aus requiredTypeIDs heraus.
//
// Massgeblich ist je Art der Nachweis, der am LAENGSTEN gilt (unbefristet
// vor befristet, sonst das spaeteste Gueltig-bis; bei Gleichstand der
// juengere). Das beantwortet die Frage, auf die es ankommt: ist der
// Mitarbeiter heute gedeckt? Wer zuerst eine fuenf Jahre gueltige
// Ausbildung und spaeter eine Auffrischung mit kuerzerer Laufzeit erfasst
// bekommt, soll nicht als "abgelaufen" gelten, solange die Ausbildung gilt.
func EvaluateQualifications(records []EmployeeQualification, requiredTypeIDs []uuid.UUID, today time.Time, warnDays int) []QualificationStanding {
	return EvaluateQualificationsWithConfirmations(records, QualificationConfirmations{}, requiredTypeIDs, today, warnDays)
}

// EvaluateQualificationsWithConfirmations ist EvaluateQualifications mit
// Zwischenbestaetigung (F.10.3): bei Arten mit Intervall gilt ein Nachweis
// nur bis zum frueheren von Gueltig-bis und faelliger Bestaetigung - eine
// ueberfaellige Bestaetigung wirkt wie ein Ablauf (Entscheidung des
// Nutzers vom 2026-10-26). Auch die Wahl des massgeblichen Nachweises
// richtet sich nach dieser wirksamen Gueltigkeit.
func EvaluateQualificationsWithConfirmations(records []EmployeeQualification, conf QualificationConfirmations, requiredTypeIDs []uuid.UUID, today time.Time, warnDays int) []QualificationStanding {
	if warnDays < 0 {
		warnDays = 0
	}
	today = dateOnly(today)

	best := make(map[uuid.UUID]EmployeeQualification)
	confirmDue := make(map[uuid.UUID]*time.Time)
	for _, r := range records {
		if r.Status == QualificationRecordStorniert {
			continue
		}
		if m, ok := conf.Months[r.TypeID]; ok && m > 0 {
			due := QualificationConfirmationDue(r.ErworbenAm, conf.Dates[r.ID], m)
			// Nach dem Ablauf ist keine Bestaetigung mehr faellig.
			if r.GueltigBis == nil || !due.After(dateOnly(*r.GueltigBis)) {
				confirmDue[r.ID] = &due
			}
		}
		if cur, ok := best[r.TypeID]; !ok || qualificationLastsLonger(effectiveRecord(r, confirmDue[r.ID]), effectiveRecord(cur, confirmDue[cur.ID])) {
			best[r.TypeID] = r
		}
	}
	required := make(map[uuid.UUID]bool, len(requiredTypeIDs))
	for _, id := range requiredTypeIDs {
		required[id] = true
	}

	out := make([]QualificationStanding, 0, len(best)+len(required))
	for typeID, r := range best {
		rec := r
		st := QualificationStanding{TypeID: typeID, Erforderlich: required[typeID], QualificationID: &rec.ID, ErworbenAm: &rec.ErworbenAm, GueltigBis: rec.GueltigBis}
		until := rec.GueltigBis
		if due := confirmDue[rec.ID]; due != nil {
			st.BestaetigungFaelligAm = due
			if until == nil || due.Before(dateOnly(*until)) {
				until = due
				st.AblaufGrund = QualificationAblaufBestaetigung
			}
		}
		if until == nil {
			st.Status = QualificationStandGueltig
		} else {
			days := int(dateOnly(*until).Sub(today).Hours() / 24)
			st.TageBisAblauf = &days
			switch {
			case days < 0:
				st.Status = QualificationStandAbgelaufen
			case days <= warnDays:
				st.Status = QualificationStandLaeuftAb
			default:
				st.Status = QualificationStandGueltig
			}
		}
		out = append(out, st)
	}
	for typeID := range required {
		if _, ok := best[typeID]; !ok {
			out = append(out, QualificationStanding{TypeID: typeID, Status: QualificationStandFehlt, Erforderlich: true})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TypeID.String() < out[j].TypeID.String() })
	return out
}

// effectiveRecord ist r mit der wirksamen Gueltigkeit: dem frueheren von
// Gueltig-bis und faelliger Bestaetigung.
func effectiveRecord(r EmployeeQualification, confirmDue *time.Time) EmployeeQualification {
	if confirmDue != nil && (r.GueltigBis == nil || confirmDue.Before(dateOnly(*r.GueltigBis))) {
		r.GueltigBis = confirmDue
	}
	return r
}

// qualificationLastsLonger sagt, ob a laenger gilt als b.
func qualificationLastsLonger(a, b EmployeeQualification) bool {
	switch {
	case a.GueltigBis == nil && b.GueltigBis == nil:
		return a.ErworbenAm.After(b.ErworbenAm)
	case a.GueltigBis == nil:
		return true
	case b.GueltigBis == nil:
		return false
	case !a.GueltigBis.Equal(*b.GueltigBis):
		return a.GueltigBis.After(*b.GueltigBis)
	default:
		return a.ErworbenAm.After(b.ErworbenAm)
	}
}
