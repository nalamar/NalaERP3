package fleet

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
func datePtr(y int, m time.Month, d int) *time.Time {
	t := day(y, m, d)
	return &t
}
func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func TestNormalizeKennzeichen(t *testing.T) {
	for in, want := range map[string]string{
		"m-ab 1234":        "M-AB 1234",
		"  m - ab  1234  ": "M-AB 1234",
		"m -ab 1234 e":     "M-AB 1234 E",
		"bad-ö 7":          "BAD-Ö 7",
		"":                 "",
	} {
		if got := NormalizeKennzeichen(in); got != want {
			t.Errorf("%q: %q, erwartet %q", in, got, want)
		}
	}
}

func TestValidateVehicleInput(t *testing.T) {
	today := day(2026, 10, 25)
	valid := VehicleInput{Kennzeichen: "M-AB 1234", Art: ArtTransporter, FIN: strPtr("WVWZZZ1JZ3W386752"), Erstzulassung: datePtr(2020, 5, 1), ZulaessigeGesamtmasseKg: intPtr(3500)}
	with := func(mod func(*VehicleInput)) VehicleInput {
		in := valid
		mod(&in)
		return in
	}
	if err := ValidateVehicleInput(valid, today); err != nil {
		t.Fatalf("gueltig abgelehnt: %v", err)
	}
	if err := ValidateVehicleInput(with(func(in *VehicleInput) {
		in.FIN = nil
		in.Erstzulassung = &today
		in.ZulaessigeGesamtmasseKg = nil
		in.Art = ArtAnhaenger
	}), today); err != nil {
		t.Fatalf("ohne Optionales, Erstzulassung heute: %v", err)
	}
	for _, tc := range []struct {
		name string
		in   VehicleInput
		want string
	}{
		{"Kennzeichen fehlt", with(func(in *VehicleInput) { in.Kennzeichen = "" }), "Kennzeichen erforderlich"},
		{"Kennzeichen zu lang", with(func(in *VehicleInput) { in.Kennzeichen = "ABC-DEF 1234567" + "8" }), "Ungültiges Kennzeichen (Buchstaben, Ziffern, Bindestrich und Leerzeichen, höchstens 15 Zeichen)"},
		{"Kennzeichen Sonderzeichen", with(func(in *VehicleInput) { in.Kennzeichen = "M-AB_1234" }), "Ungültiges Kennzeichen (Buchstaben, Ziffern, Bindestrich und Leerzeichen, höchstens 15 Zeichen)"},
		{"Kennzeichen kleingeschrieben (nicht normalisiert)", with(func(in *VehicleInput) { in.Kennzeichen = "m-ab 1" }), "Ungültiges Kennzeichen (Buchstaben, Ziffern, Bindestrich und Leerzeichen, höchstens 15 Zeichen)"},
		{"Kennzeichen ein Zeichen", with(func(in *VehicleInput) { in.Kennzeichen = "M" }), "Ungültiges Kennzeichen (Buchstaben, Ziffern, Bindestrich und Leerzeichen, höchstens 15 Zeichen)"},
		{"Stapler ist keine Fahrzeugart", with(func(in *VehicleInput) { in.Art = "stapler" }), "Ungültige Fahrzeugart"},
		{"FIN zu kurz", with(func(in *VehicleInput) { in.FIN = strPtr("WVWZZZ1JZ3W38675") }), "Ungültige FIN (17 Zeichen, Ziffern und Buchstaben ohne I, O, Q)"},
		{"FIN mit O", with(func(in *VehicleInput) { in.FIN = strPtr("WVWZZZ1JZ3W38675O") }), "Ungültige FIN (17 Zeichen, Ziffern und Buchstaben ohne I, O, Q)"},
		{"FIN mit I", with(func(in *VehicleInput) { in.FIN = strPtr("IVWZZZ1JZ3W386752") }), "Ungültige FIN (17 Zeichen, Ziffern und Buchstaben ohne I, O, Q)"},
		{"FIN mit Q", with(func(in *VehicleInput) { in.FIN = strPtr("QVWZZZ1JZ3W386752") }), "Ungültige FIN (17 Zeichen, Ziffern und Buchstaben ohne I, O, Q)"},
		{"Erstzulassung morgen", with(func(in *VehicleInput) { in.Erstzulassung = datePtr(2026, 10, 26) }), "Erstzulassung darf nicht in der Zukunft liegen"},
		{"Masse 0", with(func(in *VehicleInput) { in.ZulaessigeGesamtmasseKg = intPtr(0) }), "Zulässige Gesamtmasse muss größer als 0 sein"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateVehicleInput(tc.in, today); err == nil || err.Error() != tc.want {
				t.Fatalf("erwartet %q, bekommen %v", tc.want, err)
			}
		})
	}
}

