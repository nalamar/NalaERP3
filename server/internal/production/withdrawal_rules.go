package production

import (
	"errors"
	"math"
	"strings"
	"unicode/utf8"
)

// Fachregeln der Entnahme ohne Datenbank (ADR 0038 Frage 2/3, Backlog
// H.3.1). Tabelle in Migration 113; Service und Routen folgen in H.3.2.

const (
	// Obergrenze gegen Tippfehler, wie die Spalte numeric(18,6) erlaubt
	// es weit mehr.
	MaxEntnahmeMenge = 1_000_000
	MaxNotizLaenge   = 500

	// Grund der Lagerbewegungen; Referenz ist die Auftragsnummer.
	EntnahmeGrund       = "Entnahme Fertigungsauftrag"
	EntnahmeStornoGrund = "Storno Entnahme Fertigungsauftrag"
	// Bewegungsart der Lagerbewegung (stock_movements.movement_type):
	// Abgang, die Gegenbuchung des Stornos ist ein Zugang.
	EntnahmeBewegung       = "out"
	EntnahmeStornoBewegung = "in"

	// Hinweise - keine Sperren.
	HintMehrentnahme   = "mehrentnahme"
	HintNichtEntnommen = "nicht_entnommen"

	ReservierungAktiv       = "aktiv"
	ReservierungFreigegeben = "freigegeben"
	ReservierungVerbraucht  = "verbraucht"
)

var (
	errEntnahmeStatus = errors.New("Entnahme nur bei freigegebenen oder begonnenen Aufträgen möglich")
	errEntnahmeMenge  = errors.New("Ungültige Menge (größer als 0, höchstens 1.000.000)")
	errEntnahmeNotiz  = errors.New("Notiz ist zu lang (höchstens 500 Zeichen)")
	errEntnahmeZiel   = errors.New("Bitte eine Zeile der Stückliste oder ein Material angeben")
)

// WithdrawalInput: Zeile der Stueckliste ODER Material ohne Zeile; das
// Lager ist Pflicht (vorbelegt im Client mit dem Lager der Reservierung).
type WithdrawalInput struct {
	ItemID      string  `json:"item_id"`
	MaterialID  string  `json:"material_id"`
	WarehouseID string  `json:"warehouse_id"`
	LocationID  string  `json:"location_id"`
	Menge       float64 `json:"menge"`
	Notiz       string  `json:"notiz"`
}

func (in WithdrawalInput) Normalize() WithdrawalInput {
	in.ItemID = strings.TrimSpace(in.ItemID)
	in.MaterialID = strings.TrimSpace(in.MaterialID)
	in.WarehouseID = strings.TrimSpace(in.WarehouseID)
	in.LocationID = strings.TrimSpace(in.LocationID)
	in.Notiz = strings.TrimSpace(in.Notiz)
	in.Menge = round6(in.Menge)
	return in
}

// Validate prueft eine normalisierte Eingabe. Mit Zeile kommt das
// Material aus der Zeile; ein abweichendes Material lehnt der Service ab.
func (in WithdrawalInput) Validate() error {
	if in.ItemID == "" && in.MaterialID == "" {
		return errEntnahmeZiel
	}
	if in.WarehouseID == "" {
		return errors.New("Bitte ein Lager angeben")
	}
	if math.IsNaN(in.Menge) || in.Menge <= 0 || in.Menge > MaxEntnahmeMenge {
		return errEntnahmeMenge
	}
	if utf8.RuneCountInString(in.Notiz) > MaxNotizLaenge {
		return errEntnahmeNotiz
	}
	return nil
}

// WithdrawalAllowed: entnommen wird ab der Freigabe bis zum Fertigmelden.
// Das Storno einer Entnahme ist immer moeglich (Material kommt zurueck).
func WithdrawalAllowed(status string) error {
	if status == StatusFreigegeben || status == StatusInArbeit {
		return nil
	}
	return errEntnahmeStatus
}

func round6(v float64) float64 { return math.Round(v*1e6) / 1e6 }

// OpenQty: Bedarf der Zeile minus bisher Entnommenes (ohne Stornos), nie
// negativ. Ohne umrechenbaren Bedarf (ReservationQty ok=false) gibt es
// keine offene Menge: known=false.
func OpenQty(bedarf float64, known bool, entnommen float64) (float64, bool) {
	if !known {
		return 0, false
	}
	return Shortfall(bedarf, entnommen), true
}

// Overdrawn: die Entnahme fuehrt ueber den bekannten Bedarf hinaus
// (Hinweis, keine Sperre). Rundungsrauschen zaehlt nicht.
func Overdrawn(bedarf float64, known bool, entnommen, menge float64) bool {
	return known && round6(entnommen+menge-bedarf) > 0
}

// ReservationConsumption: was eine Entnahme von einer Reservierung
// verbraucht. Nur eine aktive Reservierung wird verbraucht; bleibt nichts
// uebrig, wird sie "verbraucht" und behaelt qty, sonst sinkt qty.
func ReservationConsumption(status string, qty, menge float64) (verbraucht, restQty float64, newStatus string) {
	if status != ReservierungAktiv || qty <= 0 {
		return 0, qty, status
	}
	verbraucht = round6(math.Min(qty, menge))
	rest := round6(qty - verbraucht)
	if rest <= 0 {
		return verbraucht, qty, ReservierungVerbraucht
	}
	return verbraucht, rest, ReservierungAktiv
}

// ReservationAfterStorno: das Storno einer Entnahme stellt den
// verbrauchten Teil wieder her - nur bei offenem Auftrag und nicht bei
// einer freigegebenen Reservierung (Storno oder Fertigmeldung des
// Auftrags haben sie bewusst geloest). restore=false: nichts zu tun.
func ReservationAfterStorno(orderStatus, status string, qty, verbraucht float64) (newQty float64, newStatus string, restore bool) {
	if verbraucht <= 0 || WithdrawalAllowed(orderStatus) != nil {
		return qty, status, false
	}
	switch status {
	case ReservierungVerbraucht:
		// qty ist der zuletzt verbrauchte Rest; uebrig war nichts.
		return round6(verbraucht), ReservierungAktiv, true
	case ReservierungAktiv:
		return round6(qty + verbraucht), ReservierungAktiv, true
	}
	return qty, status, false
}

// ValidateWithdrawalStornoReason: Storno nur mit Begruendung.
func ValidateWithdrawalStornoReason(reason string) (string, error) {
	r := strings.TrimSpace(reason)
	if r == "" {
		return "", errors.New("Storno der Entnahme nur mit Begründung")
	}
	if utf8.RuneCountInString(r) > MaxNotizLaenge {
		return "", errEntnahmeNotiz
	}
	return r, nil
}
