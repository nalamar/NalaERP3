package fleet

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/hr"
)

// Fachregeln der Termine ohne Datenbankzugriff (ADR 0032 §3, Backlog
// G.2.1). Ein Termin ist Planung (Starttermin, Intervall), eine Erledigung
// ist ein Beleg mit Ergebnis und gespeichertem Folgetermin.

const (
	TerminHU           = "hu"
	TerminUVV          = "uvv"
	TerminWartung      = "wartung"
	TerminVersicherung = "versicherung"
	TerminLeasing      = "leasing"
	TerminSonstige     = "sonstige"
)

var terminArten = map[string]bool{
	TerminHU: true, TerminUVV: true, TerminWartung: true, TerminVersicherung: true, TerminLeasing: true, TerminSonstige: true,
}

// pruefArten haben ein Pruefergebnis; ihr Ergebnis "maengel" oder
// "nicht_bestanden" sperrt das Fahrzeug (Entscheidung des Nutzers vom
// 2026-10-25). Alle anderen Termine werden schlicht "erledigt".
var pruefArten = map[string]bool{TerminHU: true, TerminUVV: true}

const (
	ErgebnisBestanden      = "bestanden"
	ErgebnisMaengel        = "maengel"
	ErgebnisNichtBestanden = "nicht_bestanden"
	ErgebnisErledigt       = "erledigt"
)

const (
	CompletionActive    = "active"
	CompletionStorniert = "storniert"
)

// Stand eines Termins.
const (
	// Weder Starttermin noch Folgetermin bekannt.
	TerminstandOhneTermin = "ohne_termin"
	// Faellig nach der Vorwarnzeit.
	TerminstandOK = "ok"
	// Faellig innerhalb der Vorwarnzeit (heute eingeschlossen).
	TerminstandFaelligBald = "faellig_bald"
	// Faelligkeit ueberschritten.
	TerminstandUeberfaellig = "ueberfaellig"
	// Juengste HU/UVV mit Maengeln oder nicht bestanden.
	TerminstandGesperrt = "gesperrt"
)

// DefaultScheduleWarnDays: 30 Tage wie bei den Pruefungen der
// Betriebsmittel (ADR 0032 §3).
const DefaultScheduleWarnDays = 30

// DefaultIntervalMonths ist der Vorschlag fuer das Intervall einer neuen
// Terminart (aenderbar am Termin): HU 24 Monate (Pkw nach der ersten HU;
// Fristen je Fahrzeugart sind verzweigt - der Folgetermin wird deshalb beim
// Erledigen angegeben, nicht festgelegt), UVV 12 (DGUV Vorschrift 70 §57),
// Versicherung 12. Wartung und Leasing haben keinen Vorschlag.
func DefaultIntervalMonths(art string) *int {
	switch art {
	case TerminHU:
		return intPtrRule(24)
	case TerminUVV, TerminVersicherung:
		return intPtrRule(12)
	}
	return nil
}

func intPtrRule(i int) *int { return &i }

// TerminLabel ist die Bezeichnung einer Art fuer Meldungen.
func TerminLabel(art, bezeichnung string) string {
	switch art {
	case TerminHU:
		return "Hauptuntersuchung"
	case TerminUVV:
		return "UVV-Prüfung"
	case TerminWartung:
		return "Wartung"
	case TerminVersicherung:
		return "Versicherung"
	case TerminLeasing:
		return "Leasingende"
	}
	return bezeichnung
}

// EndOfMonth liefert den letzten Tag des Monats von t (Kalendertag, UTC).
func EndOfMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC)
}

// NormalizeDue legt einen Faelligkeitstag fest: bei der HU das Monatsende
// (faellig ist der Monat der Plakette), sonst der Kalendertag.
func NormalizeDue(art string, d time.Time) time.Time {
	if art == TerminHU {
		return EndOfMonth(d)
	}
	return dateOnly(d)
}

