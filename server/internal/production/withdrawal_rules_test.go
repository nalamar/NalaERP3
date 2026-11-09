package production

import (
	"math"
	"strings"
	"testing"
)

// Fachregeln der Entnahme ohne Datenbank (Backlog H.3.1).

func TestWithdrawalInput(t *testing.T) {
	in := WithdrawalInput{ItemID: " i1 ", WarehouseID: " wh ", LocationID: " ", Menge: 2.0000004, Notiz: "  Rest  "}.Normalize()
	if in.ItemID != "i1" || in.WarehouseID != "wh" || in.LocationID != "" || in.Notiz != "Rest" || in.Menge != 2 {
		t.Fatalf("Normalize: %+v", in)
	}
	if err := in.Validate(); err != nil {
		t.Fatalf("gueltig abgelehnt: %v", err)
	}
	if err := (WithdrawalInput{MaterialID: "m", WarehouseID: "wh", Menge: MaxEntnahmeMenge}).Validate(); err != nil {
		t.Fatalf("Material ohne Zeile, Hoechstmenge: %v", err)
	}
	for name, c := range map[string]struct {
		in   WithdrawalInput
		want string
	}{
		"ohne Zeile und Material": {WithdrawalInput{WarehouseID: "wh", Menge: 1}, "Zeile der Stückliste oder ein Material"},
		"ohne Lager":              {WithdrawalInput{ItemID: "i", Menge: 1}, "Lager"},
		"Menge 0":                 {WithdrawalInput{ItemID: "i", WarehouseID: "wh"}, "Ungültige Menge"},
		"Menge negativ":           {WithdrawalInput{ItemID: "i", WarehouseID: "wh", Menge: -1}, "Ungültige Menge"},
		"Menge zu gross":          {WithdrawalInput{ItemID: "i", WarehouseID: "wh", Menge: MaxEntnahmeMenge + 1}, "Ungültige Menge"},
		"Menge NaN":               {WithdrawalInput{ItemID: "i", WarehouseID: "wh", Menge: math.NaN()}, "Ungültige Menge"},
		"Notiz zu lang":           {WithdrawalInput{ItemID: "i", WarehouseID: "wh", Menge: 1, Notiz: strings.Repeat("ä", MaxNotizLaenge+1)}, "Notiz ist zu lang"},
	} {
		err := c.in.Normalize().Validate()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, erwartet %q", name, err, c.want)
		}
	}
	// genau 500 Zeichen (Umlaute zaehlen als ein Zeichen)
	if err := (WithdrawalInput{ItemID: "i", WarehouseID: "wh", Menge: 1, Notiz: strings.Repeat("ä", MaxNotizLaenge)}).Validate(); err != nil {
		t.Fatalf("500 Zeichen: %v", err)
	}
	// Rundung auf 6 Stellen: 0,0000004 ist 0
	if err := (WithdrawalInput{ItemID: "i", WarehouseID: "wh", Menge: 0.0000004}).Normalize().Validate(); err == nil {
		t.Fatal("Menge unter der Spaltengenauigkeit angenommen")
	}
}