func TestNormalizeVehicleInput(t *testing.T) {
	got := normalizeVehicleInput(VehicleInput{
		Kennzeichen: " m-ab 1 ", Art: " pkw ", Bezeichnung: " Caddy ", FIN: strPtr(" wvwzzz1jz3w386752 "), KostenstelleID: strPtr("  "),
		Erstzulassung: func() *time.Time { t := time.Date(2020, 5, 1, 15, 30, 0, 0, time.UTC); return &t }(),
	})
	if got.Kennzeichen != "M-AB 1" || got.Art != "pkw" || got.Bezeichnung != "Caddy" || got.FIN == nil || *got.FIN != "WVWZZZ1JZ3W386752" ||
		got.KostenstelleID != nil || !got.Erstzulassung.Equal(day(2020, 5, 1)) {
		t.Fatalf("normalisiert: %+v", got)
	}
	if n := normalizeVehicleInput(VehicleInput{FIN: strPtr(" ")}); n.FIN != nil {
		t.Fatalf("leere FIN nicht entfernt: %v", *n.FIN)
	}
}

func TestValidateDeregistration(t *testing.T) {
	today := day(2026, 10, 25)
	if err := ValidateDeregistration(today, "verkauft", datePtr(2020, 1, 1), today); err != nil {
		t.Fatalf("gueltig abgelehnt: %v", err)
	}
	if err := ValidateDeregistration(day(2020, 1, 1), "verkauft", datePtr(2020, 1, 1), today); err != nil {
		t.Fatalf("am Tag der Erstzulassung abgelehnt: %v", err)
	}
	for _, tc := range []struct {
		name  string
		datum time.Time
		grund string
		ez    *time.Time
		want  string
	}{
		{"ohne Datum", time.Time{}, "x", nil, "Abmeldedatum erforderlich"},
		{"ohne Grund", today, "  ", nil, "Grund der Abmeldung erforderlich"},
		{"in der Zukunft", day(2026, 10, 26), "x", nil, "Abmeldedatum darf nicht in der Zukunft liegen"},
		{"vor Erstzulassung", day(2019, 12, 31), "x", datePtr(2020, 1, 1), "Abmeldedatum darf nicht vor der Erstzulassung liegen"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateDeregistration(tc.datum, tc.grund, tc.ez, today); err == nil || err.Error() != tc.want {
				t.Fatalf("erwartet %q, bekommen %v", tc.want, err)
			}
		})
	}
}

// Anlegen, Aendern und Abmelden pruefen Mandant und Eingaben, bevor sie
// Postgres anfassen (nil-Pool).
func TestVehicleServiceValidatesBeforeTouchingDatabase(t *testing.T) {
	s := NewVehicleService(nil)
	ctx := context.Background()
	valid := VehicleInput{Kennzeichen: "M-AB 1", Art: ArtPkw}
	if _, err := s.Create(ctx, valid, " ", ""); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("Create ohne Mandant: %v", err)
	}
	if _, err := s.Create(ctx, VehicleInput{Kennzeichen: "M-AB 1", Art: "bagger"}, "c1", ""); err == nil || err.Error() != "Ungültige Fahrzeugart" {
		t.Fatalf("Create ungueltig: %v", err)
	}
	if _, err := s.Update(ctx, uuid.New(), valid, ""); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("Update ohne Mandant: %v", err)
	}
	if _, err := s.Update(ctx, uuid.New(), VehicleInput{Art: ArtPkw}, "c1"); err == nil || err.Error() != "Kennzeichen erforderlich" {
		t.Fatalf("Update ungueltig: %v", err)
	}
	if _, err := s.Deregister(ctx, uuid.New(), day(2026, 1, 1), "x", ""); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("Deregister ohne Mandant: %v", err)
	}
	if _, err := s.List(ctx, "c1", ListFilter{Art: "stapler"}); err == nil || err.Error() != "Ungültige Fahrzeugart" {
		t.Fatalf("List ungueltige Art: %v", err)
	}
}
