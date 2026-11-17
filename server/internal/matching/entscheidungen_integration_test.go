package matching_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/leistungen"
	"nalaerp3/internal/llm"
	"nalaerp3/internal/matching"
	"nalaerp3/internal/quotes"
	"nalaerp3/internal/testutil"
)

// Pruefliste, Entscheidungen, Sammelbestaetigung und Freigabe (Backlog
// I.5.2c). Erfundene LVs, erfundener Benutzer.

func TestEntscheidungen(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	store := matching.NewStore(env.PG)
	svc := matching.NewLaufService(store, &fakeProfile{err: llm.ErrNotEnabled})
	company, imp := laufUmgebung(t, env)
	other, otherProject := storeMandant(t, env)
	user := testutil.SeedAuthUser(t, env, "entscheid-"+uuid.NewString()[:8]+"@example.test", "Secret123!", "sales")
	lv, positionen, _ := store.LadeLV(ctx, imp, company)
	_, ids, _ := store.LadeKatalog(ctx, company)

	// vor dem Lauf: alle Positionen, ohne Vorschlag
	liste, err := svc.Pruefliste(ctx, imp, company)
	if err != nil || len(liste) != len(lv.Positionen) {
		t.Fatalf("Prüfliste vor dem Lauf: %d %v", len(liste), err)
	}
	for i, z := range liste {
		if z.PositionNo != lv.Positionen[i].OZ || z.Kurztext != lv.Positionen[i].Kurztext || z.Quelle != "" || z.Entscheidung != "" ||
			z.ItemID != positionen[z.PositionNo] || string(z.Systemvorgaben) != "{}" {
			t.Fatalf("Zeile %d: %+v", i, z)
		}
	}
	if _, err := svc.Entscheiden(ctx, imp, liste[0].ItemID, matching.EntscheidungEingabe{Aktion: "verwerfen"}, "", company); err == nil ||
		!strings.Contains(err.Error(), "fehlt ein Vorschlag") {
		t.Fatalf("ohne Vorschlag: %v", err)
	}
	_, run, err := svc.Starten(ctx, imp, matching.LaufEingabe{Wortabgleich: true}, "", company)
	if err != nil || run(ctx) != nil {
		t.Fatalf("Lauf: %v", err)
	}
	liste, _ = svc.Pruefliste(ctx, imp, company)
	var mitCode, ohneCode, ueber, unter []matching.PruefZeile
	for _, z := range liste {
		if z.Quelle != "wortabgleich" || z.Entscheidung != "offen" || z.Schwelle != 0.7 || z.UeberSchwelle != (z.Code != "" && z.Score >= 0.7) {
			t.Fatalf("nach dem Lauf: %+v", z)
		}
		switch {
		case z.Code == "":
			ohneCode = append(ohneCode, z)
		case z.UeberSchwelle:
			mitCode, ueber = append(mitCode, z), append(ueber, z)
		default:
			mitCode, unter = append(mitCode, z), append(unter, z)
		}
	}
	if len(ohneCode) == 0 || len(ueber) < 3 || len(unter) < 2 {
		t.Fatalf("Korpus deckt die Fälle nicht ab: ohne %d, über %d, unter %d", len(ohneCode), len(ueber), len(unter))
	}

	entscheide := func(item string, in matching.EntscheidungEingabe) (*matching.PruefZeile, error) {
		return svc.Entscheiden(ctx, imp, item, in, user, company)
	}
	// Eingaben
	for _, c := range []struct {
		in   matching.EntscheidungEingabe
		want string
	}{
		{matching.EntscheidungEingabe{Aktion: "annehmen"}, "Aktion ungültig"},
		{matching.EntscheidungEingabe{Aktion: ""}, "Aktion ungültig"},
		{matching.EntscheidungEingabe{Aktion: "aendern"}, "nur und immer beim Ändern"},
		{matching.EntscheidungEingabe{Aktion: "bestaetigen", LeistungID: ids["FE-ALU-1F"]}, "nur und immer beim Ändern"},
		{matching.EntscheidungEingabe{Aktion: "verwerfen", LeistungID: "x"}, "nur und immer beim Ändern"},
	} {
		if _, err := entscheide(mitCode[0].ItemID, c.in); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%+v: %v", c.in, err)
		}
	}
	// bestaetigen
	z, err := entscheide(unter[0].ItemID, matching.EntscheidungEingabe{Aktion: " bestaetigen "})
	if err != nil || z.Entscheidung != "bestaetigt" || z.EntschiedenAm == nil || z.EntschiedenVonUserID == nil || *z.EntschiedenVonUserID != user ||
		z.Code != unter[0].Code || z.EntschiedenCode != "" {
		t.Fatalf("bestätigen: %+v %v", z, err)
	}
	if _, err := entscheide(ohneCode[0].ItemID, matching.EntscheidungEingabe{Aktion: "bestaetigen"}); err == nil ||
		!strings.Contains(err.Error(), "sonst ändern") {
		t.Fatalf("bestätigen ohne Code: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE quote_import_item_matches SET leistung_id=NULL WHERE quote_import_item_id=$1`, ueber[0].ItemID); err != nil {
		t.Fatal(err)
	}
	if _, err := entscheide(ueber[0].ItemID, matching.EntscheidungEingabe{Aktion: "bestaetigen"}); err == nil {
		t.Fatal("bestätigen ohne Leistung angenommen")
	}
	// aendern: nur aktive Leistung des Mandanten
	lst := leistungen.NewService(env.PG)
	aus := false
	abgeschaltet, err := lst.Anlegen(ctx, leistungen.Eingabe{Code: "AUS-1", Kurztext: "abgeschaltet", Einheit: "Stk", Aktiv: &aus}, "", company)
	if err != nil {
		t.Fatal(err)
	}
	fremd, err := lst.Anlegen(ctx, leistungen.Eingabe{Code: "FE-1", Kurztext: "fremd", Einheit: "Stk"}, "", other)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{abgeschaltet.ID, fremd.ID, "gibt-es-nicht"} {
		if _, err := entscheide(ohneCode[0].ItemID, matching.EntscheidungEingabe{Aktion: "aendern", LeistungID: id}); err == nil ||
			!strings.Contains(err.Error(), "nicht gefunden oder abgeschaltet") {
			t.Fatalf("ändern auf %s: %v", id, err)
		}
	}
	z, err = entscheide(ohneCode[0].ItemID, matching.EntscheidungEingabe{Aktion: "aendern", LeistungID: ids["FE-ALU-1F"]})
	if err != nil || z.Entscheidung != "geaendert" || z.EntschiedenCode != "FE-ALU-1F" || z.EntschiedenLeistungID == nil || *z.EntschiedenLeistungID != ids["FE-ALU-1F"] ||
		z.Code != "" {
		t.Fatalf("ändern: %+v %v", z, err)
	}
	// erneut entscheiden: verwerfen loescht die Aenderung
	z, err = entscheide(ohneCode[0].ItemID, matching.EntscheidungEingabe{Aktion: "verwerfen"})
	if err != nil || z.Entscheidung != "verworfen" || z.EntschiedenCode != "" || z.EntschiedenLeistungID != nil {
		t.Fatalf("verwerfen nach ändern: %+v %v", z, err)
	}
	z, err = entscheide(ohneCode[0].ItemID, matching.EntscheidungEingabe{Aktion: "aendern", LeistungID: ids["FE-ALU-1F"]})
	if err != nil || z.Entscheidung != "geaendert" {
		t.Fatalf("ändern nach verwerfen: %+v %v", z, err)
	}
	if z, err = entscheide(unter[0].ItemID, matching.EntscheidungEingabe{Aktion: "aendern", LeistungID: ids["FE-ALU-1F"]}); err != nil ||
		z.EntschiedenLeistungID == nil {
		t.Fatalf("ändern nach bestätigen: %+v %v", z, err)
	}
	if z, err = entscheide(unter[0].ItemID, matching.EntscheidungEingabe{Aktion: "bestaetigen"}); err != nil || z.Entscheidung != "bestaetigt" ||
		z.EntschiedenLeistungID != nil || z.EntschiedenCode != "" {
		t.Fatalf("bestätigen nach ändern: %+v %v", z, err)
	}
	// unbekannte Position, Position eines anderen Imports, fremder Mandant
	zweiter := importiere(t, env, quotes.NewService(env.PG, nil), otherProject, other, "../quotes/eval/testdata/lv-02.x83")
	var fremdeItem string
	if err := env.PG.QueryRow(ctx, `SELECT id::text FROM quote_import_items WHERE import_id=$1 LIMIT 1`, zweiter).Scan(&fremdeItem); err != nil {
		t.Fatal(err)
	}
	for _, item := range []string{uuid.NewString(), "kein-uuid", fremdeItem} {
		if _, err := entscheide(item, matching.EntscheidungEingabe{Aktion: "verwerfen"}); !errors.Is(err, matching.ErrItemNotFound) {
			t.Fatalf("Position %s: %v", item, err)
		}
	}
	if _, err := svc.Entscheiden(ctx, imp, mitCode[0].ItemID, matching.EntscheidungEingabe{Aktion: "verwerfen"}, "", other); !errors.Is(err, matching.ErrImportNotFound) {
		t.Fatalf("fremder Mandant: %v", err)
	}
	if _, err := svc.Pruefliste(ctx, imp, other); !errors.Is(err, matching.ErrImportNotFound) {
		t.Fatalf("Prüfliste fremd: %v", err)
	}
	if _, err := svc.SammelBestaetigen(ctx, imp, "", other); !errors.Is(err, matching.ErrImportNotFound) {
		t.Fatalf("Sammel fremd: %v", err)
	}

	// Sammelbestaetigung: offen, ueber der Schwelle, mit Leistung
	n, err := svc.SammelBestaetigen(ctx, imp, user, company)
	if err != nil || n != int64(len(ueber)-1) {
		t.Fatalf("Sammel: %d statt %d %v", n, len(ueber)-1, err)
	}
	nachher := map[string]matching.PruefZeile{}
	liste, _ = svc.Pruefliste(ctx, imp, company)
	for _, z := range liste {
		nachher[z.ItemID] = z
	}
	for _, z := range ueber[1:] {
		if g := nachher[z.ItemID]; g.Entscheidung != "bestaetigt" || g.EntschiedenVonUserID == nil || *g.EntschiedenVonUserID != user {
			t.Fatalf("über der Schwelle: %+v", g)
		}
	}
	if nachher[ueber[0].ItemID].Entscheidung != "offen" || nachher[unter[1].ItemID].Entscheidung != "offen" {
		t.Fatalf("ohne Leistung oder unter der Schwelle bestätigt: %+v %+v", nachher[ueber[0].ItemID], nachher[unter[1].ItemID])
	}
	if n, err := svc.SammelBestaetigen(ctx, imp, "", company); err != nil || n != 0 {
		t.Fatalf("Sammel erneut: %d %v", n, err)
	}

	// Freigabe des Importlaufs: nur ohne offene Vorschlaege und ohne laufenden Lauf
	qs := quotes.NewService(env.PG, nil)
	if _, err := env.PG.Exec(ctx, `UPDATE quote_import_items SET review_status='accepted' WHERE import_id=$1`, imp); err != nil {
		t.Fatal(err)
	}
	offen := 0
	for _, z := range liste {
		if z.Entscheidung == "offen" && z.Code != "" {
			offen++
		}
	}
	if _, err := qs.MarkImportReviewed(ctx, imp, company); err == nil ||
		err.Error() != "Importlauf enthält noch offene Vorschläge der Zuordnung ("+strconv.Itoa(offen)+")" {
		t.Fatalf("Freigabe mit offenen: %v", err)
	}
	for _, z := range liste {
		if z.Entscheidung == "offen" && z.Code != "" {
			if _, err := entscheide(z.ItemID, matching.EntscheidungEingabe{Aktion: "verwerfen"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := env.PG.Exec(ctx, `UPDATE quote_imports SET match_status='laeuft' WHERE id=$1`, imp); err != nil {
		t.Fatal(err)
	}
	if _, err := qs.MarkImportReviewed(ctx, imp, company); err == nil || !strings.Contains(err.Error(), "läuft bereits") {
		t.Fatalf("Freigabe während des Laufs: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE quote_imports SET match_status='fertig' WHERE id=$1`, imp); err != nil {
		t.Fatal(err)
	}
	// Positionen ohne Vorschlag (offen, Code leer) sind zulaessig
	if g, err := qs.MarkImportReviewed(ctx, imp, company); err != nil || g.Status != "reviewed" {
		t.Fatalf("Freigabe: %+v %v", g, err)
	}
	if _, err := entscheide(mitCode[1].ItemID, matching.EntscheidungEingabe{Aktion: "verwerfen"}); err == nil ||
		!strings.Contains(err.Error(), "nicht freigegebene") {
		t.Fatalf("nach Freigabe entschieden: %v", err)
	}
	if _, err := svc.SammelBestaetigen(ctx, imp, "", company); err == nil {
		t.Fatal("Sammel nach Freigabe")
	}
}

// Ohne Lauf bleibt die Freigabe wie bisher.
func TestFreigabeOhneLauf(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company, project := storeMandant(t, env)
	qs := quotes.NewService(env.PG, nil)
	imp := importiere(t, env, qs, project, company, "../quotes/eval/testdata/lv-03.x83")
	if _, err := qs.MarkImportReviewed(ctx, imp, company); err == nil || !strings.Contains(err.Error(), "offene Review-Positionen") {
		t.Fatalf("Review offen: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE quote_import_items SET review_status='rejected' WHERE import_id=$1`, imp); err != nil {
		t.Fatal(err)
	}
	if g, err := qs.MarkImportReviewed(ctx, imp, company); err != nil || g.Status != "reviewed" {
		t.Fatalf("Freigabe: %+v %v", g, err)
	}
}
