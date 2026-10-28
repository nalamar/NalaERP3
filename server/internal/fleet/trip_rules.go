package fleet

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Regeln des Fahrtenbuchs ohne Datenbank (ADR 0032 §6, Backlog G.3c.1).
//
// Geschlossene Form: Eintraege werden nicht geaendert; ein Fehler wird
// storniert und neu erfasst. Der Kilometerstand ist lueckenlos: eine neue
// Fahrt beginnt am Ende ihrer Vorgaengerin. Damit ein stornierter Eintrag
// ersetzt werden kann, darf eine Fahrt auch in eine Luecke fallen - sie
// muss dann am Ende der Vorgaengerin beginnen und darf die Nachfolgerin
// nicht ueberlappen; eine verbleibende Luecke wird sichtbar (TripGaps).

const (
	FahrtartDienstlich = "dienstlich"
	FahrtartPrivat     = "privat"
	FahrtartArbeitsweg = "arbeitsweg"
)

var fahrtarten = map[string]bool{FahrtartDienstlich: true, FahrtartPrivat: true, FahrtartArbeitsweg: true}

const (
	TripActive    = "active"
	TripStorniert = "storniert"
)

// TripInput ist eine neue Fahrt.
type TripInput struct {
	VehicleID         uuid.UUID `json:"vehicle_id"`
	EmployeeID        uuid.UUID `json:"employee_id"`
	Datum             time.Time `json:"datum"`
	KmBeginn          int       `json:"km_beginn"`
	KmEnde            int       `json:"km_ende"`
	Fahrtart          string    `json:"fahrtart"`
	Ziel              string    `json:"ziel"`
	Zweck             string    `json:"zweck"`
	Geschaeftspartner string    `json:"geschaeftspartner"`
	Notiz             string    `json:"notiz"`
}

// NormalizeTripInput entfernt Leerraum am Rand; die Fahrtart wird klein
// geschrieben.
func NormalizeTripInput(in TripInput) TripInput {
	in.Fahrtart = strings.ToLower(strings.TrimSpace(in.Fahrtart))
	in.Ziel = strings.TrimSpace(in.Ziel)
	in.Zweck = strings.TrimSpace(in.Zweck)
	in.Geschaeftspartner = strings.TrimSpace(in.Geschaeftspartner)
	in.Notiz = strings.TrimSpace(in.Notiz)
	return in
}

// ValidateTripInput prueft eine (normalisierte) Fahrt fuer sich: Datum
// nicht in der Zukunft, nicht vor der Erstzulassung, nicht nach der
// Abmeldung; Kilometer; bei Dienstfahrten Reiseziel und Reisezweck.
func ValidateTripInput(in TripInput, erstzulassung, abgemeldetAm *time.Time, today time.Time) error {
	if in.Datum.IsZero() {
		return errors.New("Datum erforderlich")
	}
	d := dateOnly(in.Datum)
	if d.After(dateOnly(today)) {
		return errors.New("Datum darf nicht in der Zukunft liegen")
	}
	if erstzulassung != nil && d.Before(dateOnly(*erstzulassung)) {
		return errors.New("Datum darf nicht vor der Erstzulassung liegen")
	}
	if abgemeldetAm != nil && d.After(dateOnly(*abgemeldetAm)) {
		return errors.New("Datum darf nicht nach der Abmeldung liegen")
	}
	if in.KmBeginn < 0 {
		return errors.New("Kilometerstand Beginn darf nicht negativ sein")
	}
	if in.KmEnde <= in.KmBeginn {
		return errors.New("Kilometerstand Ende muss über dem Beginn liegen")
	}
	if !fahrtarten[in.Fahrtart] {
		return errors.New("Ungültige Fahrtart")
	}
	if in.Fahrtart == FahrtartDienstlich {
		if in.Ziel == "" {
			return errors.New("Reiseziel ist bei Dienstfahrten erforderlich")
		}
		if in.Zweck == "" {
			return errors.New("Reisezweck ist bei Dienstfahrten erforderlich")
		}
	}
	return nil
}

// TripSpan ist der Ausschnitt einer nicht stornierten Fahrt, den die
// Kette braucht.
type TripSpan struct {
	Datum    time.Time
	KmBeginn int
	KmEnde   int
}

// CheckTripChain prueft eine neue Fahrt gegen die nicht stornierten
// Fahrten desselben Fahrzeugs.
//
//   - Kein Ueberlappen der Kilometerbereiche.
//   - Gibt es eine Vorgaengerin (Ende <= neuer Beginn), beginnt die neue
//     Fahrt genau an deren Ende.
//   - Gibt es keine, liegt die neue Fahrt vor allen anderen: sie endet
//     dann genau am Beginn der ersten (die Kette bleibt verbunden).
//   - Das Datum passt zur Reihenfolge: nicht vor der Vorgaengerin, nicht
//     nach der Nachfolgerin.
func CheckTripChain(existing []TripSpan, n TripSpan) error {
	var pred, succ *TripSpan
	for i := range existing {
		e := &existing[i]
		if e.KmBeginn < n.KmEnde && e.KmEnde > n.KmBeginn {
			return fmt.Errorf("Kilometerbereich überschneidet sich mit der Fahrt vom %s (%d–%d km)", e.Datum.Format("02.01.2006"), e.KmBeginn, e.KmEnde)
		}
		if e.KmEnde <= n.KmBeginn && (pred == nil || e.KmEnde > pred.KmEnde) {
			pred = e
		}
		if e.KmBeginn >= n.KmEnde && (succ == nil || e.KmBeginn < succ.KmBeginn) {
			succ = e
		}
	}
	if pred != nil && pred.KmEnde != n.KmBeginn {
		return fmt.Errorf("Kilometerstand Beginn muss %d km sein (Ende der vorigen Fahrt)", pred.KmEnde)
	}
	if pred == nil && succ != nil && succ.KmBeginn != n.KmEnde {
		return fmt.Errorf("Kilometerstand Ende muss %d km sein (Beginn der folgenden Fahrt)", succ.KmBeginn)
	}
	if pred != nil && dateOnly(n.Datum).Before(dateOnly(pred.Datum)) {
		return fmt.Errorf("Datum darf nicht vor der vorigen Fahrt (%s) liegen", pred.Datum.Format("02.01.2006"))
	}
	if succ != nil && dateOnly(n.Datum).After(dateOnly(succ.Datum)) {
		return fmt.Errorf("Datum darf nicht nach der folgenden Fahrt (%s) liegen", succ.Datum.Format("02.01.2006"))
	}
	return nil
}

// TripGap ist eine Luecke im Kilometerstand zwischen zwei Fahrten.
type TripGap struct {
	VonKm int `json:"von_km"`
	BisKm int `json:"bis_km"`
}

// TripGaps nennt die Luecken zwischen den nicht stornierten Fahrten (nach
// Kilometern geordnet) - entstanden durch ein Storno ohne Ersatz.
func TripGaps(spans []TripSpan) []TripGap {
	sorted := append([]TripSpan(nil), spans...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].KmBeginn < sorted[j].KmBeginn })
	gaps := make([]TripGap, 0)
	for i := 1; i < len(sorted); i++ {
		if sorted[i].KmBeginn > sorted[i-1].KmEnde {
			gaps = append(gaps, TripGap{VonKm: sorted[i-1].KmEnde, BisKm: sorted[i].KmBeginn})
		}
	}
	return gaps
}
