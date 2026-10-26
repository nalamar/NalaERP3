package fleet

import (
	"sort"
	"testing"

	"github.com/google/uuid"
)

// Reihenfolge der Faelligkeitsliste ohne Datenbank: jede Regel an einem
// Paar, das nur sie entscheidet; das Ergebnis darf nicht von der
// Eingabereihenfolge abhaengen.
func TestDueLessOrder(t *testing.T) {
	days := func(d int) *int { return &d }
	e := func(kz, art, bez, status string, tage *int, id string) ScheduleDueEntry {
		return ScheduleDueEntry{Kennzeichen: kz, Art: art, Bezeichnung: bez, ScheduleID: uuid.MustParse(id), Standing: ScheduleStanding{Status: status, TageBisFaellig: tage}}
	}
	want := []ScheduleDueEntry{
		e("M-Z 1", TerminHU, "", TerminstandGesperrt, days(30), "00000000-0000-0000-0000-000000000001"),
		e("M-A 1", TerminHU, "", TerminstandGesperrt, nil, "00000000-0000-0000-0000-000000000002"),
		e("M-Z 1", TerminUVV, "", TerminstandUeberfaellig, days(-20), "00000000-0000-0000-0000-000000000003"),
		e("M-A 1", TerminUVV, "", TerminstandUeberfaellig, days(-5), "00000000-0000-0000-0000-000000000004"),
		// Gleicher Tag: nach Kennzeichen.
		e("M-B 1", TerminWartung, "", TerminstandFaelligBald, days(3), "00000000-0000-0000-0000-000000000005"),
		e("M-C 1", TerminWartung, "", TerminstandFaelligBald, days(3), "00000000-0000-0000-0000-000000000006"),
		// Gleiches Fahrzeug, gleicher Tag: nach Terminart.
		// Die IDs sind absichtlich umgekehrt - sonst entschiede die ID.
		e("M-E 1", TerminUVV, "", TerminstandFaelligBald, days(7), "00000000-0000-0000-0000-000000000008"),
		e("M-E 1", TerminWartung, "", TerminstandFaelligBald, days(7), "00000000-0000-0000-0000-000000000007"),
		// Sonstige: nach Bezeichnung ohne Gross-/Kleinschreibung.
		e("M-E 1", TerminSonstige, "alt", TerminstandFaelligBald, days(9), "00000000-0000-0000-0000-000000000009"),
		e("M-E 1", TerminSonstige, "Beta", TerminstandFaelligBald, days(9), "00000000-0000-0000-0000-00000000000a"),
		// Sonst gleich: nach ID (bestimmt).
		e("M-E 1", TerminSonstige, "Gamma", TerminstandFaelligBald, days(9), "00000000-0000-0000-0000-00000000000b"),
		e("M-E 1", TerminSonstige, "gamma", TerminstandFaelligBald, days(9), "00000000-0000-0000-0000-00000000000c"),
	}
	n := len(want)
	for shift := 0; shift < n; shift++ {
		for _, reverse := range []bool{false, true} {
			in := make([]ScheduleDueEntry, n)
			for i := range want {
				j := (i + shift) % n
				if reverse {
					j = n - 1 - j
				}
				in[j] = want[i]
			}
			sort.SliceStable(in, func(i, j int) bool { return dueLess(in[i], in[j]) })
			for i := range want {
				if in[i].ScheduleID != want[i].ScheduleID {
					t.Fatalf("Verschiebung %d, umgekehrt %v: Position %d ist %s %s %s, erwartet %s %s %s", shift, reverse, i,
						in[i].Kennzeichen, in[i].Art, in[i].Bezeichnung, want[i].Kennzeichen, want[i].Art, want[i].Bezeichnung)
				}
			}
		}
	}
}
