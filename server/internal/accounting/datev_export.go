package accounting

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/charmap"
)

// DATEV EXTF Buchungsstapel, Format-Version 13 (ADR 0021, Backlog E.3).
// Feldreihenfolge/-typen/-formatierung sind gegen zwei unabhängige,
// deckungsgleiche Quellen recherchiert und verifiziert (siehe ADR 0021,
// Kontext) - keine erfundene Spezifikation. Dieser Teil (E.3.3.1) ist
// bewusst reine, DB-lose CSV-Bautechnik: Datenbeschaffung/Gegenkonto-
// Auflösung folgt in E.3.3.2, HTTP-Wiring in E.3.3.3.

// DatevHeaderInput sind die pro Export variablen Werte der 31-Felder-
// Kopfzeile. Alle Werte müssen vom Aufrufer bereits vollständig aufgelöst
// sein (z. B. WJ-Beginn als konkretes Datum, nicht nur ein Monat).
type DatevHeaderInput struct {
	Berater          int
	Mandant          int
	FiscalYearStart  time.Time
	SKR              string
	Sachkontenlaenge int
	DatumVon         time.Time
	DatumBis         time.Time
	Bezeichnung      string
	ExportiertVon    string
	ErzeugtAm        time.Time
}

// DatevBookingRow ist eine einzelne Buchungssatz-Zeile. Nur die Felder, die
// für unser Datenmodell eine Entsprechung haben, werden abgebildet (siehe
// ADR 0021, "Feldabdeckung der 125 Buchungssatz-Spalten") - die übrigen 117
// Spalten bleiben spezifikationskonform leer.
type DatevBookingRow struct {
	Umsatz       float64 // immer positiv; Vorzeichen kommt aus SollHaben
	SollHaben    string  // "S" oder "H"
	Konto        string
	Gegenkonto   string
	Belegdatum   time.Time
	Belegfeld1   string
	Buchungstext string
	Kost1        string
}

// datevBookingColumns ist die feste, offizielle Spaltennamen-Zeile für
// Formatkategorie 21 "Buchungsstapel" (125 Spalten, exakte Reihenfolge und
// Schreibweise laut Recherche in ADR 0021).
var datevBookingColumns = buildDatevBookingColumns()

func buildDatevBookingColumns() []string {
	cols := []string{
		"Umsatz (ohne Soll/Haben-Kz)",
		"Soll/Haben-Kennzeichen",
		"WKZ Umsatz",
		"Kurs",
		"Basisumsatz",
		"WKZ Basisumsatz",
		"Konto",
		"Gegenkonto (ohne BU-Schlüssel)",
		"BU-Schlüssel",
		"Belegdatum",
		"Belegfeld 1",
		"Belegfeld 2",
		"Skonto",
		"Buchungstext",
		"Postensperre",
		"Diverse Adressnummer",
		"Geschäftspartnerbank",
		"Sachverhalt",
		"Zinssperre",
		"Beleglink",
	}
	for i := 1; i <= 8; i++ {
		cols = append(cols,
			fmt.Sprintf("Beleginfo – Art %d", i),
			fmt.Sprintf("Beleginfo – Inhalt %d", i),
		)
	}
	cols = append(cols,
		"KOST1 – Kostenstelle",
		"KOST2 – Kostenstelle",
		"Kost Menge",
		"EU-Land u. USt-IdNr.",
		"EU-Steuersatz",
		"Abw. Versteuerungsart",
		"Sachverhalt L+L",
		"Funktionsergänzung L+L",
		"BU 49 Hauptfunktionstyp",
		"BU 49 Hauptfunktionsnummer",
		"BU 49 Funktionsergänzung",
	)
	for i := 1; i <= 20; i++ {
		cols = append(cols,
			fmt.Sprintf("Zusatzinformation – Art %d", i),
			fmt.Sprintf("Zusatzinformation – Inhalt %d", i),
		)
	}
	cols = append(cols,
		"Stück",
		"Gewicht",
		"Zahlweise",
		"Forderungsart",
		"Veranlagungsjahr",
		"Zugeordnete Fälligkeit",
		"Skontotyp",
		"Auftragsnummer",
		"Buchungstyp",
		"USt-Schlüssel (Anzahlungen)",
		"EU-Mitgliedstaat (Anzahlungen)",
		"Sachverhalt L+L (Anzahlungen)",
		"EU-Steuersatz (Anzahlungen)",
		"Erlöskonto (Anzahlungen)",
		"Herkunft-Kz",
		"Leerfeld",
		"KOST-Datum",
		"SEPA-Mandatsreferenz",
		"Skontosperre",
		"Gesellschaftername",
		"Beteiligtennummer",
		"Identifikationsnummer",
		"Zeichnernummer",
		"Postensperre bis",
		"Bezeichnung",
		"Kennzeichen",
		"Festschreibung",
		"Leistungsdatum",
		"Datum Zuord.",
		"Fälligkeit",
		"Generalumkehr",
		"Steuersatz",
		"Land",
		"Abrechnungsreferent",
		"BVV-Position",
		"EU-Mitgliedstaat u. UStID (Ursprung)",
		"EU-Steuersatz (Ursprung)",
		"Abw. Skontokonto",
	)
	return cols
}

