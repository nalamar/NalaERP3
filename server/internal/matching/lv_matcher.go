package matching

import (
	"context"

	"nalaerp3/internal/quotes/gaeb"
)

// LVMatcher schlaegt fuer alle Positionen eines gelesenen LV auf einmal vor
// (ADR 0044 Frage 3): so kann ein Matcher Positionen buendeln und Bereiche
// und Vorbemerkungen als Kontext nutzen. Das LV kommt aus dem Parser und
// enthaelt nichts aus der Erwartung. Ergebnis je OZ; fehlt eine OZ, landet
// die Position in der Pruefliste. Ein Fehler (Abbruch, Tageslimit) beendet
// den Lauf.
type LVMatcher interface {
	Name() string
	VorschlagenLV(ctx context.Context, lv *gaeb.LV, k *Katalog) (map[string]Vorschlag, error)
}

// JePosition macht aus einem Matcher je Position einen LVMatcher.
func JePosition(m Matcher) LVMatcher { return jePosition{m} }

type jePosition struct{ m Matcher }

func (j jePosition) Name() string { return j.m.Name() }

func (j jePosition) VorschlagenLV(ctx context.Context, lv *gaeb.LV, k *Katalog) (map[string]Vorschlag, error) {
	out := make(map[string]Vorschlag, len(lv.Positionen))
	for _, p := range lv.Positionen {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out[p.OZ] = j.m.Vorschlagen(Eingabe(p), k)
	}
	return out, nil
}

// MitSystemvorgaben setzt RC-Klasse, U-Wert und Brandschutzklasse jedes
// Vorschlags aus der deterministischen Erkennung (ADR 0044 Frage 5) -
// auch leer, wenn nichts erkannt wird; Profilserie und Verglasung bleiben
// vom Matcher. Positionen ohne Vorschlag bekommen einen ohne Code.
func MitSystemvorgaben(m LVMatcher) LVMatcher { return mitSystemvorgaben{m} }

type mitSystemvorgaben struct{ m LVMatcher }

func (s mitSystemvorgaben) Name() string { return s.m.Name() + "+systemvorgaben" }

func (s mitSystemvorgaben) VorschlagenLV(ctx context.Context, lv *gaeb.LV, k *Katalog) (map[string]Vorschlag, error) {
	out, err := s.m.VorschlagenLV(ctx, lv, k)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]Vorschlag{}
	}
	for _, p := range lv.Positionen {
		v, ok := out[p.OZ]
		if !ok {
			v.Begruendung = "kein Vorschlag des Matchers"
		}
		e := ErkenneSystemvorgaben(p.Kurztext, p.Langtext)
		v.Systemvorgaben.RCKlasse, v.Systemvorgaben.UWert, v.Systemvorgaben.Brandschutzklasse = e.RCKlasse, e.UWert, e.Brandschutzklasse
		out[p.OZ] = v
	}
	return out, nil
}
