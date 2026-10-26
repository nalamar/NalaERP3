package hr

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Validierung des Katalogs greift vor jedem DB-Zugriff (nil-Pool).

func TestQualificationTypeCreateValidatesBeforeTouchingDatabase(t *testing.T) {
	zero, negative := 0, -3
	valid := QualificationTypeInput{Code: "STAPLER_SCHEIN", Name: "Staplerschein", Kategorie: QualificationKategorieQualifikation}
	with := func(mod func(*QualificationTypeInput)) QualificationTypeInput {
		in := valid
		mod(&in)
		return in
	}

	cases := []struct {
		name      string
		in        QualificationTypeInput
		companyID string
		wantErr   string
	}{
		{"Mandant fehlt", valid, " ", "Mandant erforderlich"},
		{"Code fehlt", with(func(in *QualificationTypeInput) { in.Code = "  " }), "default", "Code erforderlich"},
		{"Code mit Leerzeichen", with(func(in *QualificationTypeInput) { in.Code = "STAPLER SCHEIN" }), "default", "Ungültiger Code (erlaubt sind A-Z, 0-9 und _, höchstens 40 Zeichen)"},
		{"Code mit Umlaut", with(func(in *QualificationTypeInput) { in.Code = "HUBARBEITSBÜHNE" }), "default", "Ungültiger Code (erlaubt sind A-Z, 0-9 und _, höchstens 40 Zeichen)"},
		{"Code zu lang", with(func(in *QualificationTypeInput) { in.Code = strings.Repeat("A", 41) }), "default", "Ungültiger Code (erlaubt sind A-Z, 0-9 und _, höchstens 40 Zeichen)"},
		{"Name fehlt", with(func(in *QualificationTypeInput) { in.Name = "   " }), "default", "Name erforderlich"},
		{"Kategorie fehlt", with(func(in *QualificationTypeInput) { in.Kategorie = "" }), "default", "Ungültige Kategorie"},
		// Arbeitsmedizinische Vorsorge ist bewusst keine Kategorie (ADR 0027).
		{"Kategorie unbekannt", with(func(in *QualificationTypeInput) { in.Kategorie = "gesundheit" }), "default", "Ungültige Kategorie"},
		{"Frist 0", with(func(in *QualificationTypeInput) { in.GueltigkeitMonate = &zero }), "default", "Gültigkeit muss größer als 0 sein"},
		{"Frist negativ", with(func(in *QualificationTypeInput) { in.GueltigkeitMonate = &negative }), "default", "Gültigkeit muss größer als 0 sein"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewQualificationTypeService(nil).Create(context.Background(), tc.in, tc.companyID)
			if got != nil {
				t.Fatalf("erwartet kein Ergebnis, bekommen %+v", got)
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("erwartet %q, bekommen %v", tc.wantErr, err)
			}
		})
	}
}

func TestQualificationTypeUpdateValidatesBeforeTouchingDatabase(t *testing.T) {
	zero := 0
	s := NewQualificationTypeService(nil)
	valid := QualificationTypeUpdate{Name: "Staplerschein", Kategorie: QualificationKategorieQualifikation, Aktiv: true}

	cases := []struct {
		name      string
		in        QualificationTypeUpdate
		companyID string
		wantErr   string
	}{
		{"Mandant fehlt", valid, "", "Mandant erforderlich"},
		{"Name fehlt", QualificationTypeUpdate{Kategorie: QualificationKategorieQualifikation}, "default", "Name erforderlich"},
		{"Kategorie unbekannt", QualificationTypeUpdate{Name: "X", Kategorie: "kurs"}, "default", "Ungültige Kategorie"},
		{"Frist 0", QualificationTypeUpdate{Name: "X", Kategorie: QualificationKategorieUnterweisung, GueltigkeitMonate: &zero}, "default", "Gültigkeit muss größer als 0 sein"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.Update(context.Background(), uuid.New(), tc.in, tc.companyID)
			if got != nil {
				t.Fatalf("erwartet kein Ergebnis, bekommen %+v", got)
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("erwartet %q, bekommen %v", tc.wantErr, err)
			}
		})
	}
	if _, err := s.AdoptDefaults(context.Background(), " "); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("AdoptDefaults: erwartet 'Mandant erforderlich', bekommen %v", err)
	}
}

