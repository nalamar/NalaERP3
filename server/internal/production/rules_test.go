package production

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Fachregeln der Fertigungsauftraege ohne Datenbank (Backlog H.1.1).

func strPtr(s string) *string        { return &s }
func fPtr(v float64) *float64        { return &v }
func timePtr(t time.Time) *time.Time { return &t }

func TestNextStatus(t *testing.T) {
	for _, c := range []struct{ from, aktion, want string }{
		{StatusEntwurf, AktionFreigeben, StatusFreigegeben},
		{StatusFreigegeben, AktionBeginnen, StatusInArbeit},
		{StatusInArbeit, AktionFertigmelden, StatusFertig},
		{StatusEntwurf, AktionStornieren, StatusStorniert},
		{StatusFreigegeben, AktionStornieren, StatusStorniert},
		{StatusInArbeit, AktionStornieren, StatusStorniert},
	} {
		got, err := NextStatus(c.from, c.aktion)
		if err != nil || got != c.want {
			t.Errorf("%s + %s: %q, %v", c.from, c.aktion, got, err)
		}
	}
	for _, c := range []struct{ from, aktion, want string }{
		{StatusFertig, AktionStornieren, "Ein fertiger Auftrag kann nicht storniert werden"},
		{StatusStorniert, AktionStornieren, "Auftrag ist bereits storniert"},
		{"geloescht", AktionStornieren, "Unbekannter Status"},
		{StatusEntwurf, "loeschen", "Unbekannte Aktion"},
		// kein Ueberspringen, kein Zurueck, nichts nach Storno
		{StatusEntwurf, AktionBeginnen, "Aktion im aktuellen Status nicht erlaubt"},
		{StatusEntwurf, AktionFertigmelden, "Aktion im aktuellen Status nicht erlaubt"},
		{StatusFreigegeben, AktionFreigeben, "Aktion im aktuellen Status nicht erlaubt"},
		{StatusFreigegeben, AktionFertigmelden, "Aktion im aktuellen Status nicht erlaubt"},
		{StatusInArbeit, AktionBeginnen, "Aktion im aktuellen Status nicht erlaubt"},
		{StatusFertig, AktionFertigmelden, "Aktion im aktuellen Status nicht erlaubt"},
		{StatusStorniert, AktionFreigeben, "Aktion im aktuellen Status nicht erlaubt"},
	} {
		got, err := NextStatus(c.from, c.aktion)
		if err == nil || err.Error() != c.want || got != "" {
			t.Errorf("%s + %s: %q, %v", c.from, c.aktion, got, err)
		}
	}
}

func TestNormalizeOrderInput(t *testing.T) {
	in := NormalizeOrderInput(OrderInput{Notiz: " n ", Ausfuehrungsklasse: strPtr(" exc2 "),
		Liefertermin: timePtr(time.Date(2026, 11, 3, 15, 4, 0, 0, time.UTC))})
	if in.Notiz != "n" || in.Ausfuehrungsklasse == nil || *in.Ausfuehrungsklasse != "EXC2" ||
		!in.Liefertermin.Equal(time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("normalisiert: %+v", in)
	}
	if k := NormalizeOrderInput(OrderInput{Ausfuehrungsklasse: strPtr("  ")}).Ausfuehrungsklasse; k != nil {
		t.Fatalf("leere Klasse: %v", *k)
	}
	if d := NormalizeOrderInput(OrderInput{}); d.Liefertermin != nil || d.Ausfuehrungsklasse != nil {
		t.Fatalf("ohne Angaben: %+v", d)
	}
}

