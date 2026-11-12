package production

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Fachregeln der Montagetermine ohne Datenbank (Backlog H.3.6a).

func TestAppointmentInput(t *testing.T) {
	order := uuid.New()
	von := time.Date(2026, 12, 7, 15, 30, 0, 0, time.UTC)
	in := AppointmentInput{OrderID: &order, Von: &von, Bis: dayPtr("2026-12-09"), Baustelle: "  Musterstr. 1 ", Notiz: " Kran "}.Normalize()
	if in.Baustelle != "Musterstr. 1" || in.Notiz != "Kran" || !in.Von.Equal(day("2026-12-07")) || in.Validate() != nil {
		t.Fatalf("Normalize: %+v %v", in, in.Validate())
	}
	if von.Hour() != 15 {
		t.Fatal("Normalize hat die Eingabe veraendert")
	}
	if out := (AppointmentInput{}).Normalize(); out.Von != nil || out.Bis != nil {
		t.Fatalf("ohne Tage: %+v", out)
	}
	nilID := uuid.Nil
	base := func(f func(*AppointmentInput)) AppointmentInput {
		x := AppointmentInput{OrderID: &order, Von: dayPtr("2026-12-07"), Bis: dayPtr("2026-12-07")}
		f(&x)
		return x
	}
	for _, c := range []struct {
		name string
		in   AppointmentInput
		want error
	}{
		{"ein Tag", base(func(x *AppointmentInput) {}), nil},
		{"60 Tage", base(func(x *AppointmentInput) { x.Bis = dayPtr("2027-02-04") }), nil},
		{"Grenzen Text", base(func(x *AppointmentInput) {
			x.Baustelle, x.Notiz = strings.Repeat("ä", 300), strings.Repeat("ö", 500)
		}), nil},
		{"mit Team und Fahrzeug", base(func(x *AppointmentInput) { x.TeamID, x.VehicleID = &order, &order }), nil},
		{"ohne Auftrag", base(func(x *AppointmentInput) { x.OrderID = nil }), errMontageAuftrag},
		{"Auftrag leer", base(func(x *AppointmentInput) { x.OrderID = &nilID }), errMontageAuftrag},
		{"ohne von", base(func(x *AppointmentInput) { x.Von = nil }), errMontageDatum},
		{"ohne bis", base(func(x *AppointmentInput) { x.Bis = nil }), errMontageDatum},
		{"bis vor von", base(func(x *AppointmentInput) { x.Bis = dayPtr("2026-12-06") }), errMontageZeitraum},
		{"61 Tage", base(func(x *AppointmentInput) { x.Bis = dayPtr("2027-02-05") }), errMontageZuLang},
		{"Baustelle zu lang", base(func(x *AppointmentInput) { x.Baustelle = strings.Repeat("ä", 301) }), errMontageBaustelle},
		{"Notiz zu lang", base(func(x *AppointmentInput) { x.Notiz = strings.Repeat("ö", 501) }), errMontageNotiz},
	} {
		if got := c.in.Validate(); got != c.want {
			t.Errorf("%s: %v, erwartet %v", c.name, got, c.want)
		}
	}
	if d := AssemblyDays(day("2026-12-31"), day("2027-01-01")); d != 2 {
		t.Fatalf("Tage ueber den Jahreswechsel: %d", d)
	}
	// Uhrzeiten zaehlen nicht: 7.12. 23 Uhr bis 9.12. 1 Uhr sind drei Tage.
	if d := AssemblyDays(time.Date(2026, 12, 7, 23, 0, 0, 0, time.UTC), time.Date(2026, 12, 9, 1, 0, 0, 0, time.UTC)); d != 3 {
		t.Fatalf("Tage mit Uhrzeit: %d", d)
	}
}

