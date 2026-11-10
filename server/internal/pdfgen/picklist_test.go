package pdfgen

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func pickPages(pdf []byte) int {
	return bytes.Count(pdf, []byte("/Type /Page\n")) + bytes.Count(pdf, []byte("/Type /Page\r\n"))
}

func TestRenderPickList(t *testing.T) {
	base := PickListData{Number: "FA-2026-0007", Project: "P-1 Testprojekt", Position: "3 Fenster (links)", Stueckzahl: 2,
		Liefertermin: "02.12.2026", Status: "in Arbeit", Lager: "HL – Halle", PrintedAt: "10.11.2026 08:00"}
	line := PickListLineData{Pos: 3, Material: "P-100 Blendrahmen\nmat-1 Profil", Abmessung: "L 1230,4 mm", Menge: "4,922 m", Plaetze: []string{"HL / R2: 8 m", "HL / R1: 5 m"}}

	one := base
	// Glas mit "×" und Material mit "–": Zeichen ausserhalb ASCII.
	glas := PickListLineData{Pos: 4, Material: "Isolierglas – 4/16/4", Abmessung: "1100 × 1350 mm", Menge: "2 (gesamt)"}
	one.Lines = []PickListLineData{line, glas}
	pdf, err := RenderPickList(one, "#112233", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"%PDF", "Kommissionierliste FA-2026-0007", "P-100 Blendrahmen", "HL / R2: 8 m", "4,922 m", "Liefertermin: 02.12.2026", "Lager: HL "} {
		if !bytes.Contains(pdf, []byte(want)) {
			t.Errorf("fehlt: %q", want)
		}
	}
	// Ohne Preise: kein Euro, kein Preis-Spaltenkopf.
	for _, bad := range []string{"EUR", "Preis", "Keine offenen Zeilen"} {
		if bytes.Contains(pdf, []byte(bad)) {
			t.Errorf("unerwartet: %q", bad)
		}
	}
	if n := pickPages(pdf); n != 1 {
		t.Fatalf("Seiten: %d", n)
	}

	// Leer: Hinweis statt Tabelle, ohne Termin und Lager.
	empty := base
	empty.Liefertermin, empty.Lager = "", ""
	pdf, err = RenderPickList(empty, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(pdf, []byte("Keine offenen Zeilen")) || bytes.Contains(pdf, []byte("Liefertermin")) ||
		!bytes.Contains(pdf, []byte("ohne Reservierung")) || bytes.Contains(pdf, []byte("Abmessung")) {
		t.Fatal("leere Liste falsch")
	}

	// Viele Zeilen: Umbruch mit Tabellenkopf auf jeder Seite.
	many := base
	for i := 0; i < 60; i++ {
		l := line
		l.Pos = i + 1
		l.Material = fmt.Sprintf("Material %02d", i+1)
		many.Lines = append(many.Lines, l)
	}
	pdf, err = RenderPickList(many, "", "")
	if err != nil {
		t.Fatal(err)
	}
	pages := pickPages(pdf)
	if pages < 2 || strings.Count(string(pdf), "(Abmessung)") != pages || !bytes.Contains(pdf, []byte("Material 60")) {
		t.Fatalf("Umbruch: %d Seiten, %d Tabellenköpfe", pages, strings.Count(string(pdf), "(Abmessung)"))
	}
}

func TestPickListNeedsBreak(t *testing.T) {
	// A4: 297 mm, Tabelle endet spaetestens bei 279 mm.
	for _, tc := range []struct {
		y, h float64
		want bool
	}{
		{269, 10, false},
		{269.5, 10, true},
		{279, 0, false},
		{30, 10, false},
		{275, 5.5, true},
	} {
		if got := pickListNeedsBreak(tc.y, tc.h, 297); got != tc.want {
			t.Errorf("y %v h %v: %v", tc.y, tc.h, got)
		}
	}
}

func TestPickListHead(t *testing.T) {
	got := strings.Join(pickListHead(PickListData{Stueckzahl: 1, Status: "freigegeben"}), "|")
	if got != "Stückzahl: 1    Status: freigegeben|Lager: ohne Reservierung - Plätze aus allen Lagern" {
		t.Fatalf("Kopf: %q", got)
	}
}
