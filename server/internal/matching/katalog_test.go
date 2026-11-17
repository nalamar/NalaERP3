package matching

import (
	"strings"
	"testing"
)

// Grenzwerte als feste Zahlen, nicht ueber die Konstante.
func TestSystemvorgabenValidate(t *testing.T) {
	u := func(v float64) *float64 { return &v }
	gut := []Systemvorgaben{
		{},
		{Profilserie: strings.Repeat("é", 200), UWert: u(10)},
		{Brandschutzklasse: "T30", UWert: u(0.001)},
	}
	for _, s := range gut {
		if err := s.Validate(); err != nil {
			t.Fatalf("%+v abgelehnt: %v", s, err)
		}
	}
	schlecht := []struct {
		s    Systemvorgaben
		want string
	}{
		{Systemvorgaben{Profilserie: strings.Repeat("é", 201)}, "zu lang"},
		{Systemvorgaben{Brandschutzklasse: strings.Repeat("x", 201)}, "zu lang"},
		{Systemvorgaben{UWert: u(0)}, "U-Wert"},
		{Systemvorgaben{UWert: u(10.01)}, "U-Wert"},
	}
	for _, c := range schlecht {
		if err := c.s.Validate(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%+v: %v, erwartet %q", c.s, err, c.want)
		}
	}
}
