package accounting

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/charmap"
)

func decodeWindows1252(t *testing.T, b []byte) string {
	t.Helper()
	s, err := charmap.Windows1252.NewDecoder().Bytes(b)
	if err != nil {
		t.Fatalf("decode windows-1252: %v", err)
	}
	return string(s)
}

func TestBuildDatevBuchungsstapelHeaderLine(t *testing.T) {
	header := DatevHeaderInput{
		Berater:          1001,
		Mandant:          456,
		FiscalYearStart:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		SKR:              "04",
		Sachkontenlaenge: 4,
		DatumVon:         time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		DatumBis:         time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC),
		Bezeichnung:      "Februar 2026",
		ExportiertVon:    "NalaERP3",
		ErzeugtAm:        time.Date(2026, 3, 6, 10, 25, 0, 0, time.UTC),
	}

	out, err := BuildDatevBuchungsstapel(header, nil)
	if err != nil {
		t.Fatalf("BuildDatevBuchungsstapel: %v", err)
	}
	decoded := decodeWindows1252(t, out)
	lines := strings.Split(decoded, "\r\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least header+column line, got %d lines", len(lines))
	}
	headerFields := strings.Split(lines[0], ";")
	if len(headerFields) != 31 {
		t.Fatalf("expected 31 header fields, got %d: %v", len(headerFields), headerFields)
	}

	want := map[int]string{
		0:  `"EXTF"`,
		1:  "700",
		2:  "21",
		3:  `"Buchungsstapel"`,
		4:  "13",
		5:  "20260306102500000",
		6:  "",
		7:  `"RE"`,
		8:  `"NalaERP3"`,
		9:  "",
		10: "1001",
		11: "456",
		12: "20260101",
		13: "4",
		14: "20260201",
		15: "20260228",
		16: `"Februar 2026"`,
		17: "",
		18: "1",
		19: "",
		20: "0",
		21: `"EUR"`,
		26: `"04"`,
	}
	for idx, expected := range want {
		if headerFields[idx] != expected {
			t.Fatalf("header field %d: expected %q, got %q (full: %v)", idx+1, expected, headerFields[idx], headerFields)
		}
	}
}

func TestBuildDatevBuchungsstapelColumnLine(t *testing.T) {
	out, err := BuildDatevBuchungsstapel(DatevHeaderInput{Sachkontenlaenge: 4}, nil)
	if err != nil {
		t.Fatalf("BuildDatevBuchungsstapel: %v", err)
	}
	decoded := decodeWindows1252(t, out)
	lines := strings.Split(decoded, "\r\n")
	columnFields := strings.Split(lines[1], ";")
	if len(columnFields) != 125 {
		t.Fatalf("expected 125 booking columns, got %d", len(columnFields))
	}
	wantNames := map[int]string{
		0:   "Umsatz (ohne Soll/Haben-Kz)",
		1:   "Soll/Haben-Kennzeichen",
		6:   "Konto",
		7:   "Gegenkonto (ohne BU-Schlüssel)",
		9:   "Belegdatum",
		10:  "Belegfeld 1",
		13:  "Buchungstext",
		36:  "KOST1 – Kostenstelle",
		37:  "KOST2 – Kostenstelle",
		124: "Abw. Skontokonto",
	}
	for idx, expected := range wantNames {
		if columnFields[idx] != expected {
			t.Fatalf("column %d: expected %q, got %q", idx+1, expected, columnFields[idx])
		}
	}
}

