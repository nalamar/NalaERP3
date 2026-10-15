package assets

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/hr"
	"nalaerp3/internal/testutil"
)

// Asset-Stamm gegen echtes Postgres (Backlog F.3.3).
//
// Jeder Test arbeitet in einem eigenen, frisch angelegten Mandanten: die
// Inventarnummer ist je Mandant eindeutig, und Tests, die Listen zaehlen,
// duerfen einander nicht sehen.

func seedCompany(t *testing.T, env *testutil.IntegrationEnv) string {
	t.Helper()
	id := "as-" + uuid.NewString()[:12]
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO company_profiles (id, name) VALUES ($1, $2)`, id, "Assettest "+id); err != nil {
		t.Fatalf("Mandant anlegen: %v", err)
	}
	return id
}

func seedCostCenter(t *testing.T, env *testutil.IntegrationEnv, companyID string) string {
	t.Helper()
	id := "cc-" + uuid.NewString()[:12]
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO cost_centers (id, company_id, code, name) VALUES ($1, $2, $3, 'Werkstatt')`, id, companyID, id); err != nil {
		t.Fatalf("Kostenstelle anlegen: %v", err)
	}
	return id
}

func today() time.Time { return hr.WorkDateOf(time.Now()) }

func TestAssetCreateGetUpdateFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	company := seedCompany(t, env)
	cc := seedCostCenter(t, env, company)
	actor := testutil.SeedAuthUser(t, env, "asset-"+uuid.NewString()[:8]+"@example.test", "pw-123456789", "hr")

	created, err := svc.Create(ctx, AssetInput{
		Inventarnummer: "  WZ-0017 ", Bezeichnung: " Bohrhammer ", Kategorie: KategorieElektrowerkzeug,
		Seriennummer: "SN-1", Hersteller: "Hilti", Modell: "TE 30", Standort: "Werkstatt",
		Anschaffungsdatum: datePtr(2025, 3, 1), Anschaffungspreis: floatPtr(499.9),
		KostenstelleID: strPtr(cc), PruefintervallMonate: intPtr(6), Notiz: "Koffer rot",
	}, company, actor)
	if err != nil {
		t.Fatalf("Anlegen: %v", err)
	}
	if created.Inventarnummer != "WZ-0017" || created.Bezeichnung != "Bohrhammer" {
		t.Fatalf("nicht normalisiert: %+v", created)
	}
	if created.AusgemustertAm != nil || created.AusgemustertGrund != "" {
		t.Fatalf("neues Asset darf nicht ausgemustert sein: %+v", created)
	}

	got, err := svc.Get(ctx, created.ID, company)
	if err != nil {
		t.Fatalf("Laden: %v", err)
	}
	if got.Kategorie != KategorieElektrowerkzeug || got.Seriennummer != "SN-1" || got.Hersteller != "Hilti" || got.Modell != "TE 30" ||
		got.Standort != "Werkstatt" || got.Notiz != "Koffer rot" ||
		got.Anschaffungsdatum == nil || !got.Anschaffungsdatum.Equal(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)) ||
		got.Anschaffungspreis == nil || *got.Anschaffungspreis != 499.9 ||
		got.KostenstelleID == nil || *got.KostenstelleID != cc ||
		got.PruefintervallMonate == nil || *got.PruefintervallMonate != 6 {
		t.Fatalf("gespeichertes Asset nicht wie angelegt: %+v", got)
	}
	var createdBy *string
	if err := env.PG.QueryRow(ctx, `SELECT created_by_user_id FROM assets WHERE id=$1`, created.ID).Scan(&createdBy); err != nil {
		t.Fatal(err)
	}
	if createdBy == nil || *createdBy != actor {
		t.Fatalf("Urheber nicht gespeichert: %v", createdBy)
	}

	// Aendern ersetzt ALLE Stammdaten - auch optionale auf "keine".
	updated, err := svc.Update(ctx, created.ID, AssetInput{
		Inventarnummer: "WZ-0017a", Bezeichnung: "Bohrhammer alt", Kategorie: KategorieWerkzeug,
	}, company)
	if err != nil {
		t.Fatalf("Aendern: %v", err)
	}
	if updated.Inventarnummer != "WZ-0017a" || updated.Bezeichnung != "Bohrhammer alt" || updated.Kategorie != KategorieWerkzeug ||
		updated.Seriennummer != "" || updated.Hersteller != "" || updated.Modell != "" || updated.Standort != "" || updated.Notiz != "" ||
		updated.Anschaffungsdatum != nil || updated.Anschaffungspreis != nil || updated.KostenstelleID != nil || updated.PruefintervallMonate != nil {
		t.Fatalf("Aendern hat nicht alles ersetzt: %+v", updated)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("created_at darf sich nicht aendern")
	}
	reread, err := svc.Get(ctx, created.ID, company)
	if err != nil || reread.Inventarnummer != "WZ-0017a" || reread.KostenstelleID != nil {
		t.Fatalf("Aenderung nicht gespeichert: %+v, %v", reread, err)
	}

	// Ein anderes Asset desselben Mandanten bleibt unberuehrt.
	other, err := svc.Create(ctx, AssetInput{Inventarnummer: "IT-1", Bezeichnung: "Laptop", Kategorie: KategorieIT}, company, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(ctx, created.ID, AssetInput{Inventarnummer: "WZ-1", Bezeichnung: "X", Kategorie: KategorieWerkzeug}, company); err != nil {
		t.Fatal(err)
	}
	o, _ := svc.Get(ctx, other.ID, company)
	if o.Inventarnummer != "IT-1" || o.Bezeichnung != "Laptop" {
		t.Fatalf("Aendern hat ein anderes Asset getroffen: %+v", o)
	}
}

func TestAssetInventarnummerUniquePerCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	a := seedCompany(t, env)
	b := seedCompany(t, env)

	first, err := svc.Create(ctx, AssetInput{Inventarnummer: "L-1", Bezeichnung: "Leiter", Kategorie: KategorieLeiter}, a, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Create(ctx, AssetInput{Inventarnummer: " L-1 ", Bezeichnung: "Leiter 2", Kategorie: KategorieLeiter}, a, "")
	if err == nil || err.Error() != "Asset mit dieser Inventarnummer existiert bereits" {
		t.Fatalf("doppelte Inventarnummer im selben Mandanten: erwartet fachliche Meldung, bekommen %v", err)
	}
	// Anderer Mandant: dieselbe Nummer ist frei.
	if _, err := svc.Create(ctx, AssetInput{Inventarnummer: "L-1", Bezeichnung: "Leiter", Kategorie: KategorieLeiter}, b, ""); err != nil {
		t.Fatalf("dieselbe Nummer im anderen Mandanten: %v", err)
	}
	// Aendern auf eine vergebene Nummer: dieselbe fachliche Meldung.
	second, err := svc.Create(ctx, AssetInput{Inventarnummer: "L-2", Bezeichnung: "Leiter 2", Kategorie: KategorieLeiter}, a, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Update(ctx, second.ID, AssetInput{Inventarnummer: "L-1", Bezeichnung: "Leiter 2", Kategorie: KategorieLeiter}, a)
	if err == nil || err.Error() != "Asset mit dieser Inventarnummer existiert bereits" {
		t.Fatalf("Aendern auf vergebene Nummer: erwartet fachliche Meldung, bekommen %v", err)
	}
	// Die eigene Nummer behalten ist kein Konflikt.
	if _, err := svc.Update(ctx, first.ID, AssetInput{Inventarnummer: "L-1", Bezeichnung: "Leiter neu", Kategorie: KategorieLeiter}, a); err != nil {
		t.Fatalf("eigene Nummer behalten: %v", err)
	}

	// Zwei gleichzeitige Anlagen derselben Nummer: genau eine gelingt.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.Create(ctx, AssetInput{Inventarnummer: "PAR-1", Bezeichnung: "X", Kategorie: KategorieSonstiges}, a, "")
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, e := range errs {
		if e == nil {
			ok++
		} else if e.Error() != "Asset mit dieser Inventarnummer existiert bereits" {
			t.Fatalf("gleichzeitige Anlage: roher Fehler statt fachlicher Meldung: %v", e)
		}
	}
	if ok != 1 {
		t.Fatalf("gleichzeitige Anlage: erwartet genau eine Anlage, bekommen %d", ok)
	}
}

func TestAssetCompanyScoping(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	a := seedCompany(t, env)
	b := seedCompany(t, env)
	ccB := seedCostCenter(t, env, b)

	asset, err := svc.Create(ctx, AssetInput{Inventarnummer: "M-1", Bezeichnung: "Messschieber", Kategorie: KategorieMessmittel}, a, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, asset.ID, b); err == nil || err.Error() != "Asset nicht gefunden" {
		t.Fatalf("fremder Mandant darf nicht lesen: %v", err)
	}
	if _, err := svc.Update(ctx, asset.ID, AssetInput{Inventarnummer: "M-1", Bezeichnung: "gekapert", Kategorie: KategorieMessmittel}, b); err == nil || err.Error() != "Asset nicht gefunden" {
		t.Fatalf("fremder Mandant darf nicht aendern: %v", err)
	}
	if _, err := svc.Retire(ctx, asset.ID, today(), "weg", b); err == nil || err.Error() != "Asset nicht gefunden" {
		t.Fatalf("fremder Mandant darf nicht ausmustern: %v", err)
	}
	if list, err := svc.List(ctx, b, ListFilter{IncludeAusgemustert: true}); err != nil || len(list) != 0 {
		t.Fatalf("Liste des fremden Mandanten muss leer sein: %v, %v", list, err)
	}
	// Kostenstelle eines anderen Mandanten: abgelehnt, beim Anlegen und Aendern.
	if _, err := svc.Create(ctx, AssetInput{Inventarnummer: "M-2", Bezeichnung: "X", Kategorie: KategorieMessmittel, KostenstelleID: strPtr(ccB)}, a, ""); err == nil || err.Error() != "Kostenstelle nicht gefunden" {
		t.Fatalf("fremde Kostenstelle beim Anlegen: %v", err)
	}
	if _, err := svc.Update(ctx, asset.ID, AssetInput{Inventarnummer: "M-1", Bezeichnung: "X", Kategorie: KategorieMessmittel, KostenstelleID: strPtr(ccB)}, a); err == nil || err.Error() != "Kostenstelle nicht gefunden" {
		t.Fatalf("fremde Kostenstelle beim Aendern: %v", err)
	}
	got, _ := svc.Get(ctx, asset.ID, a)
	if got.Bezeichnung != "Messschieber" {
		t.Fatalf("abgelehnte Aenderungen duerfen nichts schreiben: %+v", got)
	}
	// Leerer Mandant.
	if _, err := svc.Create(ctx, AssetInput{Inventarnummer: "M-3", Bezeichnung: "X", Kategorie: KategorieMessmittel}, " ", ""); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("leerer Mandant: %v", err)
	}
}

