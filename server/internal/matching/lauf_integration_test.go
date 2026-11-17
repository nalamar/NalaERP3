package matching_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"nalaerp3/internal/leistungen"
	"nalaerp3/internal/llm"
	"nalaerp3/internal/matching"
	"nalaerp3/internal/quotes"
	"nalaerp3/internal/quotes/eval"
	"nalaerp3/internal/testutil"
)

// Lauf ueber einen Import (Backlog I.5.2b): Wortabgleich, Profil ueber den
// Test-Anbieter, Status, Fortschritt, Entscheidungen bleiben. Erfundene LVs.

type stillRecorder struct{}

func (stillRecorder) Record(context.Context, llm.CallRecord) {}

// fakeProfile liefert immer denselben Client oder einen Fehler.
type fakeProfile struct {
	client *llm.Client
	ab     *llm.ProfilAbschrift
	err    error
	ids    []string
}

func (f *fakeProfile) ProfileClient(_ context.Context, _, id string, _ llm.Recorder) (*llm.Client, *llm.ProfilAbschrift, error) {
	f.ids = append(f.ids, id)
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.client, f.ab, nil
}

// ersterKandidat antwortet je Position mit dem ersten Kandidaten;
// fehlerAb > 0: ab diesem Aufruf das Tageslimit.
func ersterKandidat(fehlerAb int) *llm.Fake {
	return &llm.Fake{Respond: func(_ context.Context, call int, req llm.Request) (llm.Response, error) {
		if fehlerAb > 0 && call >= fehlerAb {
			return llm.Response{}, llm.ErrDailyLimit
		}
		var a struct {
			Positionen []struct {
				OZ         string `json:"oz"`
				Kandidaten []struct {
					Code string `json:"code"`
				} `json:"kandidaten"`
			} `json:"positionen"`
		}
		if err := json.Unmarshal([]byte(req.Messages[0].Content), &a); err != nil {
			return llm.Response{}, err
		}
		type v struct {
			OZ          string  `json:"oz"`
			Code        string  `json:"code"`
			Score       float64 `json:"score"`
			Begruendung string  `json:"begruendung"`
			Profilserie string  `json:"profilserie"`
			Verglasung  string  `json:"verglasung"`
		}
		var out struct {
			Vorschlaege []v `json:"vorschlaege"`
		}
		for _, p := range a.Positionen {
			out.Vorschlaege = append(out.Vorschlaege, v{OZ: p.OZ, Code: p.Kandidaten[0].Code, Score: 0.95, Begruendung: "erster Kandidat"})
		}
		b, _ := json.Marshal(out)
		return llm.Response{Text: string(b), Model: "fake-1"}, nil
	}}
}

// laufUmgebung: Mandant mit Katalog des Evaluationssets und Import lv-01.
func laufUmgebung(t *testing.T, env *testutil.IntegrationEnv) (company, imp string) {
	t.Helper()
	ctx := context.Background()
	company, project := storeMandant(t, env)
	k, err := eval.LoadKatalog("../quotes/eval/testdata/katalog.json")
	if err != nil {
		t.Fatal(err)
	}
	lst := leistungen.NewService(env.PG)
	for _, l := range k.Leistungen {
		sv := l.Systemvorgaben
		if _, err := lst.Anlegen(ctx, leistungen.Eingabe{Code: l.Code, Kurztext: l.Kurztext, Einheit: l.Einheit, Gewerk: l.Gewerk,
			Profilserie: sv.Profilserie, Verglasung: sv.Verglasung, RCKlasse: sv.RCKlasse, UWert: sv.UWert, Brandschutzklasse: sv.Brandschutzklasse},
			"", company); err != nil {
			t.Fatal(err)
		}
	}
	return company, importiere(t, env, quotes.NewService(env.PG, nil), project, company, "../quotes/eval/testdata/lv-01.x83")
}

type vorschlagZeile struct {
	code, begruendung, quelle, profilName, modell, entscheidung, systemvorgaben string
	leistungID, profilID                                                        *string
	score, schwelle                                                             float64
}