func TestAssemblyAllowedAndCancelReason(t *testing.T) {
	for _, s := range []string{StatusEntwurf, StatusFreigegeben, StatusInArbeit, StatusFertig} {
		if err := AssemblyAllowed(s); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
	if err := AssemblyAllowed(StatusStorniert); err != errMontageStatus {
		t.Fatalf("storniert: %v", err)
	}
	if r, err := ValidateAssemblyCancelReason("  verschoben "); err != nil || r != "verschoben" {
		t.Fatalf("Begruendung: %q %v", r, err)
	}
	if _, err := ValidateAssemblyCancelReason(strings.Repeat("x", 500)); err != nil {
		t.Fatalf("500 Zeichen: %v", err)
	}
	for _, r := range []string{"", "  ", strings.Repeat("x", 501)} {
		if _, err := ValidateAssemblyCancelReason(r); err == nil {
			t.Errorf("%q angenommen", r)
		}
	}
}

func TestAssemblyHintRules(t *testing.T) {
	for _, c := range []struct {
		a, b [2]string
		want bool
	}{
		{[2]string{"2026-12-07", "2026-12-09"}, [2]string{"2026-12-09", "2026-12-10"}, true},
		{[2]string{"2026-12-07", "2026-12-09"}, [2]string{"2026-12-10", "2026-12-11"}, false},
		{[2]string{"2026-12-07", "2026-12-20"}, [2]string{"2026-12-10", "2026-12-11"}, true},
		{[2]string{"2026-12-10", "2026-12-11"}, [2]string{"2026-12-01", "2026-12-09"}, false},
		{[2]string{"2026-12-10", "2026-12-10"}, [2]string{"2026-12-10", "2026-12-10"}, true},
	} {
		if got := DateRangesOverlap(day(c.a[0]), day(c.a[1]), day(c.b[0]), day(c.b[1])); got != c.want {
			t.Errorf("%v / %v: %v", c.a, c.b, got)
		}
	}

	lt := dayPtr("2026-12-10")
	if AssemblyAfterDelivery(day("2026-12-10"), lt) || !AssemblyAfterDelivery(day("2026-12-11"), lt) || AssemblyAfterDelivery(day("2027-01-01"), nil) {
		t.Fatal("nach Liefertermin")
	}

	today := day("2026-11-12")
	for _, c := range []struct {
		name, status, von string
		lastOpen          *time.Time
		want              bool
	}{
		{"offen, Montage heute", StatusInArbeit, "2026-11-12", nil, true},
		{"offen, Montage vorbei", StatusFreigegeben, "2026-11-01", nil, true},
		{"offen, Montage spaeter, ohne Planung", StatusInArbeit, "2026-12-07", nil, false},
		{"Arbeitsgang in der Montagewoche", StatusInArbeit, "2026-12-09", dayPtr("2026-12-07"), true},
		{"Arbeitsgang nach der Montagewoche", StatusEntwurf, "2026-12-09", dayPtr("2026-12-16"), true},
		{"Arbeitsgang in der Woche davor", StatusInArbeit, "2026-12-07", dayPtr("2026-12-04"), false},
		{"fertig", StatusFertig, "2026-11-01", dayPtr("2026-12-16"), false},
		{"storniert", StatusStorniert, "2026-11-01", nil, false},
	} {
		if got := AssemblyBeforeCompletion(c.status, day(c.von), c.lastOpen, today); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}

	if h := AssemblyHints(AssemblyFacts{}); h == nil || len(h) != 0 {
		t.Fatalf("ohne Hinweise: %#v", h)
	}
	all := AssemblyHints(AssemblyFacts{true, true, true, true, true})
	want := []string{HintTeamDoppelt, HintTeamAbwesend, HintFahrzeugGesperrt, HintVorFertigstellung, HintNachLiefertermin}
	if !slices.Equal(all, want) {
		t.Fatalf("alle Hinweise: %v", all)
	}
	if h := AssemblyHints(AssemblyFacts{FahrzeugGesperrt: true, NachLiefertermin: true}); !slices.Equal(h, []string{HintFahrzeugGesperrt, HintNachLiefertermin}) {
		t.Fatalf("zwei Hinweise: %v", h)
	}
}