func TestWithdrawalAllowed(t *testing.T) {
	for _, s := range []string{StatusFreigegeben, StatusInArbeit} {
		if err := WithdrawalAllowed(s); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
	for _, s := range []string{StatusEntwurf, StatusFertig, StatusStorniert, ""} {
		if err := WithdrawalAllowed(s); err == nil {
			t.Errorf("%q: Entnahme erlaubt", s)
		}
	}
}

func TestOpenQtyAndOverdrawn(t *testing.T) {
	if q, ok := OpenQty(10, true, 4); !ok || q != 6 {
		t.Fatalf("offen: %v %v", q, ok)
	}
	if q, ok := OpenQty(10, true, 12); !ok || q != 0 {
		t.Fatalf("mehr entnommen: %v %v", q, ok)
	}
	if q, ok := OpenQty(0.3, true, 0.1+0.2); !ok || q != 0 {
		t.Fatalf("Rundungsrauschen: %v", q)
	}
	if _, ok := OpenQty(10, false, 0); ok {
		t.Fatal("unbekannter Bedarf mit offener Menge")
	}
	if !Overdrawn(10, true, 8, 3) {
		t.Fatal("8+3 > 10 nicht erkannt")
	}
	if Overdrawn(10, true, 8, 2) {
		t.Fatal("genau der Bedarf als Mehrentnahme")
	}
	if Overdrawn(0.3, true, 0.1, 0.2) {
		t.Fatal("Rundungsrauschen als Mehrentnahme")
	}
	if Overdrawn(10, false, 8, 3) {
		t.Fatal("ohne bekannten Bedarf keine Mehrentnahme")
	}
}

func TestReservationConsumption(t *testing.T) {
	for name, c := range map[string]struct {
		status     string
		qty, menge float64
		verbraucht float64
		rest       float64
		wantStatus string
	}{
		"teilweise":           {ReservierungAktiv, 10, 4, 4, 6, ReservierungAktiv},
		"genau":               {ReservierungAktiv, 10, 10, 10, 10, ReservierungVerbraucht},
		"mehr als reserviert": {ReservierungAktiv, 10, 12, 10, 10, ReservierungVerbraucht},
		"Rundung":             {ReservierungAktiv, 0.3, 0.1 + 0.2, 0.3, 0.3, ReservierungVerbraucht},
		"Rest gerundet":       {ReservierungAktiv, 0.3, 0.1, 0.1, 0.2, ReservierungAktiv},
		"freigegeben":         {ReservierungFreigegeben, 10, 4, 0, 10, ReservierungFreigegeben},
		"schon verbraucht":    {ReservierungVerbraucht, 10, 4, 0, 10, ReservierungVerbraucht},
	} {
		v, r, s := ReservationConsumption(c.status, c.qty, c.menge)
		if v != c.verbraucht || r != c.rest || s != c.wantStatus {
			t.Errorf("%s: %v %v %s, erwartet %v %v %s", name, v, r, s, c.verbraucht, c.rest, c.wantStatus)
		}
	}
}

func TestReservationAfterStorno(t *testing.T) {
	// 10 reserviert, A nimmt 4 (Rest 6), B nimmt 6 (verbraucht, qty 6).
	_, rest, st := ReservationConsumption(ReservierungAktiv, 10, 4)
	_, qty, st := ReservationConsumption(st, rest, 6)
	if st != ReservierungVerbraucht || qty != 6 {
		t.Fatalf("Ausgang: %v %s", qty, st)
	}
	// Storno A: wieder aktiv mit 4; danach Storno B: 10.
	qty, st, ok := ReservationAfterStorno(StatusInArbeit, st, qty, 4)
	if !ok || st != ReservierungAktiv || qty != 4 {
		t.Fatalf("Storno A: %v %s %v", qty, st, ok)
	}
	qty, st, ok = ReservationAfterStorno(StatusFreigegeben, st, qty, 6)
	if !ok || st != ReservierungAktiv || qty != 10 {
		t.Fatalf("Storno B: %v %s %v", qty, st, ok)
	}
	for name, c := range map[string]struct {
		order, status string
		verbraucht    float64
	}{
		"Auftrag fertig":           {StatusFertig, ReservierungVerbraucht, 4},
		"Auftrag storniert":        {StatusStorniert, ReservierungAktiv, 4},
		"Reservierung freigegeben": {StatusInArbeit, ReservierungFreigegeben, 4},
		"nichts verbraucht":        {StatusInArbeit, ReservierungAktiv, 0},
	} {
		q, s, ok := ReservationAfterStorno(c.order, c.status, 7, c.verbraucht)
		if ok || q != 7 || s != c.status {
			t.Errorf("%s: %v %s %v", name, q, s, ok)
		}
	}
}

func TestWithdrawalStornoReason(t *testing.T) {
	if r, err := ValidateWithdrawalStornoReason("  falsches Lager "); err != nil || r != "falsches Lager" {
		t.Fatalf("%q %v", r, err)
	}
	if _, err := ValidateWithdrawalStornoReason("  "); err == nil {
		t.Fatal("leere Begruendung angenommen")
	}
	if _, err := ValidateWithdrawalStornoReason(strings.Repeat("x", MaxNotizLaenge+1)); err == nil {
		t.Fatal("zu lange Begruendung angenommen")
	}
}