func vorschlaege(t *testing.T, env *testutil.IntegrationEnv, imp string) map[string]vorschlagZeile {
	t.Helper()
	rows, err := env.PG.Query(context.Background(), `SELECT i.position_no, m.code, m.begruendung, m.quelle, m.profil_name, m.modell,
		m.entscheidung, m.systemvorgaben::text, m.leistung_id, m.profil_id::text, m.score::float8, m.schwelle::float8
		FROM quote_import_item_matches m JOIN quote_import_items i ON i.id = m.quote_import_item_id WHERE m.import_id=$1`, imp)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]vorschlagZeile{}
	for rows.Next() {
		var oz string
		var z vorschlagZeile
		if err := rows.Scan(&oz, &z.code, &z.begruendung, &z.quelle, &z.profilName, &z.modell, &z.entscheidung, &z.systemvorgaben, &z.leistungID, &z.profilID,
			&z.score, &z.schwelle); err != nil {
			t.Fatal(err)
		}
		out[oz] = z
	}
	return out
}

func TestLaufWortabgleich(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	store := matching.NewStore(env.PG)
	company, imp := laufUmgebung(t, env)
	svc := matching.NewLaufService(store, &fakeProfile{err: llm.ErrNotEnabled})

	if st, err := svc.Status(ctx, imp, company); err != nil || st.Status != "" || st.GestartetAm != nil {
		t.Fatalf("vor dem Lauf: %+v %v", st, err)
	}
	lv, positionen, _ := store.LadeLV(ctx, imp, company)
	st, run, err := svc.Starten(ctx, imp, matching.LaufEingabe{Wortabgleich: true}, "", company)
	if err != nil || st.Status != "laeuft" || st.Quelle != "wortabgleich" || st.Gesamt != len(lv.Positionen) || st.Fortschritt != 0 ||
		st.Schwelle == nil || *st.Schwelle != 0.7 || st.ProfilID != nil || st.GestartetAm == nil || st.BeendetAm != nil {
		t.Fatalf("Start: %+v %v", st, err)
	}
	// zweiter Start waehrend des Laufs
	if _, _, err := svc.Starten(ctx, imp, matching.LaufEingabe{Wortabgleich: true}, "", company); !errors.Is(err, matching.ErrLaufAktiv) {
		t.Fatalf("doppelt: %v", err)
	}
	if err := run(ctx); err != nil {
		t.Fatal(err)
	}
	st, _ = svc.Status(ctx, imp, company)
	if st.Status != "fertig" || st.Fortschritt != st.Gesamt || st.BeendetAm == nil || st.Fehler != "" {
		t.Fatalf("Ende: %+v", st)
	}

	// gleich dem Matcher direkt, mit Leistung und Abschrift
	k, ids, _ := store.LadeKatalog(ctx, company)
	want, err := matching.MitSystemvorgaben(matching.JePosition(matching.Wortabgleich{})).VorschlagenLV(ctx, lv, k)
	if err != nil {
		t.Fatal(err)
	}
	got := vorschlaege(t, env, imp)
	if len(got) != len(lv.Positionen) || len(positionen) != len(got) {
		t.Fatalf("Anzahl: %d von %d", len(got), len(lv.Positionen))
	}
	mitCode := 0
	for oz, w := range want {
		g := got[oz]
		if g.code != w.Code || math.Abs(g.score-math.Round(w.Score*1000)/1000) > 1e-9 || g.begruendung != w.Begruendung ||
			g.quelle != "wortabgleich" || g.profilID != nil || g.profilName != "" || g.schwelle != 0.7 || g.entscheidung != "offen" {
			t.Fatalf("%s: %+v statt %+v", oz, g, w)
		}
		if w.Code != "" {
			mitCode++
			if g.leistungID == nil || *g.leistungID != ids[w.Code] {
				t.Fatalf("%s: Leistung %v statt %s", oz, g.leistungID, ids[w.Code])
			}
		} else if g.leistungID != nil {
			t.Fatalf("%s: Leistung ohne Code", oz)
		}
	}
	if mitCode == 0 || mitCode == len(want) {
		t.Fatalf("Korpus prüft beide Fälle nicht: %d von %d mit Code", mitCode, len(want))
	}

	// Entscheidungen bleiben, offene werden ersetzt, entschiedene nicht erneut gefragt
	var verworfen, offen string
	for oz, g := range got {
		if g.code != "" && verworfen == "" {
			verworfen = oz
		} else if g.code != "" && offen == "" {
			offen = oz
		}
	}
	if _, err := env.PG.Exec(ctx, `UPDATE quote_import_item_matches SET entscheidung='verworfen', entschieden_at=now()
		WHERE quote_import_item_id=$1`, positionen[verworfen]); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE quote_import_item_matches SET code='ALT', leistung_id=NULL, begruendung='alt', systemvorgaben='{"rc_klasse":"RC9"}' WHERE quote_import_item_id=$1`,
		positionen[offen]); err != nil {
		t.Fatal(err)
	}
	st, run, err = svc.Starten(ctx, imp, matching.LaufEingabe{}, "", company) // ohne Sprachmodell: Wortabgleich
	if err != nil || st.Gesamt != len(lv.Positionen)-1 || st.Quelle != "wortabgleich" {
		t.Fatalf("zweiter Lauf: %+v %v", st, err)
	}
	if err := run(ctx); err != nil {
		t.Fatal(err)
	}
	nach := vorschlaege(t, env, imp)
	if nach[verworfen].entscheidung != "verworfen" || nach[verworfen].code != got[verworfen].code {
		t.Fatalf("entschieden verändert: %+v", nach[verworfen])
	}
	if a, b := nach[offen], got[offen]; a.code != b.code || a.score != b.score || a.begruendung != b.begruendung || a.systemvorgaben != b.systemvorgaben || a.leistungID == nil ||
		*a.leistungID != *b.leistungID || a.entscheidung != "offen" {
		t.Fatalf("offen nicht ersetzt: %+v statt %+v", nach[offen], got[offen])
	}

	// Entscheidung waehrend des Laufs (nach dem Start) bleibt stehen
	_, run, err = svc.Starten(ctx, imp, matching.LaufEingabe{Wortabgleich: true}, "", company)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE quote_import_item_matches SET entscheidung='geaendert', entschieden_code='ALT', entschieden_at=now(),
		code='ZWISCHEN' WHERE quote_import_item_id=$1`, positionen[offen]); err != nil {
		t.Fatal(err)
	}
	if err := run(ctx); err != nil {
		t.Fatal(err)
	}
	if z := vorschlaege(t, env, imp)[offen]; z.entscheidung != "geaendert" || z.code != "ZWISCHEN" {
		t.Fatalf("während des Laufs entschieden: %+v", z)
	}

	// Abbruch durch Neustart, danach neu startbar
	if _, _, err = svc.Starten(ctx, imp, matching.LaufEingabe{Wortabgleich: true}, "", company); err != nil {
		t.Fatal(err)
	}
	if n, err := svc.MarkiereAbgebrochen(ctx); err != nil || n < 1 {
		t.Fatalf("abbrechen: %d %v", n, err)
	}
	if st, _ := svc.Status(ctx, imp, company); st.Status != "abgebrochen" || st.BeendetAm == nil || !strings.Contains(st.Fehler, "Neustart") {
		t.Fatalf("abgebrochen: %+v", st)
	}
	if _, run, err = svc.Starten(ctx, imp, matching.LaufEingabe{Wortabgleich: true}, "", company); err != nil || run(ctx) != nil {
		t.Fatalf("nach Abbruch: %v", err)
	}
}

