package purchasing

import (
	"slices"
	"testing"
	"time"
)

// Fachregeln der Nachweise der Zulieferer ohne Datenbank (Backlog H.4.1b).

func d(s string) *time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return &t
}

func sp(s string) *string { return &s }

func TestNormalizeCertificate(t *testing.T) {
	for _, c := range []struct {
		name string
		in   Certificate
		err  string
		exc  *string
	}{
		{"EN 1090 mit EXC, getrimmt", Certificate{Art: " EN1090 ", ExcBis: sp(" exc2 ")}, "", sp("EXC2")},
		{"ISO 3834, leere EXC wird nil", Certificate{Art: "iso3834", ExcBis: sp("  ")}, "", nil},
		{"gleicher Tag", Certificate{Art: "sonstiges", GueltigVon: d("2026-01-01"), GueltigBis: d("2026-01-01")}, "", nil},
		{"Art unbekannt", Certificate{Art: "iso9001"}, "Art des Zertifikats unbekannt", nil},
		{"EN 1090 ohne EXC", Certificate{Art: "en1090"}, "Ausführungsklasse erforderlich", nil},
		{"EN 1090 leere EXC", Certificate{Art: "en1090", ExcBis: sp(" ")}, "Ausführungsklasse erforderlich", nil},
		{"EXC unbekannt", Certificate{Art: "en1090", ExcBis: sp("EXC5")}, "Ausführungsklasse unbekannt", nil},
		{"EXC bei ISO 3834", Certificate{Art: "iso3834", ExcBis: sp("EXC2")}, "Ausführungsklasse nur bei EN 1090", nil},
		{"bis vor von", Certificate{Art: "sonstiges", GueltigVon: d("2026-02-01"), GueltigBis: d("2026-01-31")}, "Gültig bis liegt vor gültig von", nil},
	} {
		in := c.in
		err := NormalizeCertificate(&in)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != c.err {
			t.Errorf("%s: Fehler %q, erwartet %q", c.name, got, c.err)
			continue
		}
		if c.err == "" && (c.exc == nil) != (in.ExcBis == nil) || (c.exc != nil && in.ExcBis != nil && *c.exc != *in.ExcBis) {
			t.Errorf("%s: ExcBis %v, erwartet %v", c.name, in.ExcBis, c.exc)
		}
	}
}

func TestValidPapierArt(t *testing.T) {
	for _, a := range []string{"leistungserklaerung", " Zeugnis_3_1 ", "lieferschein"} {
		if !ValidPapierArt(a) {
			t.Errorf("%q abgelehnt", a)
		}
	}
	for _, a := range []string{"", "zeugnis_3_1_b", "rechnung"} {
		if ValidPapierArt(a) {
			t.Errorf("%q angenommen", a)
		}
	}
}

func TestCovers(t *testing.T) {
	c := Certificate{Art: "en1090", GueltigVon: d("2026-01-01"), GueltigBis: d("2026-06-30")}
	for _, x := range []struct {
		from, to string
		want     bool
	}{
		{"2026-01-01", "2026-06-30", true}, // Grenzen einschliesslich
		{"2026-03-01", "2026-03-01", true},
		{"2025-12-31", "2026-03-01", false}, // noch nicht gueltig
		{"2026-03-01", "2026-07-01", false}, // vorher abgelaufen
	} {
		if got := c.Covers(*d(x.from), *d(x.to)); got != x.want {
			t.Errorf("%s..%s: %v", x.from, x.to, got)
		}
	}
	// Uhrzeit spielt keine Rolle.
	late := time.Date(2026, 6, 30, 23, 59, 0, 0, time.UTC)
	if !c.Covers(late, late) {
		t.Error("letzter Tag am Abend nicht gueltig")
	}
	if !(Certificate{Art: "en1090"}).Covers(*d("2000-01-01"), *d("2099-01-01")) {
		t.Error("ohne Gueltigkeit nicht unbegrenzt")
	}
	c.Storniert = true
	if c.Covers(*d("2026-03-01"), *d("2026-03-01")) {
		t.Error("storniertes Zertifikat gilt")
	}
}

