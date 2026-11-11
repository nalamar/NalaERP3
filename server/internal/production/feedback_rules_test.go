package production

import (
	"math"
	"strings"
	"testing"
)

// Fachregeln der Rueckmeldung je Arbeitsgang ohne Datenbank (Backlog
// H.3.5a).

func TestFeedbackInput(t *testing.T) {
	in := FeedbackInput{IstMinuten: fPtr(95.456), Notiz: "  Zuschnitt fertig "}.Normalize()
	if in.Notiz != "Zuschnitt fertig" || in.IstMinuten == nil || *in.IstMinuten != 95.46 {
		t.Fatalf("Normalize: %+v %v", in, *in.IstMinuten)
	}
	if out := (FeedbackInput{}).Normalize(); out.IstMinuten != nil {
		t.Fatalf("ohne Ist-Zeit: %+v", out)
	}
	for _, c := range []struct {
		name string
		in   FeedbackInput
		want error
	}{
		{"ohne Ist-Zeit", FeedbackInput{}, nil},
		{"Ist-Zeit", FeedbackInput{IstMinuten: fPtr(30)}, nil},
		{"kleinste Ist-Zeit", FeedbackInput{IstMinuten: fPtr(0.01)}, nil},
		{"Grenze", FeedbackInput{IstMinuten: fPtr(MaxIstMinuten)}, nil},
		{"Notiz an der Grenze", FeedbackInput{Notiz: strings.Repeat("ä", 500)}, nil},
		{"Ist 0", FeedbackInput{IstMinuten: fPtr(0)}, errRueckmeldungIst},
		{"Ist gerundet 0", FeedbackInput{IstMinuten: fPtr(0.004)}, errRueckmeldungIst},
		{"Ist negativ", FeedbackInput{IstMinuten: fPtr(-5)}, errRueckmeldungIst},
		{"Ist zu gross", FeedbackInput{IstMinuten: fPtr(MaxIstMinuten + 0.01)}, errRueckmeldungIst},
		{"Ist NaN", FeedbackInput{IstMinuten: fPtr(math.NaN())}, errRueckmeldungIst},
		{"Ist unendlich", FeedbackInput{IstMinuten: fPtr(math.Inf(1))}, errRueckmeldungIst},
		{"Notiz zu lang", FeedbackInput{Notiz: strings.Repeat("ä", 501)}, errRueckmeldungNotiz},
	} {
		if got := c.in.Normalize().Validate(); got != c.want {
			t.Errorf("%s: %v, erwartet %v", c.name, got, c.want)
		}
	}
}

func TestFeedbackAllowed(t *testing.T) {
	if err := FeedbackAllowed(StatusInArbeit); err != nil {
		t.Fatalf("in Arbeit: %v", err)
	}
	for _, s := range []string{StatusEntwurf, StatusFreigegeben, StatusFertig, StatusStorniert, ""} {
		if err := FeedbackAllowed(s); err != errRueckmeldungStatus {
			t.Errorf("%q: %v", s, err)
		}
	}
}

func TestValidateFeedbackRevertReason(t *testing.T) {
	if got, err := ValidateFeedbackRevertReason("  versehentlich gemeldet "); err != nil || got != "versehentlich gemeldet" {
		t.Fatalf("gültig: %q %v", got, err)
	}
	if _, err := ValidateFeedbackRevertReason(strings.Repeat("x", 500)); err != nil {
		t.Fatalf("Grenze: %v", err)
	}
	for _, r := range []string{"", "   ", strings.Repeat("x", 501)} {
		if _, err := ValidateFeedbackRevertReason(r); err == nil {
			t.Errorf("%d Zeichen angenommen", len(r))
		}
	}
}

func TestSummarizeSollIst(t *testing.T) {
	if got := SummarizeSollIst(nil); got != (SollIst{}) {
		t.Fatalf("ohne Arbeitsgaenge: %+v", got)
	}
	a, b := 0.1, 0.2 // ueber Variablen, sonst rechnet Go exakt
	ops := []Operation{
		{PlanMinuten: 100, Erledigt: true, IstMinuten: fPtr(a)},
		{PlanMinuten: 50.5, Erledigt: true, IstMinuten: fPtr(b)},
		{PlanMinuten: 30, Erledigt: true},        // ohne Ist-Zeit
		{PlanMinuten: 20, IstMinuten: fPtr(999)}, // nicht erledigt: Ist zaehlt nicht
		{PlanMinuten: 0.1, Erledigt: false},
	}
	want := SollIst{Arbeitsgaenge: 5, Erledigt: 3, SollMin: 200.6, SollErledigtMin: 180.5, MitIst: 2, SollMitIstMin: 150.5, IstMin: 0.3}
	if got := SummarizeSollIst(ops); got != want {
		t.Fatalf("Soll/Ist: %+v, erwartet %+v", got, want)
	}
}