func TestLaufMitProfil(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	store := matching.NewStore(env.PG)
	company, imp := laufUmgebung(t, env)
	profil, err := llm.NewSettingsService(env.PG, nil).CreateProfile(ctx, llm.ProfileInput{Name: "Test"}, "", company)
	if err != nil {
		t.Fatal(err)
	}
	ab := &llm.ProfilAbschrift{ProfilID: profil.ID, Name: "Test", Anbieter: "fake", Modell: "fake-1", Schwelle: 0.85}
	fake := ersterKandidat(0)
	fp := &fakeProfile{client: llm.NewClient(fake, llm.Limits{Timeout: 5 * time.Second}, stillRecorder{}), ab: ab}
	svc := matching.NewLaufService(store, fp)

	if _, _, err := svc.Starten(ctx, imp, matching.LaufEingabe{ProfilID: profil.ID, Wortabgleich: true}, "", company); err == nil {
		t.Fatal("Profil und Wortabgleich zugleich")
	}
	st, run, err := svc.Starten(ctx, imp, matching.LaufEingabe{ProfilID: profil.ID}, "", company)
	if err != nil || st.Quelle != "modell" || st.ProfilID == nil || *st.ProfilID != profil.ID || st.ProfilName != "Test" || st.Modell != "fake-1" ||
		st.Anbieter != "fake" || *st.Schwelle != 0.85 {
		t.Fatalf("Start: %+v %v", st, err)
	}
	if err := run(ctx); err != nil {
		t.Fatal(err)
	}
	if fp.ids[len(fp.ids)-1] != profil.ID {
		t.Fatalf("gewähltes Profil: %v", fp.ids)
	}
	got := vorschlaege(t, env, imp)
	st, _ = svc.Status(ctx, imp, company)
	if st.Status != "fertig" || st.Fortschritt != st.Gesamt || len(got) != st.Gesamt {
		t.Fatalf("Ende: %+v %d", st, len(got))
	}
	// ein Aufruf je Buendel (Bereich, hoechstens 20 Positionen)
	lv, _, _ := store.LadeLV(ctx, imp, company)
	buendel, vorher := 0, ""
	n := 0
	for _, p := range lv.Positionen {
		if p.Bereich != vorher || n == matching.StandardBuendel {
			buendel, n = buendel+1, 0
		}
		vorher, n = p.Bereich, n+1
	}
	if len(fake.Requests) != buendel {
		t.Fatalf("Aufrufe: %d statt %d", len(fake.Requests), buendel)
	}
	// gleich dem Matcher direkt mit demselben Test-Anbieter
	k, ids, _ := store.LadeKatalog(ctx, company)
	direkt := matching.MitSystemvorgaben(matching.ModellMatcher{Client: llm.NewClient(ersterKandidat(0), llm.Limits{Timeout: 5 * time.Second}, stillRecorder{})})
	want, err := direkt.VorschlagenLV(ctx, lv, k)
	if err != nil || len(want) != len(got) {
		t.Fatalf("direkt: %d %v", len(want), err)
	}
	for oz, w := range want {
		g := got[oz]
		if g.quelle != "modell" || g.profilID == nil || *g.profilID != profil.ID || g.profilName != "Test" || g.modell != "fake-1" ||
			g.schwelle != 0.85 || g.code != w.Code || w.Code == "" || math.Abs(g.score-math.Round(w.Score*1000)/1000) > 1e-9 ||
			g.begruendung != w.Begruendung || !strings.HasPrefix(g.begruendung, "erster Kandidat") || g.leistungID == nil ||
			*g.leistungID != ids[w.Code] {
			t.Fatalf("%s: %+v statt %+v", oz, g, w)
		}
	}

	// Fehler im zweiten Buendel: erstes bleibt gespeichert, Status fehler
	if _, err := env.PG.Exec(ctx, `DELETE FROM quote_import_item_matches WHERE import_id=$1`, imp); err != nil {
		t.Fatal(err)
	}
	fp.client = llm.NewClient(ersterKandidat(2), llm.Limits{Timeout: 5 * time.Second}, stillRecorder{})
	_, run, err = svc.Starten(ctx, imp, matching.LaufEingabe{}, "", company) // Standardprofil
	if err != nil {
		t.Fatal(err)
	}
	if err := run(ctx); !errors.Is(err, llm.ErrDailyLimit) {
		t.Fatalf("Lauf mit Fehler: %v", err)
	}
	st, _ = svc.Status(ctx, imp, company)
	teil := len(vorschlaege(t, env, imp))
	if st.Status != "fehler" || !strings.Contains(st.Fehler, "Tageslimit") || st.Fortschritt == 0 || st.Fortschritt >= st.Gesamt ||
		teil != st.Fortschritt || st.BeendetAm == nil || fp.ids[len(fp.ids)-1] != "" {
		t.Fatalf("Fehler: %+v %d", st, teil)
	}

	// Profil gewaehlt, aber aus: Fehler ohne Belegung; ohne Angabe: Wortabgleich
	for _, e := range []error{llm.ErrNotEnabled, llm.ErrProfileNotFound} {
		fp.err = e
		if _, _, err := svc.Starten(ctx, imp, matching.LaufEingabe{ProfilID: profil.ID}, "", company); !errors.Is(err, e) {
			t.Fatalf("gewählt %v: %v", e, err)
		}
		if st, _ := svc.Status(ctx, imp, company); st.Status != "fehler" {
			t.Fatalf("Status verändert: %+v", st)
		}
	}
	fp.err = errors.New("API-Schlüssel des Sprachmodells ungültig")
	if _, _, err := svc.Starten(ctx, imp, matching.LaufEingabe{}, "", company); err == nil || !strings.Contains(err.Error(), "ungültig") {
		t.Fatalf("anderer Fehler ohne Wortabgleich: %v", err)
	}
	fp.err = llm.ErrProfileNotFound
	if st, run, err := svc.Starten(ctx, imp, matching.LaufEingabe{}, "", company); err != nil || st.Quelle != "wortabgleich" || run(ctx) != nil {
		t.Fatalf("ohne Standardprofil: %+v %v", st, err)
	}
}

