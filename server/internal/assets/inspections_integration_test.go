package assets

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Pruefungen gegen echtes Postgres (Backlog F.3.5.1). Feste Daten in der
// Vergangenheit; nur "in der Zukunft" rechnet mit heute.

func TestInspectionRecordFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	company := seedCompany(t, env)
	actor := testutil.SeedAuthUser(t, env, "pruefung-"+uuid.NewString()[:8]+"@example.test", "pw-123456789", "hr")
	pruefer := seedEmployee(t, env, company, "Pruefer", true)

	bohrer, err := svc.Create(ctx, AssetInput{Inventarnummer: "E-1", Bezeichnung: "Bohrhammer", Kategorie: KategorieElektrowerkzeug, PruefintervallMonate: intPtr(6)}, company, "")
	if err != nil {
		t.Fatal(err)
	}
	laptop, err := svc.Create(ctx, AssetInput{Inventarnummer: "IT-1", Bezeichnung: "Laptop", Kategorie: KategorieIT}, company, "")
	if err != nil {
		t.Fatal(err)
	}

	// Ohne Angabe: naechste Pruefung aus dem Intervall, am Monatsende
	// begrenzt.
	first, err := svc.RecordInspection(ctx, InspectionInput{
		AssetID: bohrer.ID, GeprueftAm: time.Date(2025, 8, 31, 22, 0, 0, 0, time.FixedZone("x", 3600)), Ergebnis: " bestanden ",
		PrueferName: " Elektro Meier ", PrueferEmployeeID: &pruefer, Notiz: " Isolationswiderstand ok ",
	}, actor, company)
	if err != nil {
		t.Fatalf("Erfassen: %v", err)
	}
	if !first.GeprueftAm.Equal(d(2025, 8, 31)) || first.Ergebnis != ErgebnisBestanden || first.NaechstePruefungAm == nil ||
		!first.NaechstePruefungAm.Equal(d(2026, 2, 28)) || first.PrueferName != "Elektro Meier" || first.Notiz != "Isolationswiderstand ok" ||
		first.PrueferEmployeeID == nil || *first.PrueferEmployeeID != pruefer || first.Status != InspectionActive ||
		first.CreatedByUserID == nil || *first.CreatedByUserID != actor || first.AssetID != bohrer.ID {
		t.Fatalf("Pruefung nicht wie erwartet: %+v", first)
	}

	// Angegebener Folgetermin hat Vorrang vor dem Intervall.
	explicit, err := svc.RecordInspection(ctx, InspectionInput{AssetID: bohrer.ID, GeprueftAm: d(2026, 2, 20), Ergebnis: ErgebnisMaengel, NaechstePruefungAm: datePtr(2026, 5, 20), PrueferName: "Meier"}, "", company)
	if err != nil {
		t.Fatal(err)
	}
	if !explicit.NaechstePruefungAm.Equal(d(2026, 5, 20)) || explicit.PrueferEmployeeID != nil || explicit.CreatedByUserID != nil {
		t.Fatalf("angegebener Folgetermin erwartet: %+v", explicit)
	}
	// Nicht bestanden: kein Folgetermin, auch mit Intervall.
	failed, err := svc.RecordInspection(ctx, InspectionInput{AssetID: bohrer.ID, GeprueftAm: d(2026, 5, 18), Ergebnis: ErgebnisNichtBestanden, PrueferName: "Meier"}, "", company)
	if err != nil {
		t.Fatal(err)
	}
	if failed.NaechstePruefungAm != nil {
		t.Fatalf("nicht bestanden: kein Folgetermin erwartet: %+v", failed)
	}
	// Ohne Intervall und ohne Angabe: kein Folgetermin.
	lp, err := svc.RecordInspection(ctx, InspectionInput{AssetID: laptop.ID, GeprueftAm: d(2026, 3, 1), Ergebnis: ErgebnisBestanden, PrueferName: "IT"}, "", company)
	if err != nil {
		t.Fatal(err)
	}
	if lp.NaechstePruefungAm != nil {
		t.Fatalf("ohne Intervall kein Folgetermin erwartet: %+v", lp)
	}

	// Ein geaendertes Intervall wirkt nicht auf erfasste Pruefungen.
	if _, err := svc.Update(ctx, bohrer.ID, AssetInput{Inventarnummer: "E-1", Bezeichnung: "Bohrhammer", Kategorie: KategorieElektrowerkzeug, PruefintervallMonate: intPtr(3)}, company); err != nil {
		t.Fatal(err)
	}
	again, err := svc.GetInspection(ctx, first.ID, company)
	if err != nil || !again.NaechstePruefungAm.Equal(d(2026, 2, 28)) {
		t.Fatalf("Folgetermin darf sich nicht aendern: %+v, %v", again, err)
	}
	next, err := svc.RecordInspection(ctx, InspectionInput{AssetID: bohrer.ID, GeprueftAm: d(2026, 6, 1), Ergebnis: ErgebnisBestanden, PrueferName: "Meier"}, "", company)
	if err != nil || !next.NaechstePruefungAm.Equal(d(2026, 9, 1)) {
		t.Fatalf("neue Pruefung mit neuem Intervall erwartet: %+v, %v", next, err)
	}

	// Liste: juengste zuerst; nur die des Assets.
	list, err := svc.ListInspections(ctx, bohrer.ID, false, company)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 4 || list[0].ID != next.ID || list[1].ID != failed.ID || list[2].ID != explicit.ID || list[3].ID != first.ID {
		t.Fatalf("Reihenfolge der Liste: %+v", list)
	}
	// Gleicher Tag: die zuletzt erfasste zuerst.
	sameDay, err := svc.RecordInspection(ctx, InspectionInput{AssetID: bohrer.ID, GeprueftAm: d(2026, 6, 1), Ergebnis: ErgebnisNichtBestanden, PrueferName: "Meier"}, "", company)
	if err != nil {
		t.Fatal(err)
	}
	list, _ = svc.ListInspections(ctx, bohrer.ID, false, company)
	if list[0].ID != sameDay.ID || list[1].ID != next.ID {
		t.Fatalf("gleicher Tag: zuletzt erfasste zuerst erwartet: %v, %v", list[0].ID, list[1].ID)
	}
}