// Zero-basierte Indizes der von uns befüllten Buchungssatz-Spalten
// (1-basierte Feldnummern laut ADR 0021: 1, 2, 7, 8, 10, 11, 14, 37).
const (
	datevColUmsatz       = 0
	datevColSollHaben    = 1
	datevColKonto        = 6
	datevColGegenkonto   = 7
	datevColBelegdatum   = 9
	datevColBelegfeld1   = 10
	datevColBuchungstext = 13
	datevColKost1        = 36
)

// BuildDatevBuchungsstapel erzeugt den vollständigen Inhalt einer DATEV-
// EXTF-Buchungsstapel-Datei (Kopfzeile + Spaltennamen-Zeile + Datenzeilen),
// Windows-1252-kodiert, wie in ADR 0021 festgelegt.
func BuildDatevBuchungsstapel(header DatevHeaderInput, rows []DatevBookingRow) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString(datevHeaderLine(header))
	sb.WriteString("\r\n")
	sb.WriteString(strings.Join(datevBookingColumns, ";"))
	sb.WriteString("\r\n")
	for _, row := range rows {
		sb.WriteString(datevBookingLine(row, header.Sachkontenlaenge))
		sb.WriteString("\r\n")
	}

	encoded, err := charmap.Windows1252.NewEncoder().String(sb.String())
	if err != nil {
		return nil, fmt.Errorf("DATEV-Export: Zeichenkodierung nach Windows-1252 fehlgeschlagen: %w", err)
	}
	return []byte(encoded), nil
}

func datevHeaderLine(h DatevHeaderInput) string {
	fields := []string{
		datevQuote("EXTF"),                      // 1 DATEV-Format-KZ
		"700",                                   // 2 Versionsnummer (Schnittstellen-Entwicklungsleitfaden)
		"21",                                    // 3 Datenkategorie (Buchungsstapel)
		datevQuote("Buchungsstapel"),            // 4 Formatname
		"13",                                    // 5 Formatversion
		h.ErzeugtAm.Format("20060102150405000"), // 6 Erzeugt am
		"",                                      // 7 Importiert (nur beim Import gesetzt)
		datevQuote("RE"),                        // 8 Herkunft (folgenlos, siehe ADR 0021)
		datevQuote(datevTruncate(h.ExportiertVon, 25)), // 9 Exportiert von
		"",                                           // 10 Importiert von (nur beim Import gesetzt)
		strconv.Itoa(h.Berater),                      // 11 Berater
		strconv.Itoa(h.Mandant),                      // 12 Mandant
		h.FiscalYearStart.Format("20060102"),         // 13 WJ-Beginn
		strconv.Itoa(h.Sachkontenlaenge),             // 14 Sachkontenlänge
		h.DatumVon.Format("20060102"),                // 15 Datum vom
		h.DatumBis.Format("20060102"),                // 16 Datum bis
		datevQuote(datevTruncate(h.Bezeichnung, 30)), // 17 Bezeichnung
		"",                // 18 Diktatkürzel
		"1",               // 19 Buchungstyp (1 = Finanzbuchhaltung)
		"",                // 20 Rechnungslegungszweck
		"0",               // 21 Festschreibung (immer 0, siehe ADR 0021)
		datevQuote("EUR"), // 22 WKZ
		"",                // 23 reserviert
		"",                // 24 Derivatskennzeichen
		"",                // 25 reserviert 2
		"",                // 26 reserviert 3
		datevQuote(h.SKR), // 27 SKR
		"",                // 28 Branchenlösung-Id
		"",                // 29 reserviert 4
		"",                // 30 reserviert 5
		"",                // 31 Anwendungsinformation
	}
	return strings.Join(fields, ";")
}

func datevBookingLine(row DatevBookingRow, sachkontenlaenge int) string {
	fields := make([]string, len(datevBookingColumns))

	fields[datevColUmsatz] = datevDecimal(absFloat(row.Umsatz))
	fields[datevColSollHaben] = datevQuote(row.SollHaben)
	fields[datevColKonto] = datevPadAccount(row.Konto, sachkontenlaenge)
	fields[datevColGegenkonto] = datevPadAccount(row.Gegenkonto, sachkontenlaenge)
	fields[datevColBelegdatum] = row.Belegdatum.Format("0201") // TTMM, ohne Jahr (siehe ADR 0021)
	fields[datevColBelegfeld1] = datevQuote(datevTruncate(row.Belegfeld1, 36))
	fields[datevColBuchungstext] = datevQuote(datevTruncate(row.Buchungstext, 60))
	fields[datevColKost1] = datevQuote(datevTruncate(row.Kost1, 36))

	return strings.Join(fields, ";")
}

func datevQuote(s string) string {
	if s == "" {
		return ""
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func datevTruncate(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit])
}

func datevDecimal(v float64) string {
	return strings.ReplaceAll(strconv.FormatFloat(v, 'f', 2, 64), ".", ",")
}

func datevPadAccount(code string, length int) string {
	if length <= 0 || len(code) >= length {
		return code
	}
	return strings.Repeat("0", length-len(code)) + code
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