func TestLaufVoraussetzungen(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	store := matching.NewStore(env.PG)
	svc := matching.NewLaufService(store, nil)
	company, imp := laufUmgebung(t, env)
	other, otherProject := storeMandant(t, env)
	leer := importiere(t, env, quotes.NewService(env.PG, nil), otherProject, other, "../quotes/eval/testdata/lv-02.x83")

	wort := matching.LaufEingabe{Wortabgleich: true}
	if _, _, err := svc.Starten(ctx, imp, wort, "", other); !errors.Is(err, matching.ErrImportNotFound) {
		t.Fatalf("fremd: %v", err)
	}
	if _, err := svc.Status(ctx, imp, other); !errors.Is(err, matching.ErrImportNotFound) {
		t.Fatalf("Status fremd: %v", err)
	}
	if _, _, err := svc.Starten(ctx, "kein-uuid", wort, "", company); !errors.Is(err, matching.ErrImportNotFound) {
		t.Fatalf("ungültig: %v", err)
	}
	if _, _, err := svc.Starten(ctx, leer, wort, "", other); err == nil || !strings.Contains(err.Error(), "Leistungskatalog ist leer") {
		t.Fatalf("leerer Katalog: %v", err)
	}
	if _, _, err := svc.Starten(ctx, imp, matching.LaufEingabe{}, "", company); err == nil || !strings.Contains(err.Error(), "nicht eingerichtet") {
		t.Fatalf("ohne Profilquelle: %v", err)
	}
	for _, status := range []string{"uploaded", "reviewed", "applied", "failed"} {
		if _, err := env.PG.Exec(ctx, `UPDATE quote_imports SET status=$2 WHERE id=$1`, imp, status); err != nil {
			t.Fatal(err)
		}
		if _, _, err := svc.Starten(ctx, imp, wort, "", company); err == nil || !strings.Contains(err.Error(), "nur für verarbeitete") {
			t.Fatalf("Status %s: %v", status, err)
		}
	}
	if st, _ := svc.Status(ctx, imp, company); st.Status != "" {
		t.Fatalf("nichts gestartet: %+v", st)
	}
}