// NextDue schlaegt den Folgetermin aus dem Erledigungsdatum und dem
// Intervall vor; nil ohne Intervall. Bei der HU zaehlt der Monat: HU im
// Maerz 2026 mit 24 Monaten -> faellig bis 31.03.2028.
func NextDue(art string, erledigtAm time.Time, intervallMonate *int) *time.Time {
	d := hr.QualificationValidUntil(dateOnly(erledigtAm), intervallMonate)
	if d == nil {
		return nil
	}
	n := NormalizeDue(art, *d)
	return &n
}

// ValidateScheduleInput prueft einen Termin. bezeichnung ist bereits
// getrimmt.
func ValidateScheduleInput(art, bezeichnung string, startKm, intervallMonate, intervallKm *int) error {
	if !terminArten[art] {
		return errors.New("Ungültige Terminart")
	}
	if art == TerminSonstige && bezeichnung == "" {
		return errors.New("Bezeichnung erforderlich")
	}
	if art != TerminSonstige && bezeichnung != "" {
		return errors.New("Bezeichnung nur bei sonstigen Terminen")
	}
	if startKm != nil && *startKm <= 0 {
		return errors.New("Kilometerstand muss größer als 0 sein")
	}
	if intervallMonate != nil && *intervallMonate <= 0 {
		return errors.New("Intervall muss größer als 0 sein")
	}
	if intervallKm != nil && *intervallKm <= 0 {
		return errors.New("Intervall muss größer als 0 sein")
	}
	return nil
}

// ValidateCompletion prueft eine Erledigung. naechster ist bereits mit
// NormalizeDue festgelegt. today wird uebergeben, damit die Regel
// testbar ist; verglichen werden Kalendertage.
func ValidateCompletion(art string, erledigtAm time.Time, ergebnis string, naechster *time.Time, kmStand, naechsterKm *int, erstzulassung *time.Time, today time.Time) error {
	if erledigtAm.IsZero() {
		return errors.New("Datum erforderlich")
	}
	// Erfasst wird, was erledigt IST.
	if dateOnly(erledigtAm).After(dateOnly(today)) {
		return errors.New("Datum darf nicht in der Zukunft liegen")
	}
	if erstzulassung != nil && dateOnly(erledigtAm).Before(dateOnly(*erstzulassung)) {
		return errors.New("Datum darf nicht vor der Erstzulassung liegen")
	}
	if pruefArten[art] {
		if ergebnis != ErgebnisBestanden && ergebnis != ErgebnisMaengel && ergebnis != ErgebnisNichtBestanden {
			return errors.New("Ungültiges Prüfergebnis")
		}
	} else if ergebnis != ErgebnisErledigt {
		return errors.New("Ungültiges Ergebnis (erwartet: erledigt)")
	}
	if naechster != nil && !dateOnly(*naechster).After(dateOnly(erledigtAm)) {
		return errors.New("Folgetermin muss nach dem Datum liegen")
	}
	if kmStand != nil && *kmStand < 0 {
		return errors.New("Kilometerstand darf nicht negativ sein")
	}
	if naechsterKm != nil {
		if *naechsterKm <= 0 {
			return errors.New("Kilometerstand muss größer als 0 sein")
		}
		if kmStand != nil && *naechsterKm <= *kmStand {
			return errors.New("Folgetermin nach Kilometern muss über dem Kilometerstand liegen")
		}
	}
	return nil
}

// Completion ist eine Erledigung (Spalten wie Migration 100).
type Completion struct {
	ID                uuid.UUID  `json:"id"`
	ScheduleID        uuid.UUID  `json:"schedule_id"`
	ErledigtAm        time.Time  `json:"erledigt_am"`
	Ergebnis          string     `json:"ergebnis"`
	KmStand           *int       `json:"km_stand"`
	NaechsterTermin   *time.Time `json:"naechster_termin"`
	NaechsterKm       *int       `json:"naechster_km"`
	Pruefer           string     `json:"pruefer"`
	Notiz             string     `json:"notiz"`
	Status            string     `json:"status"`
	StornoReason      string     `json:"storno_reason,omitempty"`
	StorniertAt       *time.Time `json:"storniert_at,omitempty"`
	StorniertByUserID *string    `json:"storniert_by_user_id,omitempty"`
	CreatedByUserID   *string    `json:"created_by_user_id"`
	CreatedAt         time.Time  `json:"created_at"`
}