func TestInspectionRejections(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	company := seedCompany(t, env)
	other := seedCompany(t, env)
	fremderPruefer := seedEmployee(t, env, other, "Fremd", true)
	leiter, err := svc.Create(ctx, AssetInput{Inventarnummer: "L-1", Bezeichnung: "Leiter", Kategorie: KategorieLeiter, PruefintervallMonate: intPtr(12), Anschaffungsdatum: datePtr(2026, 1, 10)}, company, "")
	if err != nil {
		t.Fatal(err)
	}
	fremdesAsset := seedAsset(t, svc, other, "L-1")

	ok := InspectionInput{AssetID: leiter.ID, GeprueftAm: d(2026, 2, 1), Ergebnis: ErgebnisBestanden, PrueferName: "Meier"}
	mod := func(f func(*InspectionInput)) InspectionInput { in := ok; f(&in); return in }

	_, err = svc.RecordInspection(ctx, mod(func(in *InspectionInput) { in.AssetID = fremdesAsset.ID }), "", company)
	wantErr(t, "fremdes Asset", err, "Asset nicht gefunden")
	_, err = svc.RecordInspection(ctx, mod(func(in *InspectionInput) { in.AssetID = uuid.Nil }), "", company)
	wantErr(t, "ohne Asset", err, "asset_id erforderlich")
	_, err = svc.RecordInspection(ctx, mod(func(in *InspectionInput) { in.PrueferEmployeeID = &fremderPruefer }), "", company)
	wantErr(t, "fremder Pruefer-Mitarbeiter", err, "Mitarbeiter nicht gefunden")
	_, err = svc.RecordInspection(ctx, mod(func(in *InspectionInput) { in.GeprueftAm = today().AddDate(0, 0, 1) }), "", company)
	wantErr(t, "Zukunft", err, "Zukunft")
	_, err = svc.RecordInspection(ctx, mod(func(in *InspectionInput) { in.GeprueftAm = d(2026, 1, 9) }), "", company)
	wantErr(t, "vor Anschaffung", err, "Anschaffungsdatum")
	_, err = svc.RecordInspection(ctx, mod(func(in *InspectionInput) { in.Ergebnis = "gut" }), "", company)
	wantErr(t, "Ergebnis", err, "Ungültiges Prüfergebnis")
	_, err = svc.RecordInspection(ctx, mod(func(in *InspectionInput) { in.PrueferName = " " }), "", company)
	wantErr(t, "ohne Pruefer", err, "Prüfer erforderlich")
	_, err = svc.RecordInspection(ctx, mod(func(in *InspectionInput) {
		in.Ergebnis = ErgebnisNichtBestanden
		in.NaechstePruefungAm = datePtr(2027, 1, 1)
	}), "", company)
	wantErr(t, "nicht bestanden mit Folgetermin", err, "keine nächste Prüfung")
	_, err = svc.RecordInspection(ctx, ok, "", " ")
	wantErr(t, "ohne Mandant", err, "Mandant erforderlich")

	if _, err := svc.Retire(ctx, leiter.ID, d(2026, 3, 1), "gebrochen", company); err != nil {
		t.Fatal(err)
	}
	_, err = svc.RecordInspection(ctx, ok, "", company)
	wantErr(t, "ausgemustert", err, "ausgemustert")

	var n int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM asset_inspections WHERE asset_id IN ($1,$2)`, leiter.ID, fremdesAsset.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("abgelehnte Pruefungen duerfen nichts schreiben, gefunden %d", n)
	}
}

func TestInspectionStornoAndScoping(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	company := seedCompany(t, env)
	other := seedCompany(t, env)
	actor := testutil.SeedAuthUser(t, env, "pstorno-"+uuid.NewString()[:8]+"@example.test", "pw-123456789", "hr")
	gurt := seedAsset(t, svc, company, "PSA-1")
	zweiter := seedAsset(t, svc, company, "PSA-2")

	wrong, err := svc.RecordInspection(ctx, InspectionInput{AssetID: gurt.ID, GeprueftAm: d(2026, 3, 1), Ergebnis: ErgebnisNichtBestanden, PrueferName: "Meier"}, "", company)
	if err != nil {
		t.Fatal(err)
	}
	keep, err := svc.RecordInspection(ctx, InspectionInput{AssetID: zweiter.ID, GeprueftAm: d(2026, 3, 1), Ergebnis: ErgebnisBestanden, PrueferName: "Meier"}, "", company)
	if err != nil {
		t.Fatal(err)
	}

	// Fremder Mandant: weder lesen noch auflisten noch stornieren.
	if _, err := svc.GetInspection(ctx, wrong.ID, other); err == nil {
		t.Fatal("fremder Mandant darf die Pruefung nicht lesen")
	}
	_, err = svc.ListInspections(ctx, gurt.ID, true, other)
	wantErr(t, "Liste fremder Mandant", err, "Asset nicht gefunden")
	_, err = svc.StornoInspection(ctx, wrong.ID, "Irrtum", actor, other)
	wantErr(t, "Storno fremder Mandant", err, "Prüfung nicht gefunden")

	_, err = svc.StornoInspection(ctx, wrong.ID, " ", actor, company)
	wantErr(t, "ohne Begruendung", err, "Begründung")
	st, err := svc.StornoInspection(ctx, wrong.ID, " falsches Geraet ", actor, company)
	if err != nil {
		t.Fatalf("Storno: %v", err)
	}
	if st.Status != InspectionStorniert || st.StornoReason != "falsches Geraet" || st.StorniertAt == nil ||
		st.StorniertByUserID == nil || *st.StorniertByUserID != actor || st.Ergebnis != ErgebnisNichtBestanden {
		t.Fatalf("Storno nicht wie erwartet: %+v", st)
	}
	_, err = svc.StornoInspection(ctx, wrong.ID, "nochmal", actor, company)
	wantErr(t, "zweites Storno", err, "bereits storniert")
	if again, _ := svc.GetInspection(ctx, wrong.ID, company); again.StornoReason != "falsches Geraet" || !again.StorniertAt.Equal(*st.StorniertAt) {
		t.Fatalf("abgelehntes zweites Storno hat geschrieben: %+v", again)
	}

	// Liste: stornierte nur auf Wunsch.
	if list, _ := svc.ListInspections(ctx, gurt.ID, false, company); len(list) != 0 {
		t.Fatalf("stornierte duerfen ohne Wunsch nicht erscheinen: %+v", list)
	}
	if list, _ := svc.ListInspections(ctx, gurt.ID, true, company); len(list) != 1 || list[0].ID != wrong.ID {
		t.Fatalf("mit Stornierten: %+v", list)
	}
	// Die Pruefung des anderen Assets ist unberuehrt.
	k, _ := svc.GetInspection(ctx, keep.ID, company)
	if k.Status != InspectionActive {
		t.Fatalf("Storno hat eine andere Pruefung getroffen: %+v", k)
	}
	if list, _ := svc.ListInspections(ctx, zweiter.ID, false, company); len(list) != 1 || list[0].ID != keep.ID {
		t.Fatalf("Liste darf nur Pruefungen des Assets enthalten: %+v", list)
	}
}
