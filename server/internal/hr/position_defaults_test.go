package hr

import (
	"strings"
	"testing"
)

// Regeln des Stellen-Vorschlagskatalogs (siehe position_defaults.go).
func TestDefaultPositionsFollowTheCatalogRules(t *testing.T) {
	qualTypes := map[string]QualificationTypeInput{}
	for _, d := range DefaultQualificationTypes() {
		qualTypes[d.Code] = d
	}
	codes, names := map[string]bool{}, map[string]bool{}
	for _, p := range DefaultPositions() {
		if !qualificationCodePattern.MatchString(p.Code) {
			t.Fatalf("%s: ungueltiger Code", p.Code)
		}
		if codes[p.Code] || names[strings.ToLower(p.Name)] {
			t.Fatalf("%s: Code oder Name doppelt", p.Code)
		}
		codes[p.Code], names[strings.ToLower(p.Name)] = true, true
		if strings.TrimSpace(p.Name) == "" || p.Hinweis != hinweisStelleNichtRecherchiert {
			t.Fatalf("%s: Name fehlt oder Hinweis 'nicht recherchiert' fehlt", p.Code)
		}
		if len(p.TypeCodes) == 0 {
			t.Fatalf("%s: verlangt keine Art", p.Code)
		}
		seen := map[string]bool{}
		for _, c := range p.TypeCodes {
			qt, ok := qualTypes[c]
			if !ok {
				t.Fatalf("%s: Art %s steht nicht im Qualifikations-Vorschlagskatalog", p.Code, c)
			}
			if qt.PflichtFuerAlle {
				t.Fatalf("%s: Art %s ist ohnehin fuer alle Pflicht", p.Code, c)
			}
			if seen[c] {
				t.Fatalf("%s: Art %s doppelt", p.Code, c)
			}
			seen[c] = true
		}
	}
	// Fuenf aus ADR 0031, dazu "Fahrer" aus ADR 0032 §5.
	if len(codes) != 6 || !codes["FAHRER"] {
		t.Fatalf("erwartet 6 Stellen (ADR 0031, ADR 0032), bekommen %d", len(codes))
	}
}

func TestDefaultPositionsReturnsIndependentCopies(t *testing.T) {
	a := DefaultPositions()
	a[0].Name = "veraendert"
	a[0].TypeCodes[0] = "X"
	b := DefaultPositions()
	if b[0].Name == "veraendert" || b[0].TypeCodes[0] == "X" {
		t.Fatal("Aufrufer kann die Vorlage veraendern")
	}
}