func TestValidateOrderInput(t *testing.T) {
	ok := OrderInput{ProjectID: uuid.New(), ElevationID: uuid.New(), SingleElevationID: uuid.New(), Stueckzahl: 1}
	with := func(f func(*OrderInput)) OrderInput { c := ok; f(&c); return c }
	for name, in := range map[string]OrderInput{
		"ein Stueck":        ok,
		"Obergrenze":        with(func(c *OrderInput) { c.Stueckzahl = MaxStueckzahl }),
		"EXC4":              with(func(c *OrderInput) { c.Ausfuehrungsklasse = strPtr("EXC4") }),
		"alter Liefertag":   with(func(c *OrderInput) { c.Liefertermin = timePtr(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) }),
		"ohne Klasse (nil)": with(func(c *OrderInput) { c.Ausfuehrungsklasse = nil }),
	} {
		if err := ValidateOrderInput(in); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, k := range Ausfuehrungsklassen {
		if err := ValidateOrderInput(with(func(c *OrderInput) { c.Ausfuehrungsklasse = strPtr(k) })); err != nil {
			t.Errorf("Klasse %s: %v", k, err)
		}
	}
	for want, in := range map[string]OrderInput{
		"Projekt erforderlich":             with(func(c *OrderInput) { c.ProjectID = uuid.Nil }),
		"Position erforderlich":            with(func(c *OrderInput) { c.ElevationID = uuid.Nil }),
		"Variante erforderlich":            with(func(c *OrderInput) { c.SingleElevationID = uuid.Nil }),
		"Stückzahl muss größer als 0 sein": with(func(c *OrderInput) { c.Stueckzahl = 0 }),
		"Stückzahl ist zu groß":            with(func(c *OrderInput) { c.Stueckzahl = MaxStueckzahl + 1 }),
		"Ungültige Ausführungsklasse":      with(func(c *OrderInput) { c.Ausfuehrungsklasse = strPtr("EXC5") }),
	} {
		if err := ValidateOrderInput(in); err == nil || err.Error() != want {
			t.Errorf("%s: %v", want, err)
		}
	}
	if err := ValidateOrderInput(with(func(c *OrderInput) { c.Stueckzahl = -3 })); err == nil {
		t.Error("negative Stückzahl angenommen")
	}
	// klein geschrieben ist erst nach dem Normalisieren gueltig
	if err := ValidateOrderInput(with(func(c *OrderInput) { c.Ausfuehrungsklasse = strPtr("exc2") })); err == nil {
		t.Error("unnormalisierte Klasse angenommen")
	}
}

func TestValidateStornoReason(t *testing.T) {
	if r, err := ValidateStornoReason("  Maß falsch "); err != nil || r != "Maß falsch" {
		t.Fatalf("Grund: %q, %v", r, err)
	}
	for _, bad := range []string{"", "   ", "\t\n"} {
		if _, err := ValidateStornoReason(bad); err == nil || err.Error() != "Begründung für das Storno erforderlich" {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestQuantityExceeded(t *testing.T) {
	for _, c := range []struct {
		menge          float64
		andere, stueck int
		want           bool
	}{
		{10, 0, 10, false},
		{10, 4, 6, false},
		{10, 4, 7, true},
		{10, 0, 11, true},
		{10.0000001, 0, 10, false}, // REAL-Rauschen
		{9.9999999, 0, 10, false},
		{9.99, 0, 10, true},
		{0, 0, 1, true}, // Position ohne Menge
	} {
		if got := QuantityExceeded(c.menge, c.andere, c.stueck); got != c.want {
			t.Errorf("Menge %v, andere %d, Stück %d: %v", c.menge, c.andere, c.stueck, got)
		}
	}
}

func sampleSnapshot() Snapshot {
	return Snapshot{
		PhaseName: "Haus A", PositionNummer: "3", PositionName: "Fenster", PositionGUID: "g-pos", VarianteName: "links",
		VarianteGUID: "g-var", WidthMM: fPtr(1230), HeightMM: fPtr(1480),
		Items: []SnapshotItem{
			{Art: ArtProfil, ArticleCode: "P-1", LengthMM: fPtr(1230.4), MengeJeStueck: 2, Unit: "Stk"},
			{Art: ArtArtikel, ArticleCode: "A-7", MengeJeStueck: 1.5},
			{Art: ArtGlas, Configuration: "4/16/4", WidthMM: fPtr(1100), HeightMM: fPtr(1350), AreaM2: fPtr(1.485), MengeJeStueck: 1},
		},
	}
}

func TestFingerprint(t *testing.T) {
	base := Fingerprint(sampleSnapshot())
	if len(base) != 64 {
		t.Fatalf("Länge: %d", len(base))
	}
	same := map[string]func(*Snapshot){
		"andere Reihenfolge": func(s *Snapshot) { s.Items[0], s.Items[2] = s.Items[2], s.Items[0] },
		// wie nach numeric(12,2) gespeichert und neu geladen
		"Länge unter Rundung":  func(s *Snapshot) { s.Items[0].LengthMM = fPtr(1230.4000001) },
		"Fläche unter Rundung": func(s *Snapshot) { s.Items[2].AreaM2 = fPtr(1.48500004) },
		"Menge unter Rundung":  func(s *Snapshot) { s.Items[1].MengeJeStueck = 1.5000000001 },
		"Breite unter Rundung": func(s *Snapshot) { s.WidthMM = fPtr(1230.001) },
	}
	for name, f := range same {
		s := sampleSnapshot()
		f(&s)
		if got := Fingerprint(s); got != base {
			t.Errorf("%s: Fingerabdruck geändert", name)
		}
	}
	changed := map[string]func(*Snapshot){
		"Phase":           func(s *Snapshot) { s.PhaseName = "Haus B" },
		"Positionsnummer": func(s *Snapshot) { s.PositionNummer = "4" },
		"Positionsname":   func(s *Snapshot) { s.PositionName = "Tür" },
		"Positions-GUID":  func(s *Snapshot) { s.PositionGUID = "x" },
		"Variante":        func(s *Snapshot) { s.VarianteName = "rechts" },
		"Varianten-GUID":  func(s *Snapshot) { s.VarianteGUID = "x" },
		"Breite":          func(s *Snapshot) { s.WidthMM = fPtr(1231) },
		"Höhe weg":        func(s *Snapshot) { s.HeightMM = nil },
		"Profillänge":     func(s *Snapshot) { s.Items[0].LengthMM = fPtr(1230.5) },
		"Artikelnummer":   func(s *Snapshot) { s.Items[1].ArticleCode = "A-8" },
		"Lieferant":       func(s *Snapshot) { s.Items[1].SupplierCode = "S" },
		"Beschreibung":    func(s *Snapshot) { s.Items[1].Description = "neu" },
		"Menge":           func(s *Snapshot) { s.Items[1].MengeJeStueck = 2 },
		"Einheit":         func(s *Snapshot) { s.Items[0].Unit = "m" },
		"Glasaufbau":      func(s *Snapshot) { s.Items[2].Configuration = "4/14/4" },
		"Glasbreite":      func(s *Snapshot) { s.Items[2].WidthMM = fPtr(1101) },
		"Glashöhe":        func(s *Snapshot) { s.Items[2].HeightMM = fPtr(1351) },
		"Glasfläche":      func(s *Snapshot) { s.Items[2].AreaM2 = fPtr(1.486) },
		"Art":             func(s *Snapshot) { s.Items[1].Art = ArtProfil },
		"Zeile entfernt":  func(s *Snapshot) { s.Items = s.Items[:2] },
		"Zeile doppelt":   func(s *Snapshot) { s.Items = append(s.Items, s.Items[1]) },
		"leere Liste":     func(s *Snapshot) { s.Items = nil },
	}
	for name, f := range changed {
		s := sampleSnapshot()
		f(&s)
		if got := Fingerprint(s); got == base {
			t.Errorf("%s: Änderung nicht erkannt", name)
		}
	}
	// canonical veraendert die Eingabe nicht
	s := sampleSnapshot()
	_ = Fingerprint(s)
	if s.Items[0].Art != ArtProfil || *s.Items[0].LengthMM != 1230.4 {
		t.Fatalf("Eingabe verändert: %+v", s.Items[0])
	}
}

func TestValidateSnapshot(t *testing.T) {
	if err := ValidateSnapshot(sampleSnapshot()); err != nil {
		t.Fatalf("gültig: %v", err)
	}
	if err := ValidateSnapshot(Snapshot{PositionNummer: "1"}); err != nil {
		t.Fatalf("ohne Material: %v", err)
	}
	for want, f := range map[string]func(*Snapshot){
		"Position ohne Nummer kann nicht freigegeben werden": func(s *Snapshot) { s.PositionNummer = " " },
		"Ungültige Materialart":                              func(s *Snapshot) { s.Items[1].Art = "beschlag" },
		"Ungültige Materialmenge":                            func(s *Snapshot) { s.Items[1].MengeJeStueck = -1 },
	} {
		s := sampleSnapshot()
		f(&s)
		if err := ValidateSnapshot(s); err == nil || err.Error() != want {
			t.Errorf("%s: %v", want, err)
		}
	}
	for _, bad := range []float64{math.NaN(), math.Inf(1)} {
		s := sampleSnapshot()
		s.Items[0].MengeJeStueck = bad
		if err := ValidateSnapshot(s); err == nil || err.Error() != "Ungültige Materialmenge" {
			t.Errorf("Menge %v: %v", bad, err)
		}
	}
}
