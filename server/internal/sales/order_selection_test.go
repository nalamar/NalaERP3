package sales

import (
	"strings"
	"testing"
)

func TestSelectQuoteItemsForOrder(t *testing.T) {
	all := []quoteItemForOrder{
		{id: "n1", kind: "normal"},
		{id: "g1", kind: "grundposition", outlineNo: "01.0010"},
		{id: "a1", kind: "alternativposition", outlineNo: "01.0020", baseOutlineNo: "01.0010"},
		{id: "a2", kind: "alternativposition", outlineNo: "01.0030", baseOutlineNo: "01.0010"},
		{id: "b1", kind: "bedarfsposition_mit_gb", outlineNo: "01.0040"},
		{id: "b2", kind: "bedarfsposition_ohne_gb", outlineNo: "01.0050"},
		{id: "g2", kind: "grundposition", outlineNo: "01.0060"},
		{id: "a3", kind: "alternativposition", outlineNo: "01.0070", baseOutlineNo: "01.0060"},
		{id: "x1", kind: "alternativposition", outlineNo: "01.0080", baseOutlineNo: "09.9999"},
	}
	ids := func(items []quoteItemForOrder) string {
		out := make([]string, 0, len(items))
		for _, q := range items {
			out = append(out, q.id)
		}
		return strings.Join(out, ",")
	}
	for _, c := range []struct {
		name string
		alts []string
		want string
	}{
		{"ohne Auswahl", nil, "n1,g1,g2"},
		{"eine Alternative", []string{"a1"}, "n1,a1,g2"},
		{"zweite Alternative", []string{" a2 "}, "n1,a2,g2"},
		{"je Grundposition eine", []string{"a3", "a1"}, "n1,a1,a3"},
	} {
		got, err := selectQuoteItemsForOrder(all, c.alts)
		if err != nil || ids(got) != c.want {
			t.Fatalf("%s: %q %v, erwartet %q", c.name, ids(got), err, c.want)
		}
	}
	for _, c := range []struct {
		name string
		alts []string
		want string
	}{
		{"zwei zur selben Grundposition", []string{"a1", "a2"}, "mehrere Alternativen zur Grundposition 01.0010"},
		{"doppelt", []string{"a1", "a1"}, "mehrere Alternativen"},
		{"Bedarfsposition", []string{"b1"}, "b1 ist keine Alternativposition"},
		{"Grundposition", []string{"g1"}, "g1 ist keine Alternativposition"},
		{"unbekannt", []string{"zz"}, "zz ist keine Alternativposition"},
		{"Grundposition fehlt", []string{"x1"}, "Grundposition 09.9999 fehlt"},
	} {
		if _, err := selectQuoteItemsForOrder(all, c.alts); err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "ungültig") {
			t.Fatalf("%s: %v, erwartet %q", c.name, err, c.want)
		}
	}
}
