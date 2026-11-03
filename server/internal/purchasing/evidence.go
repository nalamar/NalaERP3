package purchasing

import (
	"errors"
	"slices"
	"strings"
	"time"
)

// Fachregeln der Nachweise der Zulieferer nach EN 1090 ohne Datenbank
// (ADR 0035, Backlog H.4.1b). Tabellen in Migration 110; Service und
// Routen folgen in H.4.2/H.4.3. Das ERP dokumentiert, es bewertet nicht:
// alle Regeln liefern Hinweise, keine Sperren (ADR 0035 §3).

const (
	ZertifikatEN1090    = "en1090"
	ZertifikatISO3834   = "iso3834"
	ZertifikatSonstiges = "sonstiges"

	PapierLeistungserklaerung = "leistungserklaerung"
	PapierZeugnis22           = "zeugnis_2_2"
	PapierZeugnis31           = "zeugnis_3_1"
	PapierZeugnis32           = "zeugnis_3_2"
	PapierLieferschein        = "lieferschein"
	PapierSonstiges           = "sonstiges"

	// Vorlauf des Hinweises am Lieferanten (ADR 0035 §3).
	AblaufVorlaufTage = 60
)

// Hinweise an Bestellung und Lieferant.
const (
	// Nachweispflichtiges Material, aber kein (nicht storniertes)
	// EN-1090-Zertifikat am Lieferanten.
	HintZertifikatFehlt = "zertifikat_fehlt"
	// Es gibt ein EN-1090-Zertifikat, aber keines gilt vom Bestelldatum
	// bis zum Stichtag (abgelaufen oder noch nicht gueltig).
	HintZertifikatUngueltig = "zertifikat_ungueltig"
	// Bestellung erhalten, eine nachweispflichtige Position hat keine
	// Leistungserklaerung.
	HintLeistungserklaerungFehlt = "leistungserklaerung_fehlt"
	// Am Lieferanten: ein Zertifikat endet in den naechsten 60 Tagen und
	// kein anderes derselben Art gilt darueber hinaus.
	HintZertifikatLaeuftAb = "zertifikat_laeuft_ab"
)

var (
	ZertifikatArten = []string{ZertifikatEN1090, ZertifikatISO3834, ZertifikatSonstiges}
	PapierArten     = []string{PapierLeistungserklaerung, PapierZeugnis22, PapierZeugnis31, PapierZeugnis32, PapierLieferschein, PapierSonstiges}
	EXCKlassen      = []string{"EXC1", "EXC2", "EXC3", "EXC4"}
)

// Certificate ist das, was die Regeln vom Zertifikat brauchen.
// GueltigVon/GueltigBis leer = ohne Anfang/Ende; GueltigBis gilt
// einschliesslich (Migration 110).
type Certificate struct {
	Art        string
	ExcBis     *string
	GueltigVon *time.Time
	GueltigBis *time.Time
	Storniert  bool
}

