package production

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Fachregeln der Montagetermine ohne Datenbank (ADR 0038 Frage 6 (a),
// Backlog H.3.6a). Tabelle in Migration 115; Service, Hinweise, Routen und
// Client folgen in H.3.6b-f. Hinweise sind keine Sperren.

const (
	// MaxMontageTage: laengster Zeitraum eines Termins (Kalendertage von
	// bis einschliesslich), wie in Migration 115.
	MaxMontageTage     = 60
	MaxBaustelleLaenge = 300

	HintTeamDoppelt       = "team_doppelt"
	HintTeamAbwesend      = "team_abwesend"
	HintFahrzeugGesperrt  = "fahrzeug_gesperrt"
	HintVorFertigstellung = "vor_fertigstellung"
)

var (
	errMontageAuftrag   = errors.New("Fertigungsauftrag für den Montagetermin erforderlich")
	errMontageDatum     = errors.New("Zeitraum der Montage erforderlich (von und bis)")
	errMontageZeitraum  = errors.New("Ungültiger Zeitraum der Montage: bis liegt vor von")
	errMontageZuLang    = errors.New("Montage ist zu lang (höchstens 60 Tage)")
	errMontageBaustelle = errors.New("Baustelle ist zu lang (höchstens 300 Zeichen)")
	errMontageNotiz     = errors.New("Notiz ist zu lang (höchstens 500 Zeichen)")
	errMontageStatus    = errors.New("Montagetermin für stornierte Aufträge nicht möglich")
)

// AppointmentInput: Montagetermin zu einem Fertigungsauftrag; das Projekt
// uebernimmt der Service vom Auftrag. Team und Fahrzeug optional.
type AppointmentInput struct {
	OrderID   *uuid.UUID `json:"order_id"`
	Von       *time.Time `json:"von"`
	Bis       *time.Time `json:"bis"`
	TeamID    *uuid.UUID `json:"team_id"`
	VehicleID *uuid.UUID `json:"vehicle_id"`
	Baustelle string     `json:"baustelle"`
	Notiz     string     `json:"notiz"`
}

// Normalize entfernt Leerraum und kuerzt die Tage auf das Datum.
func (in AppointmentInput) Normalize() AppointmentInput {
	in.Baustelle = strings.TrimSpace(in.Baustelle)
	in.Notiz = strings.TrimSpace(in.Notiz)
	for _, p := range []**time.Time{&in.Von, &in.Bis} {
		if *p != nil {
			d := dateOnly(**p)
			*p = &d
		}
	}
	return in
}

// Validate prueft eine normalisierte Eingabe. In (a) ist der Auftrag
// Pflicht; fuer (b) entfaellt nur diese Pruefung.
func (in AppointmentInput) Validate() error {
	if in.OrderID == nil || *in.OrderID == uuid.Nil {
		return errMontageAuftrag
	}
	if in.Von == nil || in.Bis == nil {
		return errMontageDatum
	}
	if in.Bis.Before(*in.Von) {
		return errMontageZeitraum
	}
	if AssemblyDays(*in.Von, *in.Bis) > MaxMontageTage {
		return errMontageZuLang
	}
	if utf8.RuneCountInString(in.Baustelle) > MaxBaustelleLaenge {
		return errMontageBaustelle
	}
	if utf8.RuneCountInString(in.Notiz) > MaxNotizLaenge {
		return errMontageNotiz
	}
	return nil
}

// AssemblyDays: Kalendertage von bis bis einschliesslich.
func AssemblyDays(von, bis time.Time) int {
	return int(dateOnly(bis).Sub(dateOnly(von)).Hours()/24) + 1
}

// AssemblyAllowed: Termine fuer jeden Auftrag ausser stornierten - auch
// im Entwurf (vorausplanen) und nach "fertig" (der Normalfall).
func AssemblyAllowed(status string) error {
	if status == StatusStorniert {
		return errMontageStatus
	}
	return nil
}

// ValidateAssemblyCancelReason: Storno eines Termins nur mit Begruendung.
func ValidateAssemblyCancelReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", errors.New("Storno des Montagetermins nur mit Begründung")
	}
	if utf8.RuneCountInString(reason) > MaxNotizLaenge {
		return "", errors.New("Begründung ist zu lang (höchstens 500 Zeichen)")
	}
	return reason, nil
}

// DateRangesOverlap: zwei Zeitraeume (Tage, je einschliesslich) teilen
// mindestens einen Tag.
func DateRangesOverlap(aVon, aBis, bVon, bBis time.Time) bool {
	return !dateOnly(aBis).Before(dateOnly(bVon)) && !dateOnly(bBis).Before(dateOnly(aVon))
}

// AssemblyAfterDelivery: die Montage endet nach dem Liefertermin.
func AssemblyAfterDelivery(bis time.Time, liefertermin *time.Time) bool {
	return liefertermin != nil && dateOnly(bis).After(dateOnly(*liefertermin))
}

// AssemblyBeforeCompletion: der Auftrag ist bei Montagebeginn
// voraussichtlich nicht fertig - er ist weder fertig noch storniert und
// die Montage beginnt heute oder frueher, oder ein offener Arbeitsgang ist
// in der Woche des Montagebeginns oder spaeter geplant (lastOpenWeek).
func AssemblyBeforeCompletion(status string, von time.Time, lastOpenWeek *time.Time, today time.Time) bool {
	if status == StatusFertig || status == StatusStorniert {
		return false
	}
	if !dateOnly(von).After(dateOnly(today)) {
		return true
	}
	return lastOpenWeek != nil && !WeekStart(*lastOpenWeek).Before(WeekStart(von))
}

// AssemblyFacts: was der Service fuer die Hinweise eines Termins ermittelt.
type AssemblyFacts struct {
	TeamDoppelt       bool
	TeamAbwesend      bool
	FahrzeugGesperrt  bool
	VorFertigstellung bool
	NachLiefertermin  bool
}

// AssemblyHints: Hinweise in fester Reihenfolge, leer statt nil.
func AssemblyHints(f AssemblyFacts) []string {
	out := []string{}
	for _, h := range []struct {
		on   bool
		code string
	}{
		{f.TeamDoppelt, HintTeamDoppelt},
		{f.TeamAbwesend, HintTeamAbwesend},
		{f.FahrzeugGesperrt, HintFahrzeugGesperrt},
		{f.VorFertigstellung, HintVorFertigstellung},
		{f.NachLiefertermin, HintNachLiefertermin},
	} {
		if h.on {
			out = append(out, h.code)
		}
	}
	return out
}