func TestBuildDatevBuchungsstapelBookingLine(t *testing.T) {
	header := DatevHeaderInput{Sachkontenlaenge: 4}
	rows := []DatevBookingRow{
		{
			Umsatz:       1234.5,
			SollHaben:    "S",
			Konto:        "3400",
			Gegenkonto:   "1200",
			Belegdatum:   time.Date(2026, 2, 21, 0, 0, 0, 0, time.UTC),
			Belegfeld1:   "RE-2026-0001",
			Buchungstext: `Wareneingang "Profile"`,
			Kost1:        "WERK-01",
		},
	}
	out, err := BuildDatevBuchungsstapel(header, rows)
	if err != nil {
		t.Fatalf("BuildDatevBuchungsstapel: %v", err)
	}
	decoded := decodeWindows1252(t, out)
	lines := strings.Split(decoded, "\r\n")
	if len(lines) < 4 {
		t.Fatalf("expected header+columns+1 booking row+trailing empty, got %d lines: %v", len(lines), lines)
	}
	bookingFields := strings.Split(lines[2], ";")
	if len(bookingFields) != 125 {
		t.Fatalf("expected 125 fields in booking row, got %d", len(bookingFields))
	}

	want := map[int]string{
		0:  "1234,50",
		1:  `"S"`,
		6:  "3400",
		7:  "1200",
		9:  "2102",
		10: `"RE-2026-0001"`,
		13: `"Wareneingang ""Profile"""`,
		36: `"WERK-01"`,
	}
	for idx, expected := range want {
		if bookingFields[idx] != expected {
			t.Fatalf("booking field %d: expected %q, got %q", idx+1, expected, bookingFields[idx])
		}
	}
	// Alle nicht befuellten Spalten muessen leer bleiben.
	for idx, v := range bookingFields {
		if _, populated := want[idx]; !populated && v != "" {
			t.Fatalf("expected column %d to stay empty, got %q", idx+1, v)
		}
	}
}

func TestBuildDatevBuchungsstapelUmsatzImmerPositiv(t *testing.T) {
	header := DatevHeaderInput{Sachkontenlaenge: 4}
	rows := []DatevBookingRow{
		{Umsatz: -50, SollHaben: "H", Konto: "1200", Gegenkonto: "3400", Belegdatum: time.Now(), Buchungstext: "Test"},
	}
	out, err := BuildDatevBuchungsstapel(header, rows)
	if err != nil {
		t.Fatalf("BuildDatevBuchungsstapel: %v", err)
	}
	decoded := decodeWindows1252(t, out)
	lines := strings.Split(decoded, "\r\n")
	bookingFields := strings.Split(lines[2], ";")
	if bookingFields[datevColUmsatz] != "50,00" {
		t.Fatalf("expected Umsatz always positive (50,00), got %q", bookingFields[datevColUmsatz])
	}
}

func TestDatevPadAccount(t *testing.T) {
	cases := []struct {
		code   string
		length int
		want   string
	}{
		{"1200", 4, "1200"},
		{"12", 4, "0012"},
		{"123456", 4, "123456"}, // laenger als Sachkontenlaenge: unveraendert, keine Kuerzung
		{"1200", 0, "1200"},
	}
	for _, c := range cases {
		if got := datevPadAccount(c.code, c.length); got != c.want {
			t.Fatalf("datevPadAccount(%q, %d) = %q, want %q", c.code, c.length, got, c.want)
		}
	}
}

func TestDatevTruncateRespectsRuneBoundaries(t *testing.T) {
	// "ü" ist ein Mehrbyte-Rune in UTF-8 - eine byte-basierte Kuerzung
	// wuerde ihn mitten im Zeichen zerschneiden.
	in := "Wareneingang Profilstück"
	out := datevTruncate(in, 20)
	if len([]rune(out)) != 20 {
		t.Fatalf("expected 20 runes, got %d (%q)", len([]rune(out)), out)
	}
}

func TestBuildDatevBuchungsstapelEmptyRowsProducesTrailingEmptyLine(t *testing.T) {
	out, err := BuildDatevBuchungsstapel(DatevHeaderInput{Sachkontenlaenge: 4}, nil)
	if err != nil {
		t.Fatalf("BuildDatevBuchungsstapel: %v", err)
	}
	decoded := decodeWindows1252(t, out)
	if !strings.HasSuffix(decoded, "\r\n") {
		t.Fatalf("expected file to end with CRLF, got %q", decoded[len(decoded)-10:])
	}
}
