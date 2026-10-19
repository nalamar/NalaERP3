package hr

import (
	"math"
	"time"
)

// Teilurlaub im Ein- oder Austrittsjahr (§§ 4, 5 BUrlG; ADR 0029, Backlog
// F.8.3.2). Ergebnis ist ein VORSCHLAG fuer die Personalabteilung, die das
// Konto des Jahres festlegt - Tarif- und Arbeitsvertraege koennen
// abweichen, und fuer den vertraglichen Mehrurlaub gilt das Gesetz nicht
// zwingend.
//
// Regeln:
//   - Voller Anspruch, wenn die Wartezeit von sechs Monaten (§4) im Jahr
//     erfuellt wird und der Mitarbeiter nicht in der ersten Jahreshaelfte
//     ausscheidet - also Eintritt bis 1. Juli, Austritt ab 1. Juli.
//   - Sonst 1/12 des Jahresanspruchs je vollem Beschaeftigungsmonat im Jahr
//     (§5 Abs. 1).
//   - Bruchteile von mindestens einem halben Tag werden auf einen ganzen
//     Tag aufgerundet (§5 Abs. 2). Kleinere Bruchteile werden auf einen
//     halben Tag aufgerundet - zugunsten des Mitarbeiters, weil das Konto
//     nur halbe Tage kennt (eigene Festlegung, in ADR 0029 vermerkt).

// PartialLeave ist der Vorschlag fuer ein Jahr mit Ein- oder Austritt.
type PartialLeave struct {
	Tage   float64 `json:"tage"`
	Monate int     `json:"monate"`
	Grund  string  `json:"grund"`
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// monthsEnd ist der letzte Tag nach n Beschaeftigungsmonaten ab from
// (§§ 187 Abs. 2, 188 Abs. 2 und 3 BGB): der Tag vor dem entsprechenden
// Tag des spaeteren Monats; fehlt dieser Tag, der letzte Tag des Monats.
// Beispiel: ab 31.01. endet der erste Monat am 28.02., der zweite am 30.03.
func monthsEnd(from time.Time, n int) time.Time {
	from = dateOnly(from)
	target := time.Date(from.Year(), from.Month()+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
	if last := daysInMonth(target.Year(), target.Month()); from.Day() > last {
		return time.Date(target.Year(), target.Month(), last, 0, 0, 0, 0, time.UTC)
	}
	return time.Date(target.Year(), target.Month(), from.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
}

// fullMonths zaehlt die vollen Beschaeftigungsmonate von from bis to.
func fullMonths(from, to time.Time) int {
	to = dateOnly(to)
	n := 0
	for n < 12 && !monthsEnd(from, n+1).After(to) {
		n++
	}
	return n
}

// roundPartialLeave rundet nach §5 Abs. 2 BUrlG (siehe oben).
func roundPartialLeave(x float64) float64 {
	x = math.Round(x*1e6) / 1e6
	whole := math.Floor(x)
	switch frac := x - whole; {
	case frac >= 0.5:
		return whole + 1
	case frac > 0:
		return whole + 0.5
	default:
		return whole
	}
}

// PartialLeaveProposal liefert den Vorschlag fuer ein Jahr. nil, wenn der
// Mitarbeiter weder in diesem Jahr ein- noch austritt - dann gilt der
// Jahresanspruch ohne Einschraenkung.
func PartialLeaveProposal(year int, hire, termination *time.Time, annual float64) *PartialLeave {
	first, last := calendarDay(year, time.January, 1), calendarDay(year, time.December, 31)
	if (hire != nil && dateOnly(*hire).After(last)) || (termination != nil && dateOnly(*termination).Before(first)) {
		return &PartialLeave{Tage: 0, Monate: 0, Grund: "im Jahr nicht beschäftigt"}
	}
	hireIn := hire != nil && hire.Year() == year
	termIn := termination != nil && termination.Year() == year
	if !hireIn && !termIn {
		return nil
	}
	from, to := first, last
	if hireIn {
		from = dateOnly(*hire)
	}
	if termIn {
		to = dateOnly(*termination)
	}
	// Wartezeit: sechs Monate Bestand des Arbeitsverhaeltnisses bis zum
	// Ende der Beschaeftigung im Jahr (ohne Austritt: bis 31.12.).
	waited := hire == nil || !monthsEnd(*hire, 6).After(to)
	secondHalf := !termIn || !to.Before(calendarDay(year, time.July, 1))
	if waited && secondHalf {
		grund := "Eintritt bis 1. Juli: Wartezeit im Jahr erfüllt, voller Anspruch"
		if termIn {
			grund = "Austritt in der zweiten Jahreshälfte nach erfüllter Wartezeit: voller Anspruch"
		}
		return &PartialLeave{Tage: annual, Monate: 12, Grund: grund}
	}
	months := fullMonths(from, to)
	grund := "Teilurlaub: 1/12 je vollem Beschäftigungsmonat (§ 5 BUrlG)"
	return &PartialLeave{Tage: roundPartialLeave(annual * float64(months) / 12), Monate: months, Grund: grund}
}
