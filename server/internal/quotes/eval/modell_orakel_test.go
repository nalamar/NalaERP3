package eval

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"nalaerp3/internal/llm"
	"nalaerp3/internal/matching"
)

// Messung des Matchers mit Modell am Korpus, nur gegen den Test-Anbieter
// (llm.Fake), ohne Netz. Der Matcher liegt in internal/matching.

type stillRecorder struct{}

func (stillRecorder) Record(context.Context, llm.CallRecord) {}

// orakelAnfrage liest aus der Anfrage nur OZ und Kandidaten.
type orakelAnfrage struct {
	Positionen []struct {
		OZ         string `json:"oz"`
		Kandidaten []struct {
			Code string `json:"code"`
		} `json:"kandidaten"`
	} `json:"positionen"`
}

type orakelZeile struct {
	OZ          string  `json:"oz"`
	Code        string  `json:"code"`
	Score       float64 `json:"score"`
	Begruendung string  `json:"begruendung"`
	Profilserie string  `json:"profilserie"`
	Verglasung  string  `json:"verglasung"`
}

// orakel antwortet mit dem erwarteten Code, wenn er unter den Kandidaten
// ist - misst Vorauswahl, Bündel und Abzüge ohne Modell.
func orakel(t *testing.T, erwartet map[string]string) *llm.Fake {
	return &llm.Fake{Respond: func(_ context.Context, _ int, req llm.Request) (llm.Response, error) {
		var a orakelAnfrage
		if len(req.Messages) != 1 || json.Unmarshal([]byte(req.Messages[0].Content), &a) != nil {
			t.Fatalf("Anfrage: %+v", req.Messages)
		}
		zeilen := []orakelZeile{}
		for _, p := range a.Positionen {
			z := orakelZeile{OZ: p.OZ, Begruendung: "Orakel"}
			for _, kd := range p.Kandidaten {
				if kd.Code == erwartet[p.OZ] {
					z.Code, z.Score = kd.Code, 0.95
				}
			}
			zeilen = append(zeilen, z)
		}
		raw, _ := json.Marshal(map[string]any{"vorschlaege": zeilen})
		return llm.Response{Text: string(raw), Model: "fake"}, nil
	}}
}

// TestModellMatcherOrakel: mit perfektem Modell erreicht die Pipeline am
// Korpus die Zielwerte (Obergrenze fuer I.4.3); mit erzwungener Vorauswahl
// (3 Kandidaten) fehlen die Treffer, deren Leistung die Vorauswahl nicht
// findet. Statt 250 Aufrufen je Position reichen 42 Buendel.
func TestModellMatcherOrakel(t *testing.T) {
	k, lvs := korpus(t)
	for _, c := range []struct {
		name          string
		m             matching.ModellMatcher
		richtig, aufr int
	}{
		// 4 Treffer fallen durch den Einheiten-Abzug in die Prüfliste (Stk im LV, m/m2 im Katalog)
		{"alle Kandidaten", matching.ModellMatcher{Modell: "orakel"}, 180, 42},
		{"Vorauswahl 3", matching.ModellMatcher{Modell: "orakel", MaxKandidaten: 3, KleinerKatalog: 1}, 160, 41},
	} {
		var gesamt []Ergebnis
		aufrufe := 0
		for _, lv := range lvs {
			erwartet := map[string]string{}
			for _, p := range lv.Positionen {
				erwartet[p.OZ] = p.Leistung
			}
			f := orakel(t, erwartet)
			m := c.m
			m.Client = llm.NewClient(f, llm.Limits{Timeout: 5 * time.Second}, stillRecorder{})
			l, err := AusfuehrenLV(context.Background(), []*ErwartetesLV{lv}, k, matching.MitSystemvorgaben(m), 0.9)
			if err != nil {
				t.Fatal(err)
			}
			gesamt = append(gesamt, l.Gesamt)
			aufrufe += len(f.Requests)
		}
		g := Summe(gesamt...)
		t.Logf("%s: richtig %d, falsch %d, automatisch %d, Treffer %s, Fehlzuordnung %s, Aufrufe %d", c.name, g.Richtig, g.Falsch, g.Automatisch,
			prozent(g.Trefferquote()), prozent(g.Fehlzuordnungsrate()), aufrufe)
		if g.Richtig != c.richtig || g.Falsch != 0 || aufrufe != c.aufr || g.Merkmale["profilserie"].Erkannt != 0 || g.Merkmale["u_wert"].Richtig != 42 {
			t.Errorf("%s: %+v, %d Aufrufe", c.name, g, aufrufe)
		}
	}
}
