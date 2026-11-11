package production

import (
	"errors"
	"math"
	"strings"
	"unicode/utf8"
)

// Fachregeln der Rueckmeldung je Arbeitsgang ohne Datenbank (ADR 0038
// Frage 5 (a), Backlog H.3.5a). Tabelle in Migration 114; Service und
// Routen folgen in H.3.5b/c. Ist-Zeit ohne Personenbezug.

// MaxIstMinuten: Obergrenze der Ist-Zeit eines Arbeitsgangs gegen
// Tippfehler (rund 1.667 Stunden), wie in Migration 114.
const MaxIstMinuten = 100_000

var (
	errRueckmeldungStatus = errors.New("Rückmeldung nur bei begonnenen Aufträgen möglich")
	errRueckmeldungIst    = errors.New("Ungültige Ist-Zeit (größer als 0, höchstens 100.000 Minuten)")
	errRueckmeldungNotiz  = errors.New("Notiz ist zu lang (höchstens 500 Zeichen)")
)

// FeedbackInput: "erledigt" mit optionaler Ist-Zeit in Minuten und Notiz.
type FeedbackInput struct {
	IstMinuten *float64 `json:"ist_minuten"`
	Notiz      string   `json:"notiz"`
}

// Normalize entfernt Leerraum und rundet die Ist-Zeit auf zwei Stellen
// (Spalte numeric(10,2)).
func (in FeedbackInput) Normalize() FeedbackInput {
	in.Notiz = strings.TrimSpace(in.Notiz)
	if in.IstMinuten != nil {
		v := math.Round(*in.IstMinuten*100) / 100
		in.IstMinuten = &v
	}
	return in
}

// Validate prueft eine normalisierte Eingabe. Ohne Ist-Zeit ist erlaubt;
// 0 Minuten nicht (dann bitte leer lassen).
func (in FeedbackInput) Validate() error {
	if in.IstMinuten != nil {
		v := *in.IstMinuten
		if math.IsNaN(v) || v <= 0 || v > MaxIstMinuten {
			return errRueckmeldungIst
		}
	}
	if utf8.RuneCountInString(in.Notiz) > MaxNotizLaenge {
		return errRueckmeldungNotiz
	}
	return nil
}

// FeedbackAllowed: gemeldet und zurueckgenommen wird nur, solange der
// Auftrag in Arbeit ist; nach "fertig" oder Storno bleibt der Stand.
func FeedbackAllowed(status string) error {
	if status == StatusInArbeit {
		return nil
	}
	return errRueckmeldungStatus
}

// ValidateFeedbackRevertReason: Ruecknahme nur mit Begruendung.
func ValidateFeedbackRevertReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", errors.New("Rücknahme der Rückmeldung nur mit Begründung")
	}
	if utf8.RuneCountInString(reason) > MaxNotizLaenge {
		return "", errors.New("Begründung ist zu lang (höchstens 500 Zeichen)")
	}
	return reason, nil
}

// SollIst: Plan- und Ist-Zeit eines Auftrags aus seinen Arbeitsgaengen
// (H.3.5c). Verglichen wird nur ueber erledigte Arbeitsgaenge mit
// Ist-Zeit (SollMitIstMin gegen IstMin), damit fehlende Ist-Zeiten den
// Vergleich nicht verzerren.
type SollIst struct {
	Arbeitsgaenge   int     `json:"arbeitsgaenge"`
	Erledigt        int     `json:"erledigt"`
	SollMin         float64 `json:"soll_min"`
	SollErledigtMin float64 `json:"soll_erledigt_min"`
	MitIst          int     `json:"mit_ist"`
	SollMitIstMin   float64 `json:"soll_mit_ist_min"`
	IstMin          float64 `json:"ist_min"`
}

// SummarizeSollIst summiert die Arbeitsgaenge; Minuten auf zwei Stellen.
func SummarizeSollIst(ops []Operation) SollIst {
	var s SollIst
	for _, op := range ops {
		s.Arbeitsgaenge++
		s.SollMin += op.PlanMinuten
		if !op.Erledigt {
			continue
		}
		s.Erledigt++
		s.SollErledigtMin += op.PlanMinuten
		if op.IstMinuten != nil {
			s.MitIst++
			s.SollMitIstMin += op.PlanMinuten
			s.IstMin += *op.IstMinuten
		}
	}
	for _, p := range []*float64{&s.SollMin, &s.SollErledigtMin, &s.SollMitIstMin, &s.IstMin} {
		*p = math.Round(*p*100) / 100
	}
	return s
}
