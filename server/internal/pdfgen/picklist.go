package pdfgen

import (
	"bytes"
	"fmt"
	"strings"

	gofpdf "github.com/jung-kurt/gofpdf"
)

// Kommissionierliste je Fertigungsauftrag (ADR 0038 Frage 4 (a), Backlog
// H.3.3b): internes Werkstattpapier ohne Briefkopf und ohne Preise, nur
// die Farben des Brandings. Unkomprimiert, damit Tests den Text finden.

type PickListData struct {
	Number       string
	Project      string
	Position     string
	Stueckzahl   int
	Liefertermin string // formatiert, leer ohne Termin
	Status       string
	Lager        string // leer ohne Reservierungslager
	PrintedAt    string
	Lines        []PickListLineData
}

type PickListLineData struct {
	Pos       int
	Material  string // mehrzeilig erlaubt
	Abmessung string
	Menge     string // mit Einheit
	Plaetze   []string
}

var pickListCols = []float64{10, 58, 30, 28, 50, 14}
var pickListHeaders = []string{"Pos", "Material", "Abmessung", "Menge", "Lager / Platz", "entn."}

const pickListLineH = 4.5

func RenderPickList(data PickListData, primaryColor, accentColor string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetTitle("Kommissionierliste "+data.Number, true)
	pdf.SetAuthor("NalaERP3", false)
	pdf.SetMargins(10, 12, 10)
	pdf.SetAutoPageBreak(false, 15)
	tr := pdf.UnicodeTranslatorFromDescriptor("cp1252")
	primary, accent := templateColors(TemplateOptions{PrimaryColor: primaryColor, AccentColor: accentColor})

	pdf.SetFooterFunc(func() {
		_, h := pdf.GetPageSize()
		pdf.SetY(h - 12)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(accent.R, accent.G, accent.B)
		pdf.CellFormat(0, 5, tr(fmt.Sprintf("Kommissionierliste %s · gedruckt %s · Seite %d", data.Number, data.PrintedAt, pdf.PageNo())), "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	})

	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetTextColor(primary.R, primary.G, primary.B)
	pdf.CellFormat(0, 7, tr("Kommissionierliste "+data.Number), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(0, 0, 0)
	for _, row := range pickListHead(data) {
		pdf.CellFormat(0, 5.5, tr(row), "", 1, "L", false, 0, "")
	}
	pdf.Ln(2)
	drawSectionRule(pdf, accent)
	pdf.Ln(2)

	if len(data.Lines) == 0 {
		pdf.SetFont("Helvetica", "", 10)
		pdf.CellFormat(0, 6, tr("Keine offenen Zeilen - alles entnommen."), "", 1, "L", false, 0, "")
	} else {
		pickListTableHeader(pdf, tr, primary, accent)
		_, pageH := pdf.GetPageSize()
		for _, l := range data.Lines {
			cells := []string{fmt.Sprintf("%d", l.Pos), l.Material, l.Abmessung, l.Menge, strings.Join(l.Plaetze, "\n"), ""}
			lines := 1
			for i, c := range cells {
				// SplitLines auf den cp1252-Bytes: SplitText erwartet UTF-8
				// und stuerzt bei Zeichen ueber 255 ab (z. B. "–").
				if n := len(pdf.SplitLines([]byte(tr(c)), pickListCols[i]-2)); n > lines {
					lines = n
				}
			}
			h := float64(lines)*pickListLineH + 1
			if pickListNeedsBreak(pdf.GetY(), h, pageH) {
				pdf.AddPage()
				pickListTableHeader(pdf, tr, primary, accent)
			}
			x, y := pdf.GetXY()
			pdf.SetFont("Helvetica", "", 9)
			for i, c := range cells {
				pdf.Rect(x, y, pickListCols[i], h, "D")
				pdf.SetXY(x+1, y+0.5)
				align := "L"
				if i == 0 || i == 3 {
					align = "R"
				}
				pdf.MultiCell(pickListCols[i]-2, pickListLineH, tr(c), "", align, false)
				x += pickListCols[i]
			}
			// Kaestchen zum Abhaken.
			pdf.Rect(x-pickListCols[5]/2-2.5, y+1, 5, 5, "D")
			pdf.SetXY(10, y+h)
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// pickListBottomMM: Platz unten fuer die Fusszeile (bei h - 12).
const pickListBottomMM = 18

// pickListNeedsBreak: eine Zeile der Hoehe h ab y passt nicht mehr ueber
// die Fusszeile.
func pickListNeedsBreak(y, h, pageH float64) bool {
	return y+h > pageH-pickListBottomMM
}

// pickListHead: Kopfzeilen ohne leere Angaben.
func pickListHead(d PickListData) []string {
	rows := []string{}
	if d.Project != "" {
		rows = append(rows, "Projekt: "+d.Project)
	}
	if d.Position != "" {
		rows = append(rows, "Position: "+d.Position)
	}
	line := fmt.Sprintf("Stückzahl: %d    Status: %s", d.Stueckzahl, d.Status)
	if d.Liefertermin != "" {
		line += "    Liefertermin: " + d.Liefertermin
	}
	rows = append(rows, line)
	if d.Lager != "" {
		rows = append(rows, "Lager: "+d.Lager)
	} else {
		rows = append(rows, "Lager: ohne Reservierung - Plätze aus allen Lagern")
	}
	return rows
}

func pickListTableHeader(pdf *gofpdf.Fpdf, tr func(string) string, primary, accent rgbColor) {
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(primary.R, primary.G, primary.B)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetDrawColor(accent.R, accent.G, accent.B)
	for i, h := range pickListHeaders {
		pdf.CellFormat(pickListCols[i], 7, tr(h), "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetTextColor(0, 0, 0)
}