func TestOrderStichtag(t *testing.T) {
	order := *d("2026-03-01")
	if got := OrderStichtag(order, nil); !got.Equal(order) {
		t.Errorf("ohne Termin: %v", got)
	}
	if got := OrderStichtag(order, []*time.Time{nil, d("2026-04-10"), d("2026-05-02"), d("2026-02-01")}); !got.Equal(*d("2026-05-02")) {
		t.Errorf("spaetester Termin: %v", got)
	}
	if got := OrderStichtag(order, []*time.Time{d("2026-02-01")}); !got.Equal(order) {
		t.Errorf("Termin vor Bestelldatum: %v", got)
	}
}

func TestOrderCertificateHint(t *testing.T) {
	order, stichtag := *d("2026-03-01"), *d("2026-05-02")
	valid := Certificate{Art: "en1090", ExcBis: sp("EXC2"), GueltigBis: d("2026-05-02")}
	short := Certificate{Art: "en1090", ExcBis: sp("EXC2"), GueltigBis: d("2026-05-01")}
	iso := Certificate{Art: "iso3834"}
	storno := Certificate{Art: "en1090", ExcBis: sp("EXC2"), Storniert: true}
	for _, c := range []struct {
		name  string
		needs bool
		certs []Certificate
		want  string
	}{
		{"nichts nachweispflichtig", false, nil, ""},
		{"gueltig bis Stichtag", true, []Certificate{short, valid}, ""},
		{"kein Zertifikat", true, nil, HintZertifikatFehlt},
		{"nur ISO 3834", true, []Certificate{iso}, HintZertifikatFehlt},
		{"nur storniert", true, []Certificate{storno}, HintZertifikatFehlt},
		{"endet vor Stichtag", true, []Certificate{short, iso}, HintZertifikatUngueltig},
		{"beginnt nach Bestelldatum", true, []Certificate{{Art: "en1090", ExcBis: sp("EXC2"), GueltigVon: d("2026-03-02")}}, HintZertifikatUngueltig},
	} {
		if got := OrderCertificateHint(c.needs, c.certs, order, stichtag); got != c.want {
			t.Errorf("%s: %q, erwartet %q", c.name, got, c.want)
		}
	}
}

func TestExpiringCertificates(t *testing.T) {
	today := *d("2026-11-03")
	certs := []Certificate{
		{Art: "en1090", GueltigBis: d("2027-01-02")},                                 // 0: genau 60 Tage
		{Art: "iso3834", GueltigVon: d("2027-01-03"), GueltigBis: d("2027-01-03")},   // 1: 61 Tage, kein Nachfolger von 2
		{Art: "iso3834", GueltigBis: d("2026-11-03")},                                // 2: heute
		{Art: "sonstiges", GueltigBis: d("2026-11-02")},                              // 3: schon abgelaufen
		{Art: "en1090", GueltigVon: d("2027-06-01")},                                 // 4: ohne Ende, beginnt erst nach 0
		{Art: "iso3834", GueltigBis: d("2026-12-01"), Storniert: true},               // 5: storniert
		{Art: "sonstiges", GueltigBis: d("2026-12-01")},                              // 6: Nachfolger 7
		{Art: "sonstiges", GueltigVon: d("2026-12-02"), GueltigBis: d("2029-01-01")}, // 7
	}
	if got := ExpiringCertificates(certs, today); !slices.Equal(got, []int{0, 2}) {
		t.Errorf("laufen ab: %v, erwartet [0 2]", got)
	}
	// Allein und schon abgelaufen: kein Hinweis "laeuft ab".
	if got := ExpiringCertificates(certs[3:4], today); len(got) != 0 {
		t.Errorf("abgelaufen: %v", got)
	}
	// Nachfolger erst einen Tag spaeter: Luecke, also Hinweis.
	certs[7].GueltigVon = d("2026-12-03")
	if got := ExpiringCertificates(certs, today); !slices.Equal(got, []int{0, 2, 6}) {
		t.Errorf("mit Luecke: %v, erwartet [0 2 6]", got)
	}
	// Nachfolger anderer Art zaehlt nicht; stornierter Nachfolger auch nicht.
	certs[7] = Certificate{Art: "en1090", GueltigBis: d("2029-01-01")}
	if got := ExpiringCertificates(certs, today); !slices.Equal(got, []int{2, 6}) {
		t.Errorf("Nachfolger EN 1090: %v, erwartet [2 6]", got)
	}
	certs[7].Storniert = true
	if got := ExpiringCertificates(certs, today); !slices.Equal(got, []int{0, 2, 6}) {
		t.Errorf("Nachfolger storniert: %v, erwartet [0 2 6]", got)
	}
}

