package hr

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Katalog der Qualifikationsarten gegen echtes Postgres (Backlog F.2.3).
//
// Jeder Test arbeitet in einem eigenen, frisch angelegten Mandanten: der
// Katalog ist mandantenweit, und Tests, die ihn zaehlen oder den
// Vorschlagskatalog uebernehmen, duerfen einander nicht sehen.

func seedCatalogCompany(t *testing.T, env *testutil.IntegrationEnv) string {
	t.Helper()
	id := "qt-" + uuid.NewString()[:12]
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO company_profiles (id, name) VALUES ($1, $2)`, id, "Katalogtest "+id); err != nil {
		t.Fatalf("Mandant anlegen: %v", err)
	}
	return id
}

func intPtr(n int) *int { return &n }

func TestQualificationTypeCreateGetListUpdateFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewQualificationTypeService(env.PG)
	company := seedCatalogCompany(t, env)

	// Der Code wird normalisiert: ohne Rand, in Grossbuchstaben.
	created, err := svc.Create(ctx, QualificationTypeInput{
		Code: "  schweisser_135 ", Name: "  Schweißerprüfung MAG  ", Kategorie: QualificationKategorieQualifikation,
		GueltigkeitMonate: intPtr(36), BestaetigungMonate: intPtr(6), Hinweis: "DIN EN ISO 9606-1",
	}, company)
	if err != nil {
		t.Fatalf("Anlegen: %v", err)
	}
	if created.Code != "SCHWEISSER_135" || created.Name != "Schweißerprüfung MAG" {
		t.Fatalf("Code/Name nicht normalisiert: %+v", created)
	}
	if !created.Aktiv || created.PflichtFuerAlle || created.GueltigkeitMonate == nil || *created.GueltigkeitMonate != 36 {
		t.Fatalf("erwartet aktive Art mit 36 Monaten, nicht Pflicht fuer alle: %+v", created)
	}

	got, err := svc.Get(ctx, created.ID, company)
	if err != nil {
		t.Fatalf("Laden: %v", err)
	}
	if got.Code != "SCHWEISSER_135" || got.Hinweis != "DIN EN ISO 9606-1" || got.Kategorie != QualificationKategorieQualifikation ||
		got.BestaetigungMonate == nil || *got.BestaetigungMonate != 6 {
		t.Fatalf("gespeicherte Art nicht wie angelegt: %+v", got)
	}

	// Aendern ersetzt alle aenderbaren Angaben - auch die Frist auf "keine".
	updated, err := svc.Update(ctx, created.ID, QualificationTypeUpdate{
		Name: "Schweißerprüfung MAG (135)", Kategorie: QualificationKategorieQualifikation,
		GueltigkeitMonate: nil, PflichtFuerAlle: true, Aktiv: true, Hinweis: "geaendert",
	}, company)
	if err != nil {
		t.Fatalf("Aendern: %v", err)
	}
	if updated.GueltigkeitMonate != nil || updated.BestaetigungMonate != nil || !updated.PflichtFuerAlle || updated.Name != "Schweißerprüfung MAG (135)" || updated.Hinweis != "geaendert" {
		t.Fatalf("Aenderung nicht wie angegeben: %+v", updated)
	}
	// Der Code bleibt - er ist nicht aenderbar.
	if updated.Code != "SCHWEISSER_135" {
		t.Fatalf("Code hat sich geaendert: %q", updated.Code)
	}

	// Stilllegen: aus der Standardliste verschwunden, mit Schalter sichtbar,
	// weiterhin einzeln abrufbar.
	second, err := svc.Create(ctx, QualificationTypeInput{Code: "ARBEITSSCHUTZ", Name: "Arbeitsschutz", Kategorie: QualificationKategorieUnterweisung, GueltigkeitMonate: intPtr(12)}, company)
	if err != nil {
		t.Fatalf("zweite Art anlegen: %v", err)
	}
	if _, err := svc.Update(ctx, created.ID, QualificationTypeUpdate{Name: updated.Name, Kategorie: updated.Kategorie, Aktiv: false}, company); err != nil {
		t.Fatalf("Stilllegen: %v", err)
	}
	active, err := svc.List(ctx, company, false)
	if err != nil {
		t.Fatalf("Liste: %v", err)
	}
	if len(active) != 1 || active[0].ID != second.ID {
		t.Fatalf("Standardliste: erwartet nur die aktive Art, bekommen %+v", active)
	}
	all, err := svc.List(ctx, company, true)
	if err != nil {
		t.Fatalf("Liste mit inaktiven: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("Liste mit inaktiven: erwartet 2, bekommen %d", len(all))
	}
	// Sortiert nach Name: "Arbeitsschutz" vor "Schweißerprüfung".
	if all[0].ID != second.ID || all[1].ID != created.ID {
		t.Fatalf("Liste nicht nach Name sortiert: %q, %q", all[0].Name, all[1].Name)
	}
	if still, err := svc.Get(ctx, created.ID, company); err != nil || still.Aktiv {
		t.Fatalf("stillgelegte Art muss abrufbar und inaktiv sein: %+v, %v", still, err)
	}
}

func TestQualificationTypeCodeIsUniquePerCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewQualificationTypeService(env.PG)
	company, other := seedCatalogCompany(t, env), seedCatalogCompany(t, env)
	in := QualificationTypeInput{Code: "STAPLER_SCHEIN", Name: "Staplerschein", Kategorie: QualificationKategorieQualifikation}

	if _, err := svc.Create(ctx, in, company); err != nil {
		t.Fatalf("Anlegen: %v", err)
	}
	// Fachliche Meldung statt rohem UNIQUE-Fehler - auch bei anderer
	// Schreibweise, weil der Code normalisiert wird.
	lower := in
	lower.Code = " stapler_schein "
	_, err := svc.Create(ctx, lower, company)
	wantEmployeeError(t, err, "Qualifikationsart mit diesem Code existiert bereits")

	// Ein anderer Mandant fuehrt seinen eigenen Katalog.
	if _, err := svc.Create(ctx, in, other); err != nil {
		t.Fatalf("derselbe Code bei anderem Mandanten muss zulaessig sein, bekommen %v", err)
	}
}

func TestQualificationTypeIsScopedToCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewQualificationTypeService(env.PG)
	company, other := seedCatalogCompany(t, env), seedCatalogCompany(t, env)

	created, err := svc.Create(ctx, QualificationTypeInput{Code: "ERSTHELFER", Name: "Ersthelfer", Kategorie: QualificationKategorieQualifikation, GueltigkeitMonate: intPtr(24)}, company)
	if err != nil {
		t.Fatalf("Anlegen: %v", err)
	}

	_, err = svc.Get(ctx, created.ID, other)
	wantEmployeeError(t, err, "Qualifikationsart nicht gefunden")
	_, err = svc.Get(ctx, uuid.New(), company)
	wantEmployeeError(t, err, "Qualifikationsart nicht gefunden")
	_, err = svc.Update(ctx, created.ID, QualificationTypeUpdate{Name: "Fremd", Kategorie: QualificationKategorieQualifikation, Aktiv: false}, other)
	wantEmployeeError(t, err, "Qualifikationsart nicht gefunden")
	_, err = svc.Update(ctx, uuid.New(), QualificationTypeUpdate{Name: "X", Kategorie: QualificationKategorieQualifikation}, company)
	wantEmployeeError(t, err, "Qualifikationsart nicht gefunden")

	if list, err := svc.List(ctx, other, true); err != nil || len(list) != 0 {
		t.Fatalf("Katalog des anderen Mandanten muss leer sein: %+v, %v", list, err)
	}
	got, err := svc.Get(ctx, created.ID, company)
	if err != nil {
		t.Fatalf("Laden: %v", err)
	}
	if got.Name != "Ersthelfer" || !got.Aktiv {
		t.Fatalf("Aenderung aus fremdem Mandanten wurde geschrieben: %+v", got)
	}
}

// Der Vorschlagskatalog wird uebernommen, nicht untergeschoben: was der
// Mandant schon hat, bleibt, wie es ist.
func TestQualificationTypeAdoptDefaults(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewQualificationTypeService(env.PG)
	company, other := seedCatalogCompany(t, env), seedCatalogCompany(t, env)
	total := len(DefaultQualificationTypes())

	// Der Mandant hat ARBEITSSCHUTZ bereits selbst angelegt - mit eigener
	// Frist und ohne Pflicht fuer alle.
	own, err := svc.Create(ctx, QualificationTypeInput{Code: "ARBEITSSCHUTZ", Name: "Unsere Sicherheitsunterweisung", Kategorie: QualificationKategorieUnterweisung, GueltigkeitMonate: intPtr(6), Hinweis: "halbjaehrlich laut Betriebsvereinbarung"}, company)
	if err != nil {
		t.Fatalf("eigene Art anlegen: %v", err)
	}

	created, err := svc.AdoptDefaults(ctx, company)
	if err != nil {
		t.Fatalf("Uebernahme: %v", err)
	}
	if len(created) != total-1 {
		t.Fatalf("erwartet %d neu angelegte Arten (alle ausser der vorhandenen), bekommen %d", total-1, len(created))
	}
	for _, qt := range created {
		if qt.Code == "ARBEITSSCHUTZ" {
			t.Fatal("die bereits vorhandene Art wurde als neu angelegt gemeldet")
		}
		if !qt.Aktiv {
			t.Fatalf("uebernommene Art ist nicht aktiv: %+v", qt)
		}
	}

	// Die eigene Art ist unveraendert.
	kept, err := svc.Get(ctx, own.ID, company)
	if err != nil {
		t.Fatalf("eigene Art laden: %v", err)
	}
	if kept.Name != "Unsere Sicherheitsunterweisung" || kept.GueltigkeitMonate == nil || *kept.GueltigkeitMonate != 6 || kept.PflichtFuerAlle || kept.Hinweis != "halbjaehrlich laut Betriebsvereinbarung" {
		t.Fatalf("Uebernahme hat die eigene Art ueberschrieben: %+v", kept)
	}

	// Der Katalog ist jetzt vollstaendig, und die Werte stimmen mit der
	// Vorlage ueberein (Stichprobe an einer Art mit und einer ohne Frist).
	all, err := svc.List(ctx, company, true)
	if err != nil {
		t.Fatalf("Liste: %v", err)
	}
	if len(all) != total {
		t.Fatalf("erwartet %d Arten im Katalog, bekommen %d", total, len(all))
	}
	byCode := make(map[string]QualificationType, len(all))
	for _, qt := range all {
		byCode[qt.Code] = qt
	}
	if e := byCode["ERSTHELFER"]; e.GueltigkeitMonate == nil || *e.GueltigkeitMonate != 24 || e.Kategorie != QualificationKategorieQualifikation {
		t.Fatalf("ERSTHELFER nicht wie in der Vorlage: %+v", e)
	}
	if s := byCode["STAPLER_SCHEIN"]; s.GueltigkeitMonate != nil || s.Kategorie != QualificationKategorieQualifikation {
		t.Fatalf("STAPLER_SCHEIN muss unbefristet sein: %+v", s)
	}
	if b := byCode["STAPLER_BEAUFTRAGUNG"]; b.GueltigkeitMonate != nil || b.Kategorie != QualificationKategorieBeauftragung {
		t.Fatalf("STAPLER_BEAUFTRAGUNG nicht wie in der Vorlage: %+v", b)
	}
	// Die Zwischenbestaetigung wird mit uebernommen (F.10.3).
	if w := byCode["SCHWEISSERPRUEFUNG"]; w.BestaetigungMonate == nil || *w.BestaetigungMonate != 6 {
		t.Fatalf("SCHWEISSERPRUEFUNG ohne Bestaetigungsintervall: %+v", w)
	}
	if e := byCode["ERSTHELFER"]; e.BestaetigungMonate != nil {
		t.Fatalf("ERSTHELFER mit Bestaetigungsintervall: %+v", e)
	}

	// Der Mandant aendert eine uebernommene Art und legt eine still; eine
	// zweite Uebernahme legt nichts an und stellt nichts zurueck.
	ersthelfer := byCode["ERSTHELFER"]
	if _, err := svc.Update(ctx, ersthelfer.ID, QualificationTypeUpdate{Name: "Ersthelfer (intern)", Kategorie: ersthelfer.Kategorie, GueltigkeitMonate: intPtr(12), Aktiv: true}, company); err != nil {
		t.Fatalf("uebernommene Art aendern: %v", err)
	}
	kran := byCode["KRANFUEHRER"]
	if _, err := svc.Update(ctx, kran.ID, QualificationTypeUpdate{Name: kran.Name, Kategorie: kran.Kategorie, Aktiv: false}, company); err != nil {
		t.Fatalf("uebernommene Art stilllegen: %v", err)
	}
	again, err := svc.AdoptDefaults(ctx, company)
	if err != nil {
		t.Fatalf("zweite Uebernahme: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("zweite Uebernahme darf nichts anlegen, bekommen %d", len(again))
	}
	if e, _ := svc.Get(ctx, ersthelfer.ID, company); e == nil || e.Name != "Ersthelfer (intern)" || *e.GueltigkeitMonate != 12 {
		t.Fatalf("zweite Uebernahme hat eine geaenderte Art zurueckgestellt: %+v", e)
	}
	if k, _ := svc.Get(ctx, kran.ID, company); k == nil || k.Aktiv {
		t.Fatalf("zweite Uebernahme hat eine stillgelegte Art wieder aktiviert: %+v", k)
	}

	// Der andere Mandant hat davon nichts bekommen.
	if list, err := svc.List(ctx, other, true); err != nil || len(list) != 0 {
		t.Fatalf("Uebernahme hat den Katalog eines anderen Mandanten gefuellt: %d Arten, %v", len(list), err)
	}
}
