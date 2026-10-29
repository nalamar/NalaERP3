package fleet

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Fachregeln der Fahrzeugkosten (ADR 0032 §7, Backlog G.3d.1). Der
// Service steht in costs.go.

const (
	KostenartKraftstoff   = "kraftstoff"
	KostenartWartung      = "wartung"
	KostenartReparatur    = "reparatur"
	KostenartVersicherung = "versicherung"
	KostenartSteuer       = "steuer"
	KostenartLeasing      = "leasing"
	KostenartSonstiges    = "sonstiges"

	CostActive    = "active"
	CostStorniert = "storniert"
)

// Kostenarten in der Reihenfolge der Auswertung.
var Kostenarten = []string{KostenartKraftstoff, KostenartWartung, KostenartReparatur, KostenartVersicherung,
	KostenartSteuer, KostenartLeasing, KostenartSonstiges}

type CostInput struct {
	VehicleID   uuid.UUID `json:"vehicle_id"`
	Datum       time.Time `json:"datum"`
	Kostenart   string    `json:"kostenart"`
	Bezeichnung string    `json:"bezeichnung"`
	BetragNetto float64   `json:"betrag_netto"`
	Lieferant   string    `json:"lieferant"`
	Belegnummer string    `json:"belegnummer"`
	KmStand     *int      `json:"km_stand"`
	Liter       *float64  `json:"liter"`
	// Ohne Angabe die Kostenstelle des Fahrzeugs.
	KostenstelleID *string `json:"kostenstelle_id"`
	Notiz          string  `json:"notiz"`
}

// NormalizeCostInput entfernt Leerraum am Rand, schreibt die Kostenart
// klein und macht eine leere Kostenstelle zu "keine Angabe".
func NormalizeCostInput(in CostInput) CostInput {
	in.Kostenart = strings.ToLower(strings.TrimSpace(in.Kostenart))
	in.Bezeichnung = strings.TrimSpace(in.Bezeichnung)
	in.Lieferant = strings.TrimSpace(in.Lieferant)
	in.Belegnummer = strings.TrimSpace(in.Belegnummer)
	in.Notiz = strings.TrimSpace(in.Notiz)
	if in.KostenstelleID != nil {
		k := strings.TrimSpace(*in.KostenstelleID)
		if k == "" {
			in.KostenstelleID = nil
		} else {
			in.KostenstelleID = &k
		}
	}
	if !in.Datum.IsZero() {
		in.Datum = dateOnly(in.Datum)
	}
	return in
}

func validKostenart(k string) bool {
	for _, x := range Kostenarten {
		if x == k {
			return true
		}
	}
	return false
}

// hasAtMostTwoDecimals: Betraege in Cent; 12.345 waere ein Tippfehler,
// den numeric(18,2) still runden wuerde.
func hasAtMostTwoDecimals(v float64) bool {
	c := v * 100
	return math.Abs(c-math.Round(c)) < 1e-6
}

// ValidateCostInput prueft eine normalisierte Eingabe ohne Datenbank.
// Ein Beleg darf vor der Erstzulassung (Leasing, Versicherung) und nach
// der Abmeldung (Schlussrechnung) liegen, aber nicht in der Zukunft.
func ValidateCostInput(in CostInput, today time.Time) error {
	if in.Datum.IsZero() {
		return errors.New("Datum erforderlich")
	}
	if in.Datum.After(dateOnly(today)) {
		return errors.New("Datum darf nicht in der Zukunft liegen")
	}
	if !validKostenart(in.Kostenart) {
		return errors.New("Ungültige Kostenart")
	}
	if in.Kostenart == KostenartSonstiges && in.Bezeichnung == "" {
		return errors.New("Bezeichnung ist bei sonstigen Kosten erforderlich")
	}
	if math.IsNaN(in.BetragNetto) || math.IsInf(in.BetragNetto, 0) || in.BetragNetto <= 0 {
		return errors.New("Betrag muss größer als 0 sein")
	}
	if !hasAtMostTwoDecimals(in.BetragNetto) {
		return errors.New("Betrag darf höchstens zwei Nachkommastellen haben")
	}
	if in.KmStand != nil && *in.KmStand < 0 {
		return errors.New("Kilometerstand darf nicht negativ sein")
	}
	if in.Liter != nil {
		if in.Kostenart != KostenartKraftstoff {
			return errors.New("Liter nur bei Kraftstoff")
		}
		if math.IsNaN(*in.Liter) || math.IsInf(*in.Liter, 0) || *in.Liter <= 0 {
			return errors.New("Liter muss größer als 0 sein")
		}
		if !hasAtMostTwoDecimals(*in.Liter) {
			return errors.New("Liter darf höchstens zwei Nachkommastellen haben")
		}
	}
	return nil
}
