package matching_test

import (
	"context"
	"strings"
	"testing"

	"nalaerp3/internal/llm"
	"nalaerp3/internal/matching"
	"nalaerp3/internal/testutil"
)

// Gruende beim Aendern und Verwerfen (Backlog I.8.3a). Erfundene LVs.

func TestGruende(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	store := matching.NewStore(env.PG)
	svc := matching.NewLaufService(store, &fakeProfile{err: llm.ErrNotEnabled})
	company, imp := laufUmgebung(t, env)
	_, ids, _ := store.LadeKatalog(ctx, company)
	_, run, err := svc.Starten(ctx, imp, matching.LaufEingabe{Wortabgleich: true}, "", company)
	if err != nil || run(ctx) != nil {
		t.Fatalf("Lauf: %v", err)
	}
	liste, _ := svc.Pruefliste(ctx, imp, company)
	var mitCode []matching.PruefZeile
	for _, z := range liste {
		if z.Grund != "" || z.GrundText != "" {
			t.Fatalf("Grund vor der Entscheidung: %+v", z)
		}
		if z.LeistungID != nil {
			mitCode = append(mitCode, z)
		}
	}
	if len(mitCode) < 2 {
		t.Fatalf("zu wenige Vorschläge: %d", len(mitCode))
	}
	entscheide := func(item string, in matching.EntscheidungEingabe) (*matching.PruefZeile, error) {
		return svc.Entscheiden(ctx, imp, item, in, "", company)
	}
	item := mitCode[0].ItemID

	// Eingaben (abgelehnt, ohne etwas zu speichern)
	for _, c := range []struct {
		in   matching.EntscheidungEingabe
		want string
	}{
		{matching.EntscheidungEingabe{Aktion: "verwerfen", Grund: "gefaellt_nicht"}, "Grund ungültig (falsche_leistung"},
		{matching.EntscheidungEingabe{Aktion: "verwerfen", GrundText: "ohne Grund"}, "Freitext nur mit Grund"},
		{matching.EntscheidungEingabe{Aktion: "bestaetigen", Grund: "sonstiges"}, "nur beim Ändern oder Verwerfen"},
		{matching.EntscheidungEingabe{Aktion: "verwerfen", Grund: "sonstiges", GrundText: strings.Repeat("ä", 1001)}, "höchstens 1000 Zeichen"},
	} {
		if _, err := entscheide(item, c.in); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%+v: %v", c.in.Grund, err)
		}
	}
	liste, _ = svc.Pruefliste(ctx, imp, company)
	for _, z := range liste {
		if z.ItemID == item && z.Entscheidung != "offen" {
			t.Fatalf("abgelehnte Eingabe gespeichert: %+v", z)
		}
	}

	// verwerfen mit Grund und Freitext (getrimmt); 1000 Zeichen gehen
	z, err := entscheide(item, matching.EntscheidungEingabe{Aktion: "verwerfen", Grund: " keine_passende_leistung ", GrundText: " nicht im Katalog "})
	if err != nil || z.Grund != "keine_passende_leistung" || z.GrundText != "nicht im Katalog" {
		t.Fatalf("verwerfen: %+v %v", z, err)
	}
	lang := strings.Repeat("ä", 1000)
	z, err = entscheide(item, matching.EntscheidungEingabe{Aktion: "aendern", LeistungID: ids["FE-ALU-1F"], Grund: "systemvorgabe_falsch", GrundText: lang})
	if err != nil || z.Entscheidung != "geaendert" || z.Grund != "systemvorgabe_falsch" || z.GrundText != lang {
		t.Fatalf("ändern: %+v %v", z, err)
	}
	// ohne Grund bleibt erlaubt und ersetzt den alten
	z, err = entscheide(item, matching.EntscheidungEingabe{Aktion: "verwerfen"})
	if err != nil || z.Grund != "" || z.GrundText != "" {
		t.Fatalf("verwerfen ohne Grund: %+v %v", z, err)
	}
	// bestaetigen loescht den Grund
	if _, err := entscheide(item, matching.EntscheidungEingabe{Aktion: "verwerfen", Grund: "falsche_leistung"}); err != nil {
		t.Fatal(err)
	}
	z, err = entscheide(item, matching.EntscheidungEingabe{Aktion: "bestaetigen"})
	if err != nil || z.Entscheidung != "bestaetigt" || z.Grund != "" || z.GrundText != "" {
		t.Fatalf("bestätigen: %+v %v", z, err)
	}
	// jeder Grund der Auswahl geht durch die Datenbank; feste Liste, weil
	// der Client dieselben Werte kennt
	for _, g := range []string{"falsche_leistung", "systemvorgabe_falsch", "einheit_menge", "keine_passende_leistung", "sonstiges"} {
		if z, err := entscheide(mitCode[1].ItemID, matching.EntscheidungEingabe{Aktion: "verwerfen", Grund: g}); err != nil || z.Grund != g {
			t.Fatalf("Grund %s: %+v %v", g, z, err)
		}
	}
	// die Pruefliste liefert den Grund
	liste, _ = svc.Pruefliste(ctx, imp, company)
	for _, z := range liste {
		if z.ItemID == mitCode[1].ItemID && z.Grund != "sonstiges" {
			t.Fatalf("Prüfliste: %+v", z)
		}
	}
	// die Datenbank sichert dasselbe ab
	for _, sql := range []string{
		`UPDATE quote_import_item_matches SET grund='x' WHERE quote_import_item_id=$1`,
		`UPDATE quote_import_item_matches SET grund='sonstiges' WHERE quote_import_item_id=$1 AND entscheidung='bestaetigt'`,
		`UPDATE quote_import_item_matches SET grund='', grund_text='t' WHERE quote_import_item_id=$1`,
	} {
		target := mitCode[1].ItemID
		if strings.Contains(sql, "bestaetigt") {
			target = item
		}
		if tag, err := env.PG.Exec(ctx, sql, target); err == nil {
			t.Fatalf("Check fehlt (%d Zeilen): %s", tag.RowsAffected(), sql)
		}
	}
}