func TestMissingDeclarations(t *testing.T) {
	items := []EvidenceItem{{ID: "a", Nachweis: true}, {ID: "b", Nachweis: true}, {ID: "c"}}
	le := func(item *string) DeliveryPaper { return DeliveryPaper{ItemID: item, Art: PapierLeistungserklaerung} }
	for _, c := range []struct {
		name   string
		status string
		papers []DeliveryPaper
		want   []string
	}{
		{"bestellt: noch nichts geliefert", "ordered", nil, nil},
		{"erhalten ohne Papiere", "received", nil, []string{"a", "b"}},
		{"Status mit Leerraum", " Received ", nil, []string{"a", "b"}},
		{"eine Position belegt", "received", []DeliveryPaper{le(sp("a"))}, []string{"b"}},
		{"fuer die ganze Bestellung", "received", []DeliveryPaper{le(nil)}, nil},
		{"leere Position = ganze Bestellung", "received", []DeliveryPaper{le(sp(" "))}, nil},
		{"Zeugnis statt Leistungserklaerung", "received", []DeliveryPaper{{ItemID: sp("a"), Art: PapierZeugnis31}}, []string{"a", "b"}},
		{"storniert zaehlt nicht", "received", []DeliveryPaper{{Art: PapierLeistungserklaerung, Storniert: true}}, []string{"a", "b"}},
	} {
		if got := MissingDeclarations(c.status, items, c.papers); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, erwartet %v", c.name, got, c.want)
		}
	}
}

func TestOrderEvidenceHints(t *testing.T) {
	items := []EvidenceItem{{ID: "a", Nachweis: true}}
	order := *d("2026-03-01")
	dates := []*time.Time{d("2026-04-01")}
	got := OrderEvidenceHints("received", items, nil, nil, order, dates)
	if !slices.Equal(got, []string{HintZertifikatFehlt, HintLeistungserklaerungFehlt}) {
		t.Errorf("beides fehlt: %v", got)
	}
	certs := []Certificate{{Art: "en1090", ExcBis: sp("EXC2"), GueltigBis: d("2026-03-31")}}
	if got := OrderEvidenceHints("ordered", items, nil, certs, order, dates); !slices.Equal(got, []string{HintZertifikatUngueltig}) {
		t.Errorf("endet vor Liefertermin: %v", got)
	}
	certs[0].GueltigBis = d("2026-04-01")
	papers := []DeliveryPaper{{Art: PapierLeistungserklaerung}}
	if got := OrderEvidenceHints("received", items, papers, certs, order, dates); len(got) != 0 {
		t.Errorf("alles da: %v", got)
	}
	if got := OrderEvidenceHints("received", []EvidenceItem{{ID: "a"}}, nil, nil, order, dates); len(got) != 0 {
		t.Errorf("nichts nachweispflichtig: %v", got)
	}
	if got := OrderEvidenceHints("canceled", items, nil, nil, order, dates); len(got) != 0 {
		t.Errorf("storniert: %v", got)
	}
}