// ScheduleStanding ist der Stand EINES Termins.
type ScheduleStanding struct {
	Status string `json:"status"`
	// FaelligAm/FaelligKm: was gerade gilt - der Folgetermin der juengsten
	// Erledigung, sonst der Starttermin.
	FaelligAm *time.Time `json:"faellig_am,omitempty"`
	FaelligKm *int       `json:"faellig_km,omitempty"`
	// TageBisFaellig: bis einschliesslich FaelligAm; negativ, wenn
	// ueberschritten.
	TageBisFaellig *int `json:"tage_bis_faellig,omitempty"`
	// Die juengste Erledigung, falls es eine gibt.
	LetzteErledigungID *uuid.UUID `json:"letzte_erledigung_id,omitempty"`
	LetzteErledigungAm *time.Time `json:"letzte_erledigung_am,omitempty"`
	LetztesErgebnis    string     `json:"letztes_ergebnis,omitempty"`
}

// EvaluateSchedule berechnet den Stand eines Termins am Tag today.
//
// Massgeblich ist die JUENGSTE nicht stornierte Erledigung (spaetestes
// Datum, bei Gleichstand die zuletzt erfasste) - wie bei den Pruefungen
// der Betriebsmittel: eine durchgefallene HU muss eine fruehere bestandene
// abloesen. Ohne Erledigung gelten Starttermin und Start-km.
//
// Eine Sperre (HU/UVV mit Maengeln oder nicht bestanden) gilt bis zur
// naechsten bestandenen Erledigung, unabhaengig vom Folgetermin; die
// Faelligkeit wird trotzdem angegeben (die Nachpruefung hat eine Frist).
func EvaluateSchedule(art string, starttermin *time.Time, startKm *int, completions []Completion, today time.Time, warnDays int) ScheduleStanding {
	if warnDays < 0 {
		warnDays = 0
	}
	today = dateOnly(today)

	var latest *Completion
	for i := range completions {
		c := &completions[i]
		if c.Status == CompletionStorniert {
			continue
		}
		if latest == nil || completionIsNewer(*c, *latest) {
			latest = c
		}
	}

	st := ScheduleStanding{FaelligAm: starttermin, FaelligKm: startKm}
	if latest != nil {
		st.FaelligAm, st.FaelligKm = latest.NaechsterTermin, latest.NaechsterKm
		st.LetzteErledigungID, st.LetzteErledigungAm, st.LetztesErgebnis = &latest.ID, &latest.ErledigtAm, latest.Ergebnis
	}
	if st.FaelligAm != nil {
		days := int(dateOnly(*st.FaelligAm).Sub(today).Hours() / 24)
		st.TageBisFaellig = &days
	}
	switch {
	case latest != nil && pruefArten[art] && (latest.Ergebnis == ErgebnisMaengel || latest.Ergebnis == ErgebnisNichtBestanden):
		st.Status = TerminstandGesperrt
	case st.TageBisFaellig == nil:
		st.Status = TerminstandOhneTermin
	case *st.TageBisFaellig < 0:
		st.Status = TerminstandUeberfaellig
	case *st.TageBisFaellig <= warnDays:
		st.Status = TerminstandFaelligBald
	default:
		st.Status = TerminstandOK
	}
	return st
}

func completionIsNewer(a, b Completion) bool {
	if !a.ErledigtAm.Equal(b.ErledigtAm) {
		return a.ErledigtAm.After(b.ErledigtAm)
	}
	return a.CreatedAt.After(b.CreatedAt)
}
