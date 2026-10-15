package assets

import (
	"math"
	"strings"
	"testing"
	"time"
)

// Fachregeln des Asset-Stamms ohne Datenbank (Backlog F.3.3).

func intPtr(n int) *int           { return &n }
func floatPtr(f float64) *float64 { return &f }
func strPtr(s string) *string     { return &s }
func datePtr(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func validInput() AssetInput {
	return AssetInput{Inventarnummer: "INV-1", Bezeichnung: "Bohrhammer", Kategorie: KategorieElektrowerkzeug}
}

func TestValidateAssetInput(t *testing.T) {
	cases := []struct {
		name    string
		mod     func(*AssetInput)
		wantErr string
	}{
		{"gueltig minimal", func(*AssetInput) {}, ""},
		{"gueltig vollstaendig", func(in *AssetInput) {
			in.Anschaffungspreis = floatPtr(0)
			in.PruefintervallMonate = intPtr(1)
			in.Anschaffungsdatum = datePtr(2026, 1, 1)
		}, ""},
		{"ohne Inventarnummer", func(in *AssetInput) { in.Inventarnummer = "" }, "Inventarnummer erforderlich"},
		{"Inventarnummer 64 Zeichen", func(in *AssetInput) { in.Inventarnummer = strings.Repeat("ä", 64) }, ""},
		{"Inventarnummer 65 Zeichen", func(in *AssetInput) { in.Inventarnummer = strings.Repeat("ä", 65) }, "höchstens 64"},
		{"ohne Bezeichnung", func(in *AssetInput) { in.Bezeichnung = "" }, "Bezeichnung erforderlich"},
		{"Fahrzeug ist keine Kategorie", func(in *AssetInput) { in.Kategorie = "fahrzeug" }, "Ungültige Kategorie"},
		{"Kategorie leer", func(in *AssetInput) { in.Kategorie = "" }, "Ungültige Kategorie"},
		{"Preis negativ", func(in *AssetInput) { in.Anschaffungspreis = floatPtr(-0.01) }, "nicht negativ"},
		{"Preis NaN", func(in *AssetInput) { in.Anschaffungspreis = floatPtr(math.NaN()) }, "nicht negativ"},
		{"Preis unendlich", func(in *AssetInput) { in.Anschaffungspreis = floatPtr(math.Inf(1)) }, "nicht negativ"},
		{"Intervall 0", func(in *AssetInput) { in.PruefintervallMonate = intPtr(0) }, "größer als 0"},
		{"Intervall negativ", func(in *AssetInput) { in.PruefintervallMonate = intPtr(-6) }, "größer als 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := validInput()
			tc.mod(&in)
			err := ValidateAssetInput(in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unerwarteter Fehler: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("erwartet Fehler mit %q, bekommen %v", tc.wantErr, err)
			}
		})
	}
}

func TestEveryKategorieIsAccepted(t *testing.T) {
	// Muss mit dem CHECK in Migration 089 uebereinstimmen.
	for _, k := range []string{"werkzeug", "elektrowerkzeug", "messmittel", "leiter", "psa", "it", "maschine", "sonstiges"} {
		in := validInput()
		in.Kategorie = k
		if err := ValidateAssetInput(in); err != nil {
			t.Fatalf("Kategorie %q abgelehnt: %v", k, err)
		}
	}
	if len(kategorien) != 8 {
		t.Fatalf("erwartet 8 Kategorien wie in Migration 089, bekommen %d", len(kategorien))
	}
}

func TestNormalizeAssetInput(t *testing.T) {
	in := normalizeAssetInput(AssetInput{
		Inventarnummer: "  inv-7 ", Bezeichnung: " Leiter ", Kategorie: " leiter ",
		Seriennummer: " S1 ", Hersteller: " H ", Modell: " M ", Standort: " Lager ",
		KostenstelleID:    strPtr("   "),
		Anschaffungsdatum: func() *time.Time { t := time.Date(2026, 3, 4, 15, 30, 0, 0, time.FixedZone("x", 3600)); return &t }(),
	})
	// Inventarnummer wird nur getrimmt, NICHT in Grossbuchstaben gewandelt.
	if in.Inventarnummer != "inv-7" || in.Bezeichnung != "Leiter" || in.Kategorie != "leiter" ||
		in.Seriennummer != "S1" || in.Hersteller != "H" || in.Modell != "M" || in.Standort != "Lager" {
		t.Fatalf("nicht normalisiert: %+v", in)
	}
	if in.KostenstelleID != nil {
		t.Fatalf("leere Kostenstelle muss nil werden, bekommen %q", *in.KostenstelleID)
	}
	if !in.Anschaffungsdatum.Equal(time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("Anschaffungsdatum nicht auf den Kalendertag gekuerzt: %v", in.Anschaffungsdatum)
	}
	k := normalizeAssetInput(AssetInput{KostenstelleID: strPtr(" cc-1 ")})
	if k.KostenstelleID == nil || *k.KostenstelleID != "cc-1" {
		t.Fatalf("Kostenstelle nicht getrimmt: %v", k.KostenstelleID)
	}
}

func TestValidateRetirement(t *testing.T) {
	today := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		datum       time.Time
		grund       string
		anschaffung *time.Time
		wantErr     string
	}{
		{"heute", today, "defekt", nil, ""},
		{"heute, spaet am Tag in anderer Zone", time.Date(2026, 10, 15, 23, 0, 0, 0, time.FixedZone("x", 7200)), "defekt", nil, ""},
		{"gestern", today.AddDate(0, 0, -1), "defekt", nil, ""},
		{"morgen", today.AddDate(0, 0, 1), "defekt", nil, "Zukunft"},
		{"ohne Datum", time.Time{}, "defekt", nil, "Ausmusterungsdatum erforderlich"},
		{"ohne Grund", today, "  ", nil, "Grund"},
		{"am Anschaffungstag", today, "defekt", datePtr(2026, 10, 15), ""},
		{"vor Anschaffung", today.AddDate(0, 0, -1), "defekt", datePtr(2026, 10, 15), "vor dem Anschaffungsdatum"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRetirement(tc.datum, tc.grund, tc.anschaffung, today)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unerwarteter Fehler: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("erwartet Fehler mit %q, bekommen %v", tc.wantErr, err)
			}
		})
	}
}
