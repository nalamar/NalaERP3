package materials

import (
	"context"
	"testing"
)

func TestCreateRejectsMissingCompanyID(t *testing.T) {
	svc := NewService(nil, nil, "")

	got, err := svc.Create(context.Background(), MaterialCreate{Nummer: "M-1", Bezeichnung: "Test"}, "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil material, got %#v", got)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

func TestCreateRejectsMissingRequiredFields(t *testing.T) {
	svc := NewService(nil, nil, "")

	got, err := svc.Create(context.Background(), MaterialCreate{}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil material, got %#v", got)
	}
	if err.Error() != "Nummer und Bezeichnung sind erforderlich" {
		t.Fatalf("expected Nummer und Bezeichnung sind erforderlich, got %q", err.Error())
	}
}

func TestCreateRejectsNegativeMindestbestand(t *testing.T) {
	svc := NewService(nil, nil, "")
	negative := -1.0

	got, err := svc.Create(context.Background(), MaterialCreate{Nummer: "M-1", Bezeichnung: "Test", MindestBestand: &negative}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if got != nil {
		t.Fatalf("expected nil material, got %#v", got)
	}
	if err.Error() != "Mindestbestand darf nicht negativ sein" {
		t.Fatalf("expected Mindestbestand darf nicht negativ sein, got %q", err.Error())
	}
}

func TestUpdateRejectsMissingID(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.Update(context.Background(), "", MaterialUpdate{}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestUpdateRejectsEmptyNummer(t *testing.T) {
	svc := NewService(nil, nil, "")
	empty := ""

	_, err := svc.Update(context.Background(), "mat-1", MaterialUpdate{Nummer: &empty}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Nummer erforderlich" {
		t.Fatalf("expected Nummer erforderlich, got %q", err.Error())
	}
}

func TestUpdateRejectsEmptyBezeichnung(t *testing.T) {
	svc := NewService(nil, nil, "")
	empty := ""

	_, err := svc.Update(context.Background(), "mat-1", MaterialUpdate{Bezeichnung: &empty}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Bezeichnung erforderlich" {
		t.Fatalf("expected Bezeichnung erforderlich, got %q", err.Error())
	}
}

func TestUpdateRejectsEmptyEinheit(t *testing.T) {
	svc := NewService(nil, nil, "")
	empty := ""

	_, err := svc.Update(context.Background(), "mat-1", MaterialUpdate{Einheit: &empty}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Einheit erforderlich" {
		t.Fatalf("expected Einheit erforderlich, got %q", err.Error())
	}
}

func TestDeleteSoftRejectsMissingID(t *testing.T) {
	svc := NewService(nil, nil, "")

	err := svc.DeleteSoft(context.Background(), "", "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "ID erforderlich" {
		t.Fatalf("expected ID erforderlich, got %q", err.Error())
	}
}

func TestNormalizeAndValidateRCKlasseAcceptsKnownClassesCaseInsensitive(t *testing.T) {
	cases := map[string]string{
		"RC1":   "RC1",
		"rc1n":  "RC1N",
		" Rc2 ": "RC2",
		"rc2n":  "RC2N",
		"RC3":   "RC3",
		"rc4":   "RC4",
		"Rc5":   "RC5",
		"RC6":   "RC6",
	}
	for in, want := range cases {
		got, err := normalizeAndValidateRCKlasse(in)
		if err != nil {
			t.Fatalf("normalizeAndValidateRCKlasse(%q): unexpected error %v", in, err)
		}
		if got != want {
			t.Fatalf("normalizeAndValidateRCKlasse(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeAndValidateRCKlasseAllowsEmpty(t *testing.T) {
	got, err := normalizeAndValidateRCKlasse("   ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty result, got %q", got)
	}
}

func TestNormalizeAndValidateRCKlasseRejectsUnknownClass(t *testing.T) {
	_, err := normalizeAndValidateRCKlasse("RC7")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültige RC-Klasse (gültig: RC1, RC1N, RC2, RC2N, RC3, RC4, RC5, RC6)" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestValidateUWertAllowsNilAndPositive(t *testing.T) {
	if err := validateUWert(nil); err != nil {
		t.Fatalf("unexpected error for nil: %v", err)
	}
	positive := 1.3
	if err := validateUWert(&positive); err != nil {
		t.Fatalf("unexpected error for positive value: %v", err)
	}
}

func TestValidateUWertRejectsZeroAndNegative(t *testing.T) {
	for _, v := range []float64{0, -0.5} {
		v := v
		if err := validateUWert(&v); err == nil {
			t.Fatalf("expected validation error for %v, got nil", v)
		} else if err.Error() != "U-Wert muss größer als 0 sein" {
			t.Fatalf("unexpected error message for %v: %q", v, err.Error())
		}
	}
}

func TestCreateRejectsInvalidRCKlasse(t *testing.T) {
	svc := NewService(nil, nil, "")

	_, err := svc.Create(context.Background(), MaterialCreate{Nummer: "M-1", Bezeichnung: "Test", RCKlasse: "RC9"}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültige RC-Klasse (gültig: RC1, RC1N, RC2, RC2N, RC3, RC4, RC5, RC6)" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestCreateRejectsNonPositiveUWert(t *testing.T) {
	svc := NewService(nil, nil, "")
	uWert := 0.0

	_, err := svc.Create(context.Background(), MaterialCreate{Nummer: "M-1", Bezeichnung: "Test", UWert: &uWert}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "U-Wert muss größer als 0 sein" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestUpdateRejectsInvalidRCKlasse(t *testing.T) {
	svc := NewService(nil, nil, "")
	invalid := "RC9"

	_, err := svc.Update(context.Background(), "mat-1", MaterialUpdate{RCKlasse: &invalid}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "Ungültige RC-Klasse (gültig: RC1, RC1N, RC2, RC2N, RC3, RC4, RC5, RC6)" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestUpdateRejectsNonPositiveUWert(t *testing.T) {
	svc := NewService(nil, nil, "")
	uWert := -2.0

	_, err := svc.Update(context.Background(), "mat-1", MaterialUpdate{UWert: &uWert}, "default")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "U-Wert muss größer als 0 sein" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}