func day(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// NormalizeCertificate prueft und bereinigt die Eingabe eines Zertifikats
// mit denselben Regeln wie die Datenbank, aber mit lesbarer Meldung.
func NormalizeCertificate(c *Certificate) error {
	c.Art = strings.ToLower(strings.TrimSpace(c.Art))
	if !slices.Contains(ZertifikatArten, c.Art) {
		return errors.New("Ungültige Art des Zertifikats")
	}
	if c.ExcBis != nil {
		v := strings.ToUpper(strings.TrimSpace(*c.ExcBis))
		c.ExcBis = &v
		if v == "" {
			c.ExcBis = nil
		}
	}
	if c.Art == ZertifikatEN1090 {
		if c.ExcBis == nil {
			return errors.New("Ausführungsklasse erforderlich")
		}
		if !slices.Contains(EXCKlassen, *c.ExcBis) {
			return errors.New("Ungültige Ausführungsklasse")
		}
	} else if c.ExcBis != nil {
		return errors.New("Ungültige Ausführungsklasse: nur bei EN 1090")
	}
	if c.GueltigVon != nil && c.GueltigBis != nil && day(*c.GueltigBis).Before(day(*c.GueltigVon)) {
		return errors.New("Ungültiger Zeitraum: gültig bis liegt vor gültig von")
	}
	return nil
}

// ValidPapierArt sagt, ob die Art eines Lieferpapiers bekannt ist.
func ValidPapierArt(art string) bool {
	return slices.Contains(PapierArten, strings.ToLower(strings.TrimSpace(art)))
}

// Covers sagt, ob das Zertifikat vom Tag from bis einschliesslich to
// gilt. Stornierte gelten nie.
func (c Certificate) Covers(from, to time.Time) bool {
	if c.Storniert {
		return false
	}
	if c.GueltigVon != nil && day(*c.GueltigVon).After(day(from)) {
		return false
	}
	return c.GueltigBis == nil || !day(*c.GueltigBis).Before(day(to))
}

// OrderStichtag ist der spaeteste Liefertermin der Positionen, ohne
// Liefertermin (oder davor) das Bestelldatum: bis dahin muss das
// Zertifikat gelten (ADR 0035 §3).
func OrderStichtag(orderDate time.Time, deliveryDates []*time.Time) time.Time {
	out := day(orderDate)
	for _, d := range deliveryDates {
		if d != nil && day(*d).After(out) {
			out = day(*d)
		}
	}
	return out
}

// OrderCertificateHint liefert den Hinweis zum Zertifikat einer Bestellung
// oder "". needsEvidence: mindestens eine Position mit nachweispflichtigem
// Material. Nur EN-1090-Zertifikate zaehlen.
func OrderCertificateHint(needsEvidence bool, certs []Certificate, orderDate, stichtag time.Time) string {
	if !needsEvidence {
		return ""
	}
	found := false
	for _, c := range certs {
		if c.Art != ZertifikatEN1090 || c.Storniert {
			continue
		}
		found = true
		if c.Covers(orderDate, stichtag) {
			return ""
		}
	}
	if !found {
		return HintZertifikatFehlt
	}
	return HintZertifikatUngueltig
}

// ExpiringCertificates liefert die Indizes der Zertifikate, die zwischen
// today und today+60 Tagen (einschliesslich) enden, ohne dass ein anderes
// nicht storniertes derselben Art ueber das Ende hinaus gilt. Bereits
// abgelaufene zaehlen nicht.
func ExpiringCertificates(certs []Certificate, today time.Time) []int {
	today = day(today)
	limit := today.AddDate(0, 0, AblaufVorlaufTage)
	var out []int
	for i, c := range certs {
		if c.Storniert || c.GueltigBis == nil {
			continue
		}
		end := day(*c.GueltigBis)
		if end.Before(today) || end.After(limit) {
			continue
		}
		next := end.AddDate(0, 0, 1)
		followed := false
		for j, o := range certs {
			if j != i && o.Art == c.Art && o.Covers(next, next) {
				followed = true
				break
			}
		}
		if !followed {
			out = append(out, i)
		}
	}
	return out
}

// EvidenceItem ist eine Bestellposition fuer die Pruefung der
// Lieferpapiere.
type EvidenceItem struct {
	ID       string
	Nachweis bool
}

// DeliveryPaper ist ein Lieferpapier der Bestellung; ItemID leer = gilt
// fuer die ganze Bestellung.
type DeliveryPaper struct {
	ItemID    *string
	Art       string
	Storniert bool
}

// MissingDeclarations liefert die IDs der nachweispflichtigen Positionen
// ohne Leistungserklaerung - nur im Status received (vorher ist nichts
// geliefert). Eine Leistungserklaerung ohne Position gilt fuer alle.
func MissingDeclarations(status string, items []EvidenceItem, papers []DeliveryPaper) []string {
	if strings.ToLower(strings.TrimSpace(status)) != "received" {
		return nil
	}
	covered := map[string]bool{}
	all := false
	for _, p := range papers {
		if p.Storniert || p.Art != PapierLeistungserklaerung {
			continue
		}
		if p.ItemID == nil || strings.TrimSpace(*p.ItemID) == "" {
			all = true
		} else {
			covered[*p.ItemID] = true
		}
	}
	var out []string
	for _, it := range items {
		if it.Nachweis && !all && !covered[it.ID] {
			out = append(out, it.ID)
		}
	}
	return out
}

// OrderEvidenceHints fasst die Hinweise einer Bestellung zusammen; eine
// stornierte Bestellung hat keine.
func OrderEvidenceHints(status string, items []EvidenceItem, papers []DeliveryPaper, certs []Certificate, orderDate time.Time, deliveryDates []*time.Time) []string {
	if strings.ToLower(strings.TrimSpace(status)) == "canceled" {
		return nil
	}
	needs := false
	for _, it := range items {
		needs = needs || it.Nachweis
	}
	var out []string
	if h := OrderCertificateHint(needs, certs, orderDate, OrderStichtag(orderDate, deliveryDates)); h != "" {
		out = append(out, h)
	}
	if len(MissingDeclarations(status, items, papers)) > 0 {
		out = append(out, HintLeistungserklaerungFehlt)
	}
	return out
}
