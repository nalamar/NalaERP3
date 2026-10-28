package fleet

import (
	"reflect"
	"testing"
	"time"
)

// Regeln des Fahrtenbuchs ohne Datenbank (Backlog G.3c.1).

func TestNormalizeTripInput(t *testing.T) {
	got := NormalizeTripInput(TripInput{Fahrtart: " Dienstlich ", Ziel: " Ulm ", Zweck: " Montage ", Geschaeftspartner: " Fa. X ", Notiz: " n "})
	if got.Fahrtart != FahrtartDienstlich || got.Ziel != "Ulm" || got.Zweck != "Montage" || got.Geschaeftspartner != "Fa. X" || got.Notiz != "n" {
		t.Fatalf("%+v", got)
	}
}

func TestValidateTripInput(t *testing.T) {
	today := day(2026, 5, 10)
	ez := datePtr(2026, 2, 1)
	ab := datePtr(2026, 5, 1)
	valid := TripInput{Datum: day(2026, 4, 1), KmBeginn: 1000, KmEnde: 1050, Fahrtart: FahrtartDienstlich, Ziel: "Ulm", Zweck: "Montage"}
	for name, in := range map[string]TripInput{
		"dienstlich":            valid,
		"privat ohne Ziel":      {Datum: day(2026, 4, 1), KmBeginn: 0, KmEnde: 1, Fahrtart: FahrtartPrivat},
		"Arbeitsweg ohne Ziel":  {Datum: day(2026, 4, 30), KmBeginn: 5, KmEnde: 30, Fahrtart: FahrtartArbeitsweg},
		"Tag der Erstzulassung": {Datum: day(2026, 2, 1), KmBeginn: 5, KmEnde: 30, Fahrtart: FahrtartPrivat},
		"Tag der Abmeldung":     {Datum: day(2026, 5, 1), KmBeginn: 5, KmEnde: 30, Fahrtart: FahrtartPrivat},
	} {
		if err := ValidateTripInput(in, ez, ab, today); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if err := ValidateTripInput(TripInput{Datum: today, KmBeginn: 1, KmEnde: 2, Fahrtart: FahrtartPrivat}, nil, nil, today); err != nil {
		t.Fatalf("heute ohne Erstzulassung/Abmeldung: %v", err)
	}
	mod := func(f func(*TripInput)) TripInput {
		in := valid
		f(&in)
		return in
	}
	for name, tc := range map[string]struct {
		in   TripInput
		want string
	}{
		"ohne Datum":        {mod(func(i *TripInput) { i.Datum = time.Time{} }), "Datum erforderlich"},
		"Zukunft":           {mod(func(i *TripInput) { i.Datum = day(2026, 5, 11) }), "Datum darf nicht in der Zukunft liegen"},
		"vor Erstzulassung": {mod(func(i *TripInput) { i.Datum = day(2026, 1, 31) }), "Datum darf nicht vor der Erstzulassung liegen"},
		"nach Abmeldung":    {mod(func(i *TripInput) { i.Datum = day(2026, 5, 2) }), "Datum darf nicht nach der Abmeldung liegen"},
		"km negativ":        {mod(func(i *TripInput) { i.KmBeginn = -1 }), "Kilometerstand Beginn darf nicht negativ sein"},
		"km gleich":         {mod(func(i *TripInput) { i.KmEnde = 1000 }), "Kilometerstand Ende muss über dem Beginn liegen"},
		"km rueckwaerts":    {mod(func(i *TripInput) { i.KmEnde = 999 }), "Kilometerstand Ende muss über dem Beginn liegen"},
		"Fahrtart":          {mod(func(i *TripInput) { i.Fahrtart = "urlaub" }), "Ungültige Fahrtart"},
		"ohne Ziel":         {mod(func(i *TripInput) { i.Ziel = "" }), "Reiseziel ist bei Dienstfahrten erforderlich"},
		"ohne Zweck":        {mod(func(i *TripInput) { i.Zweck = "" }), "Reisezweck ist bei Dienstfahrten erforderlich"},
	} {
		if err := ValidateTripInput(tc.in, ez, ab, today); err == nil || err.Error() != tc.want {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func span(d, beginn, ende int) TripSpan {
	return TripSpan{Datum: day(2026, 3, d), KmBeginn: beginn, KmEnde: ende}
}

func TestCheckTripChain(t *testing.T) {
	// Kette 1000-1100 (3.), 1100-1150 (5.); Luecke 1150-1300; 1300-1400 (9.).
	existing := []TripSpan{span(5, 1100, 1150), span(3, 1000, 1100), span(9, 1300, 1400)}
	if err := CheckTripChain(nil, span(1, 0, 10)); err != nil {
		t.Fatalf("erste Fahrt ueberhaupt: %v", err)
	}
	for name, n := range map[string]TripSpan{
		"am Ende anschliessen":         span(9, 1400, 1450),
		"am Ende, spaeterer Tag":       span(12, 1400, 1450),
		"in die Luecke, ganz":          span(7, 1150, 1300),
		"in die Luecke, teilweise":     span(5, 1150, 1200),
		"Luecke: Tag der Nachfolgerin": span(9, 1150, 1300),
		"vor der ersten, verbunden":    span(1, 900, 1000),
	} {
		if err := CheckTripChain(existing, n); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for name, tc := range map[string]struct {
		n    TripSpan
		want string
	}{
		"Luecke am Ende":         {span(10, 1410, 1450), "Kilometerstand Beginn muss 1400 km sein (Ende der vorigen Fahrt)"},
		"in der Luecke versetzt": {span(6, 1160, 1200), "Kilometerstand Beginn muss 1150 km sein (Ende der vorigen Fahrt)"},
		"ueberlappt":             {span(9, 1350, 1450), "Kilometerbereich überschneidet sich mit der Fahrt vom 09.03.2026 (1300–1400 km)"},
		"umschliesst":            {span(4, 1050, 1120), "Kilometerbereich überschneidet sich mit der Fahrt vom"},
		"Luecke zu gross":        {span(6, 1150, 1310), "Kilometerbereich überschneidet sich mit der Fahrt vom 09.03.2026 (1300–1400 km)"},
		"vor der ersten, Luecke": {span(1, 900, 990), "Kilometerstand Ende muss 1000 km sein (Beginn der folgenden Fahrt)"},
		"Datum vor der vorigen":  {span(8, 1400, 1450), "Datum darf nicht vor der vorigen Fahrt (09.03.2026) liegen"},
		"Datum nach der Folgen":  {span(10, 1150, 1300), "Datum darf nicht nach der folgenden Fahrt (09.03.2026) liegen"},
		"vor der ersten, spaet":  {span(4, 900, 1000), "Datum darf nicht nach der folgenden Fahrt (03.03.2026) liegen"},
	} {
		err := CheckTripChain(existing, tc.n)
		if err == nil || len(err.Error()) < len(tc.want) || err.Error()[:len(tc.want)] != tc.want {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestTripGaps(t *testing.T) {
	if got := TripGaps(nil); got == nil || len(got) != 0 {
		t.Fatalf("leer: %#v", got)
	}
	got := TripGaps([]TripSpan{span(9, 1300, 1400), span(3, 1000, 1100), span(5, 1100, 1150), span(12, 1500, 1510)})
	want := []TripGap{{VonKm: 1150, BisKm: 1300}, {VonKm: 1400, BisKm: 1500}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
	if got := TripGaps([]TripSpan{span(3, 1000, 1100), span(5, 1100, 1150)}); len(got) != 0 {
		t.Fatalf("lueckenlos: %+v", got)
	}
}