// Die Regeln, die der Vorschlagskatalog einhalten muss (ADR 0027). Wer
// einen Eintrag ergaenzt, bekommt hier Bescheid, wenn er eine davon
// verletzt.
func TestDefaultQualificationTypesFollowTheCatalogRules(t *testing.T) {
	defaults := DefaultQualificationTypes()
	if len(defaults) == 0 {
		t.Fatal("Vorschlagskatalog ist leer")
	}

	seen := make(map[string]bool)
	var pflicht []string
	for _, d := range defaults {
		t.Run(d.Code, func(t *testing.T) {
			// Jeder Eintrag muss so, wie er dasteht, anlegbar sein - auch
			// der Code darf keine Normalisierung brauchen.
			if d.Code != normalizeQualificationCode(d.Code) {
				t.Fatalf("Code ist nicht normalisiert: %q", d.Code)
			}
			if err := validateQualificationTypeInput(d); err != nil {
				t.Fatalf("Eintrag ist nicht anlegbar: %v", err)
			}
			if seen[d.Code] {
				t.Fatalf("Code kommt mehrfach vor: %s", d.Code)
			}
			seen[d.Code] = true

			switch d.Kategorie {
			case QualificationKategorieUnterweisung:
				// Eine wiederkehrende Unterweisung ohne Frist wuerde nie angemahnt.
				if d.GueltigkeitMonate == nil {
					t.Fatal("Unterweisung ohne Frist")
				}
			case QualificationKategorieBeauftragung:
				// Eine Beauftragung gilt bis Widerruf.
				if d.GueltigkeitMonate != nil {
					t.Fatalf("Beauftragung mit Frist (%d Monate)", *d.GueltigkeitMonate)
				}
			}
			if strings.TrimSpace(d.Hinweis) == "" {
				t.Fatal("Eintrag ohne Hinweis zur Herkunft der Frist")
			}
			// Keine Gesundheitsdaten im Katalog.
			for _, forbidden := range []string{"VORSORGE", "EIGNUNG", "G25", "G41", "UNTERSUCHUNG"} {
				if strings.Contains(d.Code, forbidden) || strings.Contains(strings.ToUpper(d.Name), forbidden) {
					t.Fatalf("Eintrag gehoert nicht in den Katalog (Gesundheitsdaten): %s", d.Name)
				}
			}
		})
		if d.PflichtFuerAlle {
			pflicht = append(pflicht, d.Code)
		}
	}

	if len(pflicht) != 1 || pflicht[0] != "ARBEITSSCHUTZ" {
		t.Fatalf("erwartet genau ARBEITSSCHUTZ als Pflicht fuer alle, bekommen %v", pflicht)
	}

	// Die fuenf vom Nutzer genannten Themen (2026-10-13) muessen enthalten
	// sein; Stapler und Hubarbeitsbuehne mit allen drei Bestandteilen.
	for _, code := range []string{
		"ARBEITSSCHUTZ", "ERSTHELFER",
		"STAPLER_SCHEIN", "STAPLER_UNTERWEISUNG", "STAPLER_BEAUFTRAGUNG",
		"HUBARBEITSBUEHNE_SCHEIN", "HUBARBEITSBUEHNE_UNTERWEISUNG", "HUBARBEITSBUEHNE_BEAUFTRAGUNG",
		"LEITERN_UNTERWEISUNG", "LEITERN_PRUEFER",
		// Fuhrpark (ADR 0032 §5).
		"FUEHRERSCHEINKONTROLLE",
	} {
		if !seen[code] {
			t.Errorf("Vorschlagskatalog enthaelt %s nicht", code)
		}
	}
}

// Ein Aufrufer darf die Vorlage nicht veraendern koennen.
func TestDefaultQualificationTypesReturnsIndependentCopies(t *testing.T) {
	first := DefaultQualificationTypes()
	first[0].Name = "veraendert"
	*first[0].GueltigkeitMonate = 99

	second := DefaultQualificationTypes()
	if second[0].Name == "veraendert" || *second[0].GueltigkeitMonate == 99 {
		t.Fatalf("Aenderung an einer Kopie wirkt auf die Vorlage: %+v", second[0])
	}
}

// Fuehrerscheinkontrolle (ADR 0032 §5, Entscheidung des Nutzers vom
// 2026-10-25): Frist 6 Monate als Vorschlag, keine Kopie des
// Fuehrerscheins.
func TestDefaultQualificationTypesFuehrerscheinkontrolle(t *testing.T) {
	for _, d := range DefaultQualificationTypes() {
		if d.Code != "FUEHRERSCHEINKONTROLLE" {
			continue
		}
		if d.Kategorie != QualificationKategorieQualifikation || d.GueltigkeitMonate == nil || *d.GueltigkeitMonate != 6 || d.PflichtFuerAlle {
			t.Fatalf("Fuehrerscheinkontrolle: %+v", d)
		}
		if !strings.Contains(d.Hinweis, "keine Kopie des Führerscheins") || !strings.Contains(d.Hinweis, "§21 StVG") {
			t.Fatalf("Hinweis: %q", d.Hinweis)
		}
		return
	}
	t.Fatal("FUEHRERSCHEINKONTROLLE fehlt")
}
