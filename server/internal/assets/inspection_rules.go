package assets

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/hr"
)

// Fachregeln der Pruefungen ohne Datenbankzugriff (ADR 0028, Backlog
// F.3.5.1).

const (
	ErgebnisBestanden      = "bestanden"
	ErgebnisMaengel        = "maengel"
	ErgebnisNichtBestanden = "nicht_bestanden"
)

const (
	InspectionActive    = "active"
	InspectionStorniert = "storniert"
)

var ergebnisse = map[string]bool{
	ErgebnisBestanden:      true,
	ErgebnisMaengel:        true,
	ErgebnisNichtBestanden: true,
}

// Pruefstand eines Assets.
const (
	// Letzte Pruefung bestanden (oder mit Maengeln) und noch nicht faellig.
	PruefstandGeprueft = "geprueft"
	// Naechste Pruefung innerhalb der Vorwarnzeit faellig.
	PruefstandFaelligBald = "faellig_bald"
	// Naechste Pruefung ueberschritten.
	PruefstandUeberfaellig = "ueberfaellig"
	// Asset hat ein Pruefintervall, aber keine (nicht stornierte) Pruefung.
	PruefstandNieGeprueft = "nie_geprueft"
	// Letzte Pruefung nicht bestanden: darf nicht ausgegeben werden
	// (Entscheidung des Nutzers vom 2026-10-15).
	PruefstandGesperrt = "gesperrt"
	// Weder Pruefintervall noch Pruefung - z. B. ein Laptop.
	PruefstandNichtPruefpflichtig = "nicht_pruefpflichtig"
)

// DefaultInspectionWarnDays ist die Vorwarnzeit, wenn der Aufrufer keine
// nennt. 30 Tage statt 60 wie bei den Qualifikationen: eine Pruefung ist
// ein Termin mit einem Pruefer, kein Lehrgang mit Wartezeit (Annahme).
const DefaultInspectionWarnDays = 30

type Inspection struct {
	ID                 uuid.UUID  `json:"id"`
	AssetID            uuid.UUID  `json:"asset_id"`
	GeprueftAm         time.Time  `json:"geprueft_am"`
	Ergebnis           string     `json:"ergebnis"`
	NaechstePruefungAm *time.Time `json:"naechste_pruefung_am"`
	PrueferName        string     `json:"pruefer_name"`
	PrueferEmployeeID  *uuid.UUID `json:"pruefer_employee_id"`
	Notiz              string     `json:"notiz"`
	Status             string     `json:"status"`
	StornoReason       string     `json:"storno_reason,omitempty"`
	StorniertAt        *time.Time `json:"storniert_at,omitempty"`
	StorniertByUserID  *string    `json:"storniert_by_user_id,omitempty"`
	CreatedByUserID    *string    `json:"created_by_user_id"`
	CreatedAt          time.Time  `json:"created_at"`
}

// NextInspectionDue berechnet die naechste Pruefung aus dem Pruefdatum und
// dem Pruefintervall. Dieselbe Rechnung wie das Gueltig-bis der
// Qualifikationen: gleicher Tag N Monate spaeter, am Monatsende auf den
// letzten Tag des Zielmonats begrenzt. nil = kein Intervall.
func NextInspectionDue(geprueftAm time.Time, pruefintervallMonate *int) *time.Time {
	return hr.QualificationValidUntil(dateOnly(geprueftAm), pruefintervallMonate)
}

// ValidateInspection prueft die Angaben einer Pruefung. today wird
// uebergeben, damit die Regel testbar ist; verglichen werden Kalendertage.
func ValidateInspection(geprueftAm time.Time, ergebnis string, naechste *time.Time, prueferName string, anschaffungsdatum *time.Time, today time.Time) error {
	if geprueftAm.IsZero() {
		return errors.New("Prüfdatum erforderlich")
	}
	// Erfasst wird, was geprueft IST.
	if dateOnly(geprueftAm).After(dateOnly(today)) {
		return errors.New("Prüfdatum darf nicht in der Zukunft liegen")
	}
	if anschaffungsdatum != nil && dateOnly(geprueftAm).Before(dateOnly(*anschaffungsdatum)) {
		return errors.New("Prüfdatum darf nicht vor dem Anschaffungsdatum liegen")
	}
	if !ergebnisse[ergebnis] {
		return errors.New("Ungültiges Prüfergebnis")
	}
	if strings.TrimSpace(prueferName) == "" {
		return errors.New("Prüfer erforderlich")
	}
	if naechste != nil {
		// Ein durchgefallenes Stueck wird repariert und neu geprueft oder
		// ausgemustert - ein Folgetermin waere irrefuehrend.
		if ergebnis == ErgebnisNichtBestanden {
			return errors.New("Bei nicht bestandener Prüfung gibt es keine nächste Prüfung")
		}
		if !dateOnly(*naechste).After(dateOnly(geprueftAm)) {
			return errors.New("Nächste Prüfung muss nach dem Prüfdatum liegen")
		}
	}
	return nil
}

