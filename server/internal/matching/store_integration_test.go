package matching_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/leistungen"
	"nalaerp3/internal/matching"
	"nalaerp3/internal/quotes"
	"nalaerp3/internal/quotes/gaeb"
	"nalaerp3/internal/testutil"
)

// LV und Katalog aus der Datenbank (Backlog I.5.2a) und die Pruefungen der
// Migration 123. Korpus aus dem Evaluationsset (erfundene LVs).

func storeMandant(t *testing.T, env *testutil.IntegrationEnv) (company, project string) {
	t.Helper()
	company = "mat-" + uuid.NewString()[:8]
	project = uuid.NewString()
	ctx := context.Background()
	if _, err := env.PG.Exec(ctx, `INSERT INTO company_profiles (id, name) VALUES ($1, $2)`, company, "Matching-Test "+company); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `INSERT INTO projects (id, nummer, name, company_id) VALUES ($1, $2, 'Matching', $3)`,
		project, "P-"+company, company); err != nil {
		t.Fatal(err)
	}
	return company, project
}

// importiere legt einen Importlauf an und speichert das Parserergebnis der
// Datei wie der Endpunkt "process".
func importiere(t *testing.T, env *testutil.IntegrationEnv, svc *quotes.Service, project, company, datei string) string {
	t.Helper()
	ctx := context.Background()
	f, err := os.Open(datei)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	res, err := quotes.GAEBDAXMLParser{}.ParseGAEB(ctx, f, filepath.Base(datei))
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	if _, err := env.PG.Exec(ctx, `INSERT INTO quote_imports (id, project_id, source_kind, source_filename, source_document_id, status)
		VALUES ($1, $2, 'gaeb', $3, 'test', 'uploaded')`, id, project, filepath.Base(datei)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveImportParse(ctx, id, res, company); err != nil {
		t.Fatal(err)
	}
	return id
}

func parseDatei(t *testing.T, datei string) *gaeb.LV {
	t.Helper()
	f, err := os.Open(datei)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	lv, err := gaeb.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	return lv
}

func TestStoreLadeLV(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := quotes.NewService(env.PG, nil)
	store := matching.NewStore(env.PG)
	company, project := storeMandant(t, env)
	other, _ := storeMandant(t, env)

	dateien, err := filepath.Glob("../quotes/eval/testdata/lv-*.x83")
	if err != nil || len(dateien) < 10 {
		t.Fatalf("Korpus: %v %v", dateien, err)
	}
	var erstes string
	for _, datei := range dateien {
		id := importiere(t, env, svc, project, company, datei)
		if erstes == "" {
			erstes = id
		}
		want := parseDatei(t, datei)
		want.Format, want.Phase = "", ""
		if want.Vorbemerkungen == nil {
			want.Vorbemerkungen = []string{}
		}
		got, pos, err := store.LadeLV(ctx, id, company)
		if err != nil {
			t.Fatalf("%s: %v", datei, err)
		}
		// LV ohne Bereiche: leer und nil sind gleichwertig
		if len(want.Bereiche) == 0 && len(got.Bereiche) == 0 {
			want.Bereiche, got.Bereiche = nil, nil
		}
		if !reflect.DeepEqual(got.Vorbemerkungen, want.Vorbemerkungen) || !reflect.DeepEqual(got.Bereiche, want.Bereiche) ||
			len(got.Positionen) != len(want.Positionen) {
			t.Fatalf("%s: LV weicht ab\n got %q %+v %d\nwant %q %+v %d", datei, got.Vorbemerkungen, got.Bereiche, len(got.Positionen),
				want.Vorbemerkungen, want.Bereiche, len(want.Positionen))
		}
		for i := range want.Positionen {
			if !reflect.DeepEqual(got.Positionen[i], want.Positionen[i]) {
				t.Fatalf("%s: Position %d weicht ab\n got %#v\nwant %#v", datei, i, got.Positionen[i], want.Positionen[i])
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: LV weicht ab", datei)
		}
		if len(pos) != len(want.Positionen) {
			t.Fatalf("%s: Positionen %d statt %d", datei, len(pos), len(want.Positionen))
		}
		for _, p := range want.Positionen {
			var oz string
			if err := env.PG.QueryRow(ctx, `SELECT position_no FROM quote_import_items WHERE id=$1 AND import_id=$2`, pos[p.OZ], id).Scan(&oz); err != nil || oz != p.OZ {
				t.Fatalf("%s: Position %s -> %q: %q %v", datei, p.OZ, pos[p.OZ], oz, err)
			}
		}
	}

	// fremder Mandant, unbekannt, ungueltig, ohne Mandant
	for _, c := range []struct{ id, company string }{{erstes, other}, {uuid.NewString(), company}, {"kein-uuid", company}} {
		if _, _, err := store.LadeLV(ctx, c.id, c.company); !errors.Is(err, matching.ErrImportNotFound) {
			t.Fatalf("%+v: %v", c, err)
		}
	}
	if _, _, err := store.LadeLV(ctx, erstes, " "); err == nil || errors.Is(err, matching.ErrImportNotFound) {
		t.Fatalf("ohne Mandant: %v", err)
	}
	// nur GAEB-Importe
	if _, err := env.PG.Exec(ctx, `UPDATE quote_imports SET source_kind='csv' WHERE id=$1`, erstes); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.LadeLV(ctx, erstes, company); !errors.Is(err, matching.ErrImportNotFound) {
		t.Fatalf("kein GAEB: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE quote_imports SET source_kind='gaeb' WHERE id=$1`, erstes); err != nil {
		t.Fatal(err)
	}
	// Import vor Migration 116: Beschreibung als Kurztext
	var oz, beschreibung string
	if err := env.PG.QueryRow(ctx, `UPDATE quote_import_items SET short_text='', long_text='' WHERE id=(
		SELECT id FROM quote_import_items WHERE import_id=$1 ORDER BY sort_order LIMIT 1) RETURNING position_no, description`, erstes).
		Scan(&oz, &beschreibung); err != nil {
		t.Fatal(err)
	}
	lv, _, err := store.LadeLV(ctx, erstes, company)
	if err != nil || lv.Positionen[0].OZ != oz || lv.Positionen[0].Kurztext != beschreibung || lv.Positionen[0].Langtext != "" {
		t.Fatalf("alte Position: %+v %v", lv.Positionen[0], err)
	}
	if lv.Positionen[1].Kurztext == "" || strings.Contains(lv.Positionen[1].Kurztext, "\n\n") {
		t.Fatalf("übrige Positionen mit Kurztext: %+v", lv.Positionen[1])
	}
	// doppelte Ordnungszahl (nur ohne Parser moeglich)
	if _, err := env.PG.Exec(ctx, `UPDATE quote_import_items SET position_no=$2 WHERE import_id=$1 AND position_no<>$2 AND sort_order=(
		SELECT max(sort_order) FROM quote_import_items WHERE import_id=$1)`, erstes, oz); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.LadeLV(ctx, erstes, company); err == nil || err.Error() != "Ordnungszahl "+oz+" ungültig: doppelt" {
		t.Fatalf("doppelt: %v", err)
	}
	if _, err := env.PG.Exec(ctx, `DELETE FROM quote_import_items WHERE import_id=$1`, erstes); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.LadeLV(ctx, erstes, company); err == nil || err.Error() != "Im Importlauf fehlen Positionen" {
		t.Fatalf("ohne Positionen: %v", err)
	}
}

func TestStoreLadeKatalog(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	lst := leistungen.NewService(env.PG)
	store := matching.NewStore(env.PG)
	a, _ := storeMandant(t, env)
	b, _ := storeMandant(t, env)

	if k, ids, err := store.LadeKatalog(ctx, a); err != nil || k.Leistungen == nil || len(k.Leistungen) != 0 || len(ids) != 0 {
		t.Fatalf("leer: %+v %v %v", k, ids, err)
	}
	u := 1.1
	aus := false
	fe, err := lst.Anlegen(ctx, leistungen.Eingabe{Code: "FE-2", Kurztext: "Fenster", Einheit: "Stk", Gewerk: "fenster", Profilserie: "A75",
		Verglasung: "3-fach", RCKlasse: "RC2", UWert: &u, Brandschutzklasse: "T30"}, "", a)
	if err != nil {
		t.Fatal(err)
	}
	de, err := lst.Anlegen(ctx, leistungen.Eingabe{Code: "DE-1", Kurztext: "Demontage", Einheit: "Stk"}, "", a)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		e       leistungen.Eingabe
		company string
	}{
		{leistungen.Eingabe{Code: "AUS", Kurztext: "abgeschaltet", Einheit: "Stk", Aktiv: &aus}, a},
		{leistungen.Eingabe{Code: "FREMD", Kurztext: "anderer Mandant", Einheit: "Stk"}, b},
	} {
		if _, err := lst.Anlegen(ctx, c.e, "", c.company); err != nil {
			t.Fatal(err)
		}
	}
	k, ids, err := store.LadeKatalog(ctx, a)
	want := &matching.Katalog{Leistungen: []matching.Leistung{
		{Code: "DE-1", Kurztext: "Demontage", Einheit: "Stk"},
		{Code: "FE-2", Kurztext: "Fenster", Einheit: "Stk", Gewerk: "fenster", Systemvorgaben: matching.Systemvorgaben{
			Profilserie: "A75", Verglasung: "3-fach", RCKlasse: "RC2", UWert: &u, Brandschutzklasse: "T30"}},
	}}
	if err != nil || !reflect.DeepEqual(k, want) || !reflect.DeepEqual(ids, map[string]string{"DE-1": de.ID, "FE-2": fe.ID}) {
		t.Fatalf("Katalog: %+v %v %v", k, ids, err)
	}
	if err := k.Validate(); err != nil {
		t.Fatalf("Katalog gültig: %v", err)
	}
	if _, _, err := store.LadeKatalog(ctx, ""); err == nil {
		t.Fatal("ohne Mandant")
	}
}

func TestMigration123Pruefungen(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := quotes.NewService(env.PG, nil)
	company, project := storeMandant(t, env)
	imp := importiere(t, env, svc, project, company, "../quotes/eval/testdata/lv-01.x83")
	var item string
	if err := env.PG.QueryRow(ctx, `SELECT id::text FROM quote_import_items WHERE import_id=$1 ORDER BY sort_order LIMIT 1`, imp).Scan(&item); err != nil {
		t.Fatal(err)
	}
	insert := func(spalten, werte string) error {
		_, err := env.PG.Exec(ctx, `INSERT INTO quote_import_item_matches (quote_import_item_id, import_id, quelle, schwelle`+spalten+`)
			VALUES ($1, $2, 'wortabgleich', 0.7`+werte+`)`, item, imp)
		if err == nil {
			_, _ = env.PG.Exec(ctx, `DELETE FROM quote_import_item_matches WHERE quote_import_item_id=$1`, item)
		}
		return err
	}
	// gueltig: offen ohne Vorschlag, bestaetigt mit Code, geaendert mit Code, verworfen
	for _, c := range []struct{ spalten, werte string }{
		{"", ""},
		{", code, score, entscheidung, entschieden_at", ", 'FE-1', 1, 'bestaetigt', now()"},
		{", entscheidung, entschieden_code, entschieden_at", ", 'geaendert', 'FE-2', now()"},
		{", code, entscheidung, entschieden_at", ", 'FE-1', 'verworfen', now()"},
		{", systemvorgaben", `, '{"rc_klasse":"RC2"}'`},
	} {
		if err := insert(c.spalten, c.werte); err != nil {
			t.Fatalf("gültig %s: %v", c.werte, err)
		}
	}
	for name, c := range map[string]struct{ spalten, werte string }{
		"Score über 1":                {", score", ", 1.001"},
		"Score negativ":               {", score", ", -0.001"},
		"Code zu lang":                {", code", ", repeat('x', 51)"},
		"Begründung zu lang":          {", begruendung", ", repeat('x', 2001)"},
		"Systemvorgaben als Liste":    {", systemvorgaben", ", '[]'"},
		"bestätigt ohne Code":         {", entscheidung, entschieden_at", ", 'bestaetigt', now()"},
		"geändert ohne Code":          {", entscheidung, entschieden_at", ", 'geaendert', now()"},
		"offen mit Zeitpunkt":         {", entschieden_at", ", now()"},
		"entschieden ohne Zeitpunkt":  {", code, entscheidung", ", 'FE-1', 'verworfen'"},
		"offen mit geändertem Code":   {", entschieden_code", ", 'FE-2'"},
		"unbekannte Entscheidung":     {", entscheidung, entschieden_at", ", 'vielleicht', now()"},
		"Wortabgleich mit Profilname": {", profil_name", ", 'Claude'"},
	} {
		if err := insert(c.spalten, c.werte); err == nil {
			t.Fatalf("%s: angenommen", name)
		}
	}
	if _, err := env.PG.Exec(ctx, `INSERT INTO quote_import_item_matches (quote_import_item_id, import_id, quelle, schwelle)
		VALUES ($1, $2, 'modell', 0.9)`, item, imp); err == nil {
		t.Fatal("Modell ohne Profilname angenommen")
	}
	if _, err := env.PG.Exec(ctx, `INSERT INTO quote_import_item_matches (quote_import_item_id, import_id, quelle, schwelle, profil_name, code, score)
		VALUES ($1, $2, 'modell', 0.9, 'Claude', 'FE-1', 0.95)`, item, imp); err != nil {
		t.Fatal(err)
	}
	// erneutes Verarbeiten legt die Positionen neu an: Vorschlag verschwindet
	importiereErneut(t, env, svc, imp, company)
	var n int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM quote_import_item_matches WHERE import_id=$1`, imp).Scan(&n); err != nil || n != 0 {
		t.Fatalf("nach erneutem Verarbeiten: %d %v", n, err)
	}

	// Status des Laufs am Import
	ok := `UPDATE quote_imports SET match_status='laeuft', match_quelle='modell', match_gestartet_at=now(), match_gesamt=10,
		match_fortschritt=10, match_schwelle=1 WHERE id=$1`
	if _, err := env.PG.Exec(ctx, ok, imp); err != nil {
		t.Fatalf("Status gültig: %v", err)
	}
	for name, sql := range map[string]string{
		"unbekannter Status":      `UPDATE quote_imports SET match_status='pausiert' WHERE id=$1`,
		"Status ohne Start":       `UPDATE quote_imports SET match_gestartet_at=NULL WHERE id=$1`,
		"Start ohne Status":       `UPDATE quote_imports SET match_status='' WHERE id=$1`,
		"Status ohne Quelle":      `UPDATE quote_imports SET match_quelle='' WHERE id=$1`,
		"unbekannte Quelle":       `UPDATE quote_imports SET match_quelle='raten' WHERE id=$1`,
		"Fortschritt über Gesamt": `UPDATE quote_imports SET match_fortschritt=11 WHERE id=$1`,
		"Fortschritt negativ":     `UPDATE quote_imports SET match_fortschritt=-1 WHERE id=$1`,
		"Schwelle über 1":         `UPDATE quote_imports SET match_schwelle=1.001 WHERE id=$1`,
		"Fehlermeldung zu lang":   `UPDATE quote_imports SET match_fehler=repeat('x', 2001) WHERE id=$1`,
	} {
		if _, err := env.PG.Exec(ctx, sql, imp); err == nil {
			t.Fatalf("%s: angenommen", name)
		}
	}
}

func importiereErneut(t *testing.T, env *testutil.IntegrationEnv, svc *quotes.Service, imp, company string) {
	t.Helper()
	f, err := os.Open("../quotes/eval/testdata/lv-01.x83")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	res, err := quotes.GAEBDAXMLParser{}.ParseGAEB(context.Background(), f, "lv-01.x83")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveImportParse(context.Background(), imp, res, company); err != nil {
		t.Fatal(err)
	}
}