func TestAssetList(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	company := seedCompany(t, env)

	for _, in := range []AssetInput{
		{Inventarnummer: "C-3", Bezeichnung: "Leiter", Kategorie: KategorieLeiter},
		{Inventarnummer: "A-1", Bezeichnung: "Flex", Kategorie: KategorieElektrowerkzeug},
		{Inventarnummer: "B-2", Bezeichnung: "Leiter alt", Kategorie: KategorieLeiter},
	} {
		if _, err := svc.Create(ctx, in, company, ""); err != nil {
			t.Fatal(err)
		}
	}
	all, err := svc.List(ctx, company, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if nums := inventarnummern(all); nums != "A-1,B-2,C-3" {
		t.Fatalf("erwartet nach Inventarnummer sortiert, bekommen %s", nums)
	}
	leitern, err := svc.List(ctx, company, ListFilter{Kategorie: KategorieLeiter})
	if err != nil {
		t.Fatal(err)
	}
	if nums := inventarnummern(leitern); nums != "B-2,C-3" {
		t.Fatalf("Filter Kategorie: %s", nums)
	}
	if _, err := svc.List(ctx, company, ListFilter{Kategorie: "fahrzeug"}); err == nil || err.Error() != "Ungültige Kategorie" {
		t.Fatalf("unbekannte Kategorie im Filter: %v", err)
	}

	// Ausgemusterte nur auf Wunsch.
	b2 := findByNummer(t, all, "B-2")
	if _, err := svc.Retire(ctx, b2.ID, today(), "Holm gebrochen", company); err != nil {
		t.Fatal(err)
	}
	active, _ := svc.List(ctx, company, ListFilter{})
	if nums := inventarnummern(active); nums != "A-1,C-3" {
		t.Fatalf("ohne Ausgemusterte: %s", nums)
	}
	withRetired, _ := svc.List(ctx, company, ListFilter{IncludeAusgemustert: true, Kategorie: KategorieLeiter})
	if nums := inventarnummern(withRetired); nums != "B-2,C-3" {
		t.Fatalf("mit Ausgemusterten, gefiltert: %s", nums)
	}
}

func TestAssetRetire(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	company := seedCompany(t, env)

	asset, err := svc.Create(ctx, AssetInput{
		Inventarnummer: "PSA-1", Bezeichnung: "Auffanggurt", Kategorie: KategoriePSA,
		Anschaffungsdatum: datePtr(2024, 5, 10), PruefintervallMonate: intPtr(12),
	}, company, "")
	if err != nil {
		t.Fatal(err)
	}
	other, err := svc.Create(ctx, AssetInput{Inventarnummer: "PSA-2", Bezeichnung: "Auffanggurt", Kategorie: KategoriePSA}, company, "")
	if err != nil {
		t.Fatal(err)
	}

	// Abgelehnt: ohne Grund, vor Anschaffung, in der Zukunft.
	if _, err := svc.Retire(ctx, asset.ID, today(), "  ", company); err == nil || !strings.Contains(err.Error(), "Grund") {
		t.Fatalf("ohne Grund: %v", err)
	}
	if _, err := svc.Retire(ctx, asset.ID, time.Date(2024, 5, 9, 0, 0, 0, 0, time.UTC), "Sturz", company); err == nil || !strings.Contains(err.Error(), "Anschaffungsdatum") {
		t.Fatalf("vor Anschaffung: %v", err)
	}
	if _, err := svc.Retire(ctx, asset.ID, today().AddDate(0, 0, 1), "Sturz", company); err == nil || !strings.Contains(err.Error(), "Zukunft") {
		t.Fatalf("in der Zukunft: %v", err)
	}
	if _, err := svc.Retire(ctx, uuid.New(), today(), "Sturz", company); err == nil || err.Error() != "Asset nicht gefunden" {
		t.Fatalf("unbekanntes Asset: %v", err)
	}

	// Noch ausgegeben: abgelehnt. Die Ausgabe wird hier direkt angelegt -
	// der Ausgabe-Service kommt erst mit F.3.4.
	emp := uuid.New()
	if _, err := env.PG.Exec(ctx, `INSERT INTO hr_employees (id, first_name, last_name, company_id) VALUES ($1, 'Gurt', 'Träger', $2)`, emp, company); err != nil {
		t.Fatal(err)
	}
	assignment := uuid.New()
	if _, err := env.PG.Exec(ctx, `INSERT INTO asset_assignments (id, asset_id, employee_id, ausgegeben_am) VALUES ($1, $2, $3, '2026-01-05')`, assignment, asset.ID, emp); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Retire(ctx, asset.ID, today(), "Sturz", company); err == nil || !strings.Contains(err.Error(), "noch ausgegeben") {
		t.Fatalf("offene Ausgabe: %v", err)
	}
	// Storniert bzw. zurueckgegeben zaehlt nicht als offen.
	if _, err := env.PG.Exec(ctx, `UPDATE asset_assignments SET status='storniert', storniert_at=now(), storno_reason='Irrtum' WHERE id=$1`, assignment); err != nil {
		t.Fatal(err)
	}
	second := uuid.New()
	if _, err := env.PG.Exec(ctx, `INSERT INTO asset_assignments (id, asset_id, employee_id, ausgegeben_am, zurueckgegeben_am) VALUES ($1, $2, $3, '2026-02-01', '2026-02-03')`, second, asset.ID, emp); err != nil {
		t.Fatal(err)
	}
	// Eine offene Ausgabe eines ANDEREN Assets haelt dieses nicht auf.
	if _, err := env.PG.Exec(ctx, `INSERT INTO asset_assignments (id, asset_id, employee_id, ausgegeben_am) VALUES ($1, $2, $3, '2026-02-01')`, uuid.New(), other.ID, emp); err != nil {
		t.Fatal(err)
	}

	// Kurz nach Mitternacht in einer Zone oestlich von UTC: gemeint ist
	// der 11.05. (in UTC waere es noch der 10.).
	retired, err := svc.Retire(ctx, asset.ID, time.Date(2024, 5, 11, 0, 30, 0, 0, time.FixedZone("CEST", 2*3600)), "  nach Sturz ausgesondert ", company)
	if err != nil {
		t.Fatalf("Ausmustern: %v", err)
	}
	if retired.AusgemustertAm == nil || !retired.AusgemustertAm.Equal(time.Date(2024, 5, 11, 0, 0, 0, 0, time.UTC)) || retired.AusgemustertGrund != "nach Sturz ausgesondert" {
		t.Fatalf("Ausmusterung nicht wie erwartet: %+v", retired)
	}
	// Endgueltig: zweites Ausmustern abgelehnt, Datum und Grund bleiben.
	if _, err := svc.Retire(ctx, asset.ID, today(), "nochmal", company); err == nil || err.Error() != "Asset ist bereits ausgemustert" {
		t.Fatalf("zweites Ausmustern: %v", err)
	}
	// Stammdaten berichtigen laesst die Ausmusterung unberuehrt.
	upd, err := svc.Update(ctx, asset.ID, AssetInput{Inventarnummer: "PSA-1", Bezeichnung: "Auffanggurt (Seriennummer berichtigt)", Kategorie: KategoriePSA, Seriennummer: "X9"}, company)
	if err != nil {
		t.Fatal(err)
	}
	if upd.AusgemustertAm == nil || upd.AusgemustertGrund != "nach Sturz ausgesondert" {
		t.Fatalf("Aendern hat die Ausmusterung beruehrt: %+v", upd)
	}
	// Das andere Asset ist nicht ausgemustert.
	o, _ := svc.Get(ctx, other.ID, company)
	if o.AusgemustertAm != nil {
		t.Fatalf("Ausmustern hat ein anderes Asset getroffen: %+v", o)
	}
}

func inventarnummern(list []Asset) string {
	out := make([]string, 0, len(list))
	for _, a := range list {
		out = append(out, a.Inventarnummer)
	}
	return strings.Join(out, ",")
}

func findByNummer(t *testing.T, list []Asset, nr string) Asset {
	t.Helper()
	for _, a := range list {
		if a.Inventarnummer == nr {
			return a
		}
	}
	t.Fatalf("Inventarnummer %s nicht gefunden", nr)
	return Asset{}
}