// InspectionStanding ist der Pruefstand EINES Assets.
type InspectionStanding struct {
	Status string `json:"status"`
	// Die folgenden Angaben beschreiben die massgebliche Pruefung und
	// fehlen, wenn es keine gibt.
	InspectionID       *uuid.UUID `json:"inspection_id,omitempty"`
	GeprueftAm         *time.Time `json:"geprueft_am,omitempty"`
	Ergebnis           string     `json:"ergebnis,omitempty"`
	NaechstePruefungAm *time.Time `json:"naechste_pruefung_am,omitempty"`
	// TageBisFaellig: Tage bis einschliesslich NaechstePruefungAm; negativ,
	// wenn ueberschritten. nil ohne Folgetermin.
	TageBisFaellig *int `json:"tage_bis_faellig,omitempty"`
	// Maengel: die massgebliche Pruefung hat Maengel festgestellt. Das
	// Stueck bleibt ausgebbar (Annahme: "Maengel" heisst nutzbar mit
	// Auflage, "nicht bestanden" heisst gesperrt); der Hinweis soll aber
	// sichtbar sein.
	Maengel bool `json:"maengel"`
}

// EvaluateInspections berechnet den Pruefstand eines Assets am Tag today.
//
// Massgeblich ist die JUENGSTE nicht stornierte Pruefung (spaetestes
// Pruefdatum, bei Gleichstand die zuletzt erfasste) - anders als bei den
// Qualifikationen, wo der am laengsten gueltige Nachweis zaehlt: eine
// durchgefallene Pruefung muss eine fruehere bestandene abloesen, sonst
// bliebe ein defekter Auffanggurt ausgebbar.
//
// pruefintervallMonate unterscheidet "nie geprueft" (mit Intervall) von
// "nicht pruefpflichtig" (ohne).
func EvaluateInspections(records []Inspection, pruefintervallMonate *int, today time.Time, warnDays int) InspectionStanding {
	if warnDays < 0 {
		warnDays = 0
	}
	today = dateOnly(today)

	var latest *Inspection
	for i := range records {
		r := &records[i]
		if r.Status == InspectionStorniert {
			continue
		}
		if latest == nil || inspectionIsNewer(*r, *latest) {
			latest = r
		}
	}
	if latest == nil {
		if pruefintervallMonate != nil {
			return InspectionStanding{Status: PruefstandNieGeprueft}
		}
		return InspectionStanding{Status: PruefstandNichtPruefpflichtig}
	}

	rec := *latest
	st := InspectionStanding{
		InspectionID: &rec.ID, GeprueftAm: &rec.GeprueftAm, Ergebnis: rec.Ergebnis,
		NaechstePruefungAm: rec.NaechstePruefungAm, Maengel: rec.Ergebnis == ErgebnisMaengel,
	}
	if rec.Ergebnis == ErgebnisNichtBestanden {
		st.Status = PruefstandGesperrt
		return st
	}
	if rec.NaechstePruefungAm == nil {
		st.Status = PruefstandGeprueft
		return st
	}
	days := int(dateOnly(*rec.NaechstePruefungAm).Sub(today).Hours() / 24)
	st.TageBisFaellig = &days
	switch {
	case days < 0:
		st.Status = PruefstandUeberfaellig
	case days <= warnDays:
		st.Status = PruefstandFaelligBald
	default:
		st.Status = PruefstandGeprueft
	}
	return st
}

// inspectionIsNewer sagt, ob a juenger ist als b.
func inspectionIsNewer(a, b Inspection) bool {
	if !a.GeprueftAm.Equal(b.GeprueftAm) {
		return a.GeprueftAm.After(b.GeprueftAm)
	}
	return a.CreatedAt.After(b.CreatedAt)
}
