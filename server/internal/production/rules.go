package production

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Fachregeln der Fertigungsauftraege ohne Datenbank (ADR 0034, Backlog
// H.1.1). Tabellen in Migration 107; der Service folgt in H.1.2.

const (
	StatusEntwurf     = "entwurf"
	StatusFreigegeben = "freigegeben"
	StatusInArbeit    = "in_arbeit"
	StatusFertig      = "fertig"
	StatusStorniert   = "storniert"

	ArtProfil  = "profil"
	ArtArtikel = "artikel"
	ArtGlas    = "glas"

	// Obergrenze gegen Tippfehler; ein Los ueber 10000 Stueck ist im
	// Fenster- und Fassadenbau nicht realistisch.
	MaxStueckzahl = 10000
)

// Hinweise am Auftrag - keine Sperren (ADR 0034 §1, §2, §4).
const (
	HintKeinKundenauftrag   = "kein_kundenauftrag"
	HintMengeUeberschritten = "menge_ueberschritten"
	HintPositionGeaendert   = "position_geaendert"
	HintPositionEntfernt    = "position_entfernt"
	// Bei der Freigabe war nicht alles verfuegbar (H.1.5).
	HintFehlmenge = "fehlmenge"

	// Grund der Reservierungen eines Auftrags; Referenz ist die Nummer.
	ReservierungGrund = "Fertigungsauftrag"
)

var Ausfuehrungsklassen = []string{"EXC1", "EXC2", "EXC3", "EXC4"}

// Aktionen und ihr Uebergang; der Status laeuft nur vorwaerts.
const (
	AktionFreigeben    = "freigeben"
	AktionBeginnen     = "beginnen"
	AktionFertigmelden = "fertigmelden"
	AktionStornieren   = "stornieren"
)

var transitions = map[string]struct{ from, to string }{
	AktionFreigeben:    {StatusEntwurf, StatusFreigegeben},
	AktionBeginnen:     {StatusFreigegeben, StatusInArbeit},
	AktionFertigmelden: {StatusInArbeit, StatusFertig},
}

// NextStatus liefert den Status nach einer Aktion oder einen Fehler,
// wenn die Aktion im aktuellen Status nicht erlaubt ist. Storniert wird
// aus jedem Status vor "fertig".
func NextStatus(current, aktion string) (string, error) {
	if aktion == AktionStornieren {
		switch current {
		case StatusEntwurf, StatusFreigegeben, StatusInArbeit:
			return StatusStorniert, nil
		case StatusFertig:
			return "", errors.New("Ein fertiger Auftrag kann nicht storniert werden")
		case StatusStorniert:
			return "", errors.New("Auftrag ist bereits storniert")
		}
		return "", errors.New("Unbekannter Status")
	}
	t, ok := transitions[aktion]
	if !ok {
		return "", errors.New("Unbekannte Aktion")
	}
	if current != t.from {
		return "", errors.New("Aktion im aktuellen Status nicht erlaubt")
	}
	return t.to, nil
}

type OrderInput struct {
	ProjectID         uuid.UUID  `json:"project_id"`
	ElevationID       uuid.UUID  `json:"elevation_id"`
	SingleElevationID uuid.UUID  `json:"single_elevation_id"`
	Stueckzahl        int        `json:"stueckzahl"`
	Liefertermin      *time.Time `json:"liefertermin"`
	// Leer heisst: keine Angabe.
	Ausfuehrungsklasse *string `json:"ausfuehrungsklasse"`
	Notiz              string  `json:"notiz"`
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// NormalizeOrderInput entfernt Leerraum, schreibt die Ausfuehrungsklasse
// gross ("exc2" -> "EXC2") und kuerzt den Liefertermin auf den Tag.
func NormalizeOrderInput(in OrderInput) OrderInput {
	in.Notiz = strings.TrimSpace(in.Notiz)
	if in.Ausfuehrungsklasse != nil {
		k := strings.ToUpper(strings.TrimSpace(*in.Ausfuehrungsklasse))
		if k == "" {
			in.Ausfuehrungsklasse = nil
		} else {
			in.Ausfuehrungsklasse = &k
		}
	}
	if in.Liefertermin != nil {
		d := dateOnly(*in.Liefertermin)
		in.Liefertermin = &d
	}
	return in
}

// ValidateOrderInput prueft eine normalisierte Eingabe. Ein Liefertermin
// in der Vergangenheit ist erlaubt (Nacherfassung).
func ValidateOrderInput(in OrderInput) error {
	if in.ProjectID == uuid.Nil {
		return errors.New("Projekt erforderlich")
	}
	if in.ElevationID == uuid.Nil {
		return errors.New("Position erforderlich")
	}
	if in.SingleElevationID == uuid.Nil {
		return errors.New("Variante erforderlich")
	}
	return ValidateOrderDetails(in.Stueckzahl, in.Ausfuehrungsklasse)
}

// ValidateOrderDetails prueft die im Entwurf aenderbaren Angaben.
func ValidateOrderDetails(stueckzahl int, ausfuehrungsklasse *string) error {
	if stueckzahl <= 0 {
		return errors.New("Stückzahl muss größer als 0 sein")
	}
	if stueckzahl > MaxStueckzahl {
		return errors.New("Stückzahl ist zu groß")
	}
	if ausfuehrungsklasse != nil && !validExc(*ausfuehrungsklasse) {
		return errors.New("Ungültige Ausführungsklasse")
	}
	return nil
}

func validExc(k string) bool {
	for _, x := range Ausfuehrungsklassen {
		if x == k {
			return true
		}
	}
	return false
}

// ValidateStornoReason: Storno nur mit Begruendung.
func ValidateStornoReason(reason string) (string, error) {
	r := strings.TrimSpace(reason)
	if r == "" {
		return "", errors.New("Begründung für das Storno erforderlich")
	}
	return r, nil
}

// QuantityExceeded: Summe der nicht stornierten Auftraege zur Position
// (ohne diesen) plus dieser Auftrag liegt ueber der Positionsmenge. Die
// Menge kommt aus Logikal als REAL; Rundungsrauschen zaehlt nicht.
func QuantityExceeded(positionMenge float64, andereAuftraege, stueckzahl int) bool {
	return float64(andereAuftraege+stueckzahl) > positionMenge+1e-6
}

// Snapshot ist der Inhalt, der bei der Freigabe kopiert wird.
type Snapshot struct {
	PhaseName      string         `json:"phase_name"`
	PositionNummer string         `json:"position_nummer"`
	PositionName   string         `json:"position_name"`
	PositionGUID   string         `json:"position_guid"`
	VarianteName   string         `json:"variante_name"`
	VarianteGUID   string         `json:"variante_guid"`
	WidthMM        *float64       `json:"width_mm"`
	HeightMM       *float64       `json:"height_mm"`
	Items          []SnapshotItem `json:"items"`
}

type SnapshotItem struct {
	Art           string   `json:"art"`
	SupplierCode  string   `json:"supplier_code"`
	ArticleCode   string   `json:"article_code"`
	Description   string   `json:"description"`
	Configuration string   `json:"configuration"`
	LengthMM      *float64 `json:"length_mm"`
	WidthMM       *float64 `json:"width_mm"`
	HeightMM      *float64 `json:"height_mm"`
	AreaM2        *float64 `json:"area_m2"`
	MengeJeStueck float64  `json:"menge_je_stueck"`
	Unit          string   `json:"unit"`
	// Lagermaterial der Projektzeile (leer: keins). Nicht im Fingerabdruck:
	// ein geaenderter Materialbezug aendert nicht, was gefertigt wird.
	MaterialID string `json:"-"`
}

func roundTo(p *float64, digits int) *float64 {
	if p == nil {
		return nil
	}
	f := math.Pow(10, float64(digits))
	v := math.Round(*p*f) / f
	return &v
}

// canonical rundet wie die Spalten der Migration 107 (Masse auf 2,
// Flaeche auf 4, Menge auf 6 Stellen) und sortiert die Positionen - so
// ergibt dieselbe Position nach Speichern und Neuladen denselben
// Fingerabdruck, unabhaengig von der Reihenfolge der Abfrage.
func canonical(s Snapshot) Snapshot {
	s.WidthMM, s.HeightMM = roundTo(s.WidthMM, 2), roundTo(s.HeightMM, 2)
	items := make([]SnapshotItem, len(s.Items))
	for i, it := range s.Items {
		it.LengthMM, it.WidthMM, it.HeightMM = roundTo(it.LengthMM, 2), roundTo(it.WidthMM, 2), roundTo(it.HeightMM, 2)
		it.AreaM2 = roundTo(it.AreaM2, 4)
		it.MengeJeStueck = *roundTo(&it.MengeJeStueck, 6)
		items[i] = it
	}
	sort.SliceStable(items, func(a, b int) bool {
		ka, _ := json.Marshal(items[a])
		kb, _ := json.Marshal(items[b])
		return string(ka) < string(kb)
	})
	s.Items = items
	return s
}

// Fingerprint ist der SHA-256 der kanonischen Kopie (hex). Weicht der
// Fingerabdruck der aktuellen Projektdaten von dem bei der Freigabe ab,
// zeigt der Auftrag HintPositionGeaendert.
func Fingerprint(s Snapshot) string {
	b, _ := json.Marshal(canonical(s))
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// ValidateSnapshot: eine Freigabe braucht eine Positionsnummer und
// gueltige Materialzeilen.
func ValidateSnapshot(s Snapshot) error {
	if strings.TrimSpace(s.PositionNummer) == "" {
		return errors.New("Position ohne Nummer kann nicht freigegeben werden")
	}
	for _, it := range s.Items {
		switch it.Art {
		case ArtProfil, ArtArtikel, ArtGlas:
		default:
			return errors.New("Ungültige Materialart")
		}
		if math.IsNaN(it.MengeJeStueck) || math.IsInf(it.MengeJeStueck, 0) || it.MengeJeStueck < 0 {
			return errors.New("Ungültige Materialmenge")
		}
	}
	return nil
}

// Einheiten, die "Stueck" bedeuten (Logikal schreibt "Stk", die
// Stammdaten kennen "pcs"); verglichen ohne Gross-/Kleinschreibung.
var stueckEinheiten = map[string]bool{"": true, "stk": true, "stk.": true, "st": true, "st.": true, "stück": true, "stueck": true, "pcs": true, "pc": true}

func normEinheit(u string) string {
	u = strings.ToLower(strings.TrimSpace(u))
	if stueckEinheiten[u] {
		return "stk"
	}
	return u
}

// ReservationQty rechnet den Bedarf einer Zeile der Fertigungsstueckliste
// (mengeGesamt = Menge je Stueck x Stueckzahl) in die Einheit des
// Lagermaterials um (Backlog H.1.5). Gleiche Einheit: unveraendert; leere
// Zeileneinheit zaehlt als Stueck. Profile mit Laenge nur auf Material in
// m oder mm (Gesamtlaenge). Alles andere (z. B. Profilstueck auf Stangen,
// Glas auf m²) laesst sich nicht sicher umrechnen: ok=false, die Zeile
// wird nicht reserviert.
func ReservationQty(art, zeilenEinheit, materialEinheit string, lengthMM *float64, mengeGesamt float64) (float64, bool) {
	if mengeGesamt <= 0 {
		return 0, false
	}
	z, m := normEinheit(zeilenEinheit), normEinheit(materialEinheit)
	if art == ArtProfil && lengthMM != nil && *lengthMM > 0 {
		// Zugeschnittene Stuecke sind keine Stangen: nur Laengeneinheiten.
		if z != "stk" {
			return 0, false
		}
		switch m {
		case "m":
			return mengeGesamt * *lengthMM / 1000, true
		case "mm":
			return mengeGesamt * *lengthMM, true
		}
		return 0, false
	}
	if z == m {
		return mengeGesamt, true
	}
	return 0, false
}

// Shortfall: Bedarf minus Reserviertes, auf die 6 Stellen der Spalte
// gerundet (das Abrunden der Reservierung erzeugt sonst Rauschen), nie
// negativ.
func Shortfall(bedarf, reserviert float64) float64 {
	d := math.Round((bedarf-reserviert)*1e6) / 1e6
	if d <= 0 {
		return 0
	}
	return d
}
