package assets

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Pruefstand, Faelligkeitsliste und Pruefung bei der Ausgabe gegen echtes
// Postgres (Backlog F.3.5.2). Der Stichtag ist ueberall ein Parameter
// bzw. das Ausgabedatum - kein Ergebnis haengt vom echten Tagesdatum ab.

func recordInsp(t *testing.T, svc *Service, company string, asset uuid.UUID, in InspectionInput) *Inspection {
	t.Helper()
	in.AssetID = asset
	if in.PrueferName == "" {
		in.PrueferName = "Meier"
	}
	i, err := svc.RecordInspection(context.Background(), in, "", company)
	if err != nil {
		t.Fatalf("Pruefung erfassen: %v", err)
	}
	return i
}

func createAsset(t *testing.T, svc *Service, company, nr string, intervall *int) *Asset {
	t.Helper()
	a, err := svc.Create(context.Background(), AssetInput{Inventarnummer: nr, Bezeichnung: "Stueck " + nr, Kategorie: KategorieElektrowerkzeug, Standort: "Lager " + nr, PruefintervallMonate: intervall}, company, "")
	if err != nil {
		t.Fatalf("Asset anlegen: %v", err)
	}
	return a
}

// dueFixture legt einen Betrieb mit je einem Asset in jedem Stand am
// 15.06.2026 (Vorwarnzeit 30) an.
type dueFixture struct {
	company, other                                                               string
	gesperrt, ueber2, ueber10, nie, bald, ok, frei, weg, stornoGesperrt, maengel *Asset
	holder                                                                       uuid.UUID
}

func newDueFixture(t *testing.T, env *testutil.IntegrationEnv, svc *Service) dueFixture {
	t.Helper()
	ctx := context.Background()
	f := dueFixture{company: seedCompany(t, env), other: seedCompany(t, env)}
	c := f.company

	// Inventarnummer bewusst hinten im Alphabet: dass "gesperrt" vorn
	// steht, muss aus dem Rang kommen, nicht aus der Nummer.
	f.gesperrt = createAsset(t, svc, c, "Z-GES", intPtr(12))
	recordInsp(t, svc, c, f.gesperrt.ID, InspectionInput{GeprueftAm: d(2026, 1, 10), Ergebnis: ErgebnisBestanden})
	recordInsp(t, svc, c, f.gesperrt.ID, InspectionInput{GeprueftAm: d(2026, 5, 1), Ergebnis: ErgebnisNichtBestanden})

	f.ueber2 = createAsset(t, svc, c, "B-UE2", intPtr(6))
	recordInsp(t, svc, c, f.ueber2.ID, InspectionInput{GeprueftAm: d(2025, 12, 13), Ergebnis: ErgebnisBestanden}) // faellig 13.06.

	f.ueber10 = createAsset(t, svc, c, "C-UE10", intPtr(6))
	recordInsp(t, svc, c, f.ueber10.ID, InspectionInput{GeprueftAm: d(2025, 12, 5), Ergebnis: ErgebnisBestanden}) // faellig 05.06.

	// Maengel sperren (F.3.7) - auch mit einem Folgetermin in der Zukunft.
	f.maengel = createAsset(t, svc, c, "M-MAE", intPtr(12))
	recordInsp(t, svc, c, f.maengel.ID, InspectionInput{GeprueftAm: d(2026, 5, 20), Ergebnis: ErgebnisMaengel})

	f.nie = createAsset(t, svc, c, "D-NIE", intPtr(12))
	// Zweites "nie geprueft" ohne Tage: Gleichstand, entschieden nach
	// Inventarnummer.
	createAsset(t, svc, c, "D0-NIE", intPtr(12))

	f.bald = createAsset(t, svc, c, "E-BALD", intPtr(6))
	recordInsp(t, svc, c, f.bald.ID, InspectionInput{GeprueftAm: d(2026, 1, 10), Ergebnis: ErgebnisBestanden}) // faellig 10.07.

	f.ok = createAsset(t, svc, c, "F-OK", intPtr(12))
	recordInsp(t, svc, c, f.ok.ID, InspectionInput{GeprueftAm: d(2026, 3, 1), Ergebnis: ErgebnisBestanden})

	f.frei = createAsset(t, svc, c, "G-FREI", nil)

	f.weg = createAsset(t, svc, c, "H-WEG", intPtr(6))
	recordInsp(t, svc, c, f.weg.ID, InspectionInput{GeprueftAm: d(2025, 1, 1), Ergebnis: ErgebnisBestanden})
	if _, err := svc.Retire(ctx, f.weg.ID, d(2026, 2, 1), "verschrottet", c); err != nil {
		t.Fatal(err)
	}

	// Durchgefallene Pruefung storniert: es gilt die fruehere bestandene.
	f.stornoGesperrt = createAsset(t, svc, c, "I-STO", intPtr(12))
	recordInsp(t, svc, c, f.stornoGesperrt.ID, InspectionInput{GeprueftAm: d(2025, 6, 20), Ergebnis: ErgebnisBestanden}) // faellig 20.06.2026
	wrong := recordInsp(t, svc, c, f.stornoGesperrt.ID, InspectionInput{GeprueftAm: d(2026, 5, 2), Ergebnis: ErgebnisNichtBestanden})
	if _, err := svc.StornoInspection(ctx, wrong.ID, "falsches Geraet", "", c); err != nil {
		t.Fatal(err)
	}

	// Wer hat das ueberfaellige Stueck? Ausgegeben vor der Faelligkeit.
	f.holder = seedEmployee(t, env, c, "Halter", true)
	if _, err := svc.Issue(ctx, IssueInput{AssetID: f.ueber10.ID, EmployeeID: f.holder, AusgegebenAm: d(2026, 1, 3)}, "", c); err != nil {
		t.Fatal(err)
	}

	// B-UE2 war zweimal ausgegeben und ist zurueck: kein Halter, und die
	// zurueckgegebenen Ausgaben duerfen keine weiteren Zeilen erzeugen.
	for _, r := range [][2]int{{2, 5}, {7, 9}} {
		x, err := svc.Issue(ctx, IssueInput{AssetID: f.ueber2.ID, EmployeeID: f.holder, AusgegebenAm: d(2026, 1, r[0])}, "", c)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Return(ctx, x.ID, d(2026, 1, r[1]), "", "", c); err != nil {
			t.Fatal(err)
		}
	}

	// Anderer Mandant: ein ueberfaelliges Stueck, das nie erscheinen darf.
	x := createAsset(t, svc, f.other, "A-GES", intPtr(6))
	recordInsp(t, svc, f.other, x.ID, InspectionInput{GeprueftAm: d(2025, 1, 1), Ergebnis: ErgebnisNichtBestanden})
	return f
}

func TestInspectionDueList(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	f := newDueFixture(t, env, svc)

	list, err := svc.DueList(ctx, d(2026, 6, 15), 30, f.company)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(list))
	for _, e := range list {
		got = append(got, e.Inventarnummer+":"+e.Status)
	}
	want := "M-MAE:gesperrt,Z-GES:gesperrt,C-UE10:ueberfaellig,B-UE2:ueberfaellig,D-NIE:nie_geprueft,D0-NIE:nie_geprueft,I-STO:faellig_bald,E-BALD:faellig_bald"
	if strings.Join(got, ",") != want {
		t.Fatalf("Liste:\n erwartet %s\n bekommen %s", want, strings.Join(got, ","))
	}
	byNr := map[string]AssetDueEntry{}
	for _, e := range list {
		byNr[e.Inventarnummer] = e
	}
	c := byNr["C-UE10"]
	if c.TageBisFaellig == nil || *c.TageBisFaellig != -10 || c.Maengel || c.EmployeeID == nil || *c.EmployeeID != f.holder ||
		c.MitarbeiterName != "Max Halter" || c.Bezeichnung != "Stueck C-UE10" || c.Standort != "Lager C-UE10" || c.Kategorie != KategorieElektrowerkzeug {
		t.Fatalf("Eintrag C-UE10 nicht wie erwartet: %+v", c)
	}
	if b := byNr["B-UE2"]; *b.TageBisFaellig != -2 || b.EmployeeID != nil || b.MitarbeiterName != "" || b.Maengel {
		t.Fatalf("Eintrag B-UE2 nicht wie erwartet: %+v", b)
	}
	if m := byNr["M-MAE"]; !m.Maengel || m.TageBisFaellig != nil || m.Ergebnis != ErgebnisMaengel {
		t.Fatalf("Eintrag M-MAE: gesperrt wegen Maengeln erwartet: %+v", m)
	}
	if z := byNr["Z-GES"]; z.Maengel {
		t.Fatalf("Eintrag Z-GES: gesperrt wegen 'nicht bestanden', nicht wegen Maengeln: %+v", z)
	}
	if i := byNr["I-STO"]; *i.TageBisFaellig != 5 || !i.GeprueftAm.Equal(d(2025, 6, 20)) {
		t.Fatalf("stornierte Pruefung darf nicht zaehlen: %+v", i)
	}
	if e := byNr["E-BALD"]; *e.TageBisFaellig != 25 {
		t.Fatalf("Eintrag E-BALD: %+v", e)
	}

	// Vorwarnzeit als Parameter: mit 0 faellt "bald faellig" weg.
	short, err := svc.DueList(ctx, d(2026, 6, 15), 0, f.company)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range short {
		if e.Status == PruefstandFaelligBald {
			t.Fatalf("Vorwarnzeit 0: kein 'bald faellig' erwartet: %+v", e)
		}
	}
	if len(short) != 6 {
		t.Fatalf("Vorwarnzeit 0: erwartet 6 Eintraege, bekommen %d", len(short))
	}

	// Anderer Mandant sieht nur sein Stueck.
	o, err := svc.DueList(ctx, d(2026, 6, 15), 30, f.other)
	if err != nil || len(o) != 1 || o[0].Status != PruefstandGesperrt {
		t.Fatalf("anderer Mandant: %+v, %v", o, err)
	}
	if _, err := svc.DueList(ctx, d(2026, 6, 15), 30, " "); err == nil {
		t.Fatal("leerer Mandant muss abgelehnt werden")
	}
}

// Waechter: die betriebsweite Liste und der Stand eines Assets duerfen
// nicht auseinanderlaufen. Fuer jedes nicht ausgemusterte Asset gilt:
// steht es in der Liste, dann mit genau dem Stand aus InspectionStandingOf;
// fehlt es, dann ist dieser Stand einer, der nicht in die Liste gehoert.
func TestInspectionDueListMatchesStanding(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	f := newDueFixture(t, env, svc)

	for _, day := range []int{1, 15, 30} {
		today := d(2026, 6, day)
		list, err := svc.DueList(ctx, today, 30, f.company)
		if err != nil {
			t.Fatal(err)
		}
		inList := map[uuid.UUID]AssetDueEntry{}
		for _, e := range list {
			inList[e.AssetID] = e
		}
		all, err := svc.List(ctx, f.company, ListFilter{})
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range all {
			st, err := svc.InspectionStandingOf(ctx, a.ID, today, 30, f.company)
			if err != nil {
				t.Fatal(err)
			}
			e, ok := inList[a.ID]
			if ok != inspectionNeedsAttention(st.Status) {
				t.Fatalf("%s am %d.06.: in Liste=%v, Stand %s", a.Inventarnummer, day, ok, st.Status)
			}
			if ok && (e.Status != st.Status || (e.TageBisFaellig == nil) != (st.TageBisFaellig == nil) ||
				(e.TageBisFaellig != nil && *e.TageBisFaellig != *st.TageBisFaellig) || e.Maengel != st.Maengel ||
				(e.InspectionID == nil) != (st.InspectionID == nil) || (e.InspectionID != nil && *e.InspectionID != *st.InspectionID)) {
				t.Fatalf("%s am %d.06.: Liste %+v, Einzelstand %+v", a.Inventarnummer, day, e.InspectionStanding, *st)
			}
		}
	}
}

func TestInspectionStandingOf(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	f := newDueFixture(t, env, svc)

	st, err := svc.InspectionStandingOf(ctx, f.gesperrt.ID, d(2026, 6, 15), 30, f.company)
	if err != nil || st.Status != PruefstandGesperrt || st.Ergebnis != ErgebnisNichtBestanden {
		t.Fatalf("gesperrt erwartet: %+v, %v", st, err)
	}
	// Vor der durchgefallenen Pruefung war es geprueft.
	st, _ = svc.InspectionStandingOf(ctx, f.gesperrt.ID, d(2026, 4, 30), 30, f.company)
	if st.Status != PruefstandGeprueft {
		t.Fatalf("vor dem 01.05. geprueft erwartet: %+v", st)
	}
	st, _ = svc.InspectionStandingOf(ctx, f.frei.ID, d(2026, 6, 15), 30, f.company)
	if st.Status != PruefstandNichtPruefpflichtig {
		t.Fatalf("ohne Intervall: %+v", st)
	}
	st, _ = svc.InspectionStandingOf(ctx, f.nie.ID, d(2026, 6, 15), 30, f.company)
	if st.Status != PruefstandNieGeprueft {
		t.Fatalf("nie geprueft: %+v", st)
	}
	// Warnzeit wird durchgereicht.
	st, _ = svc.InspectionStandingOf(ctx, f.bald.ID, d(2026, 6, 15), 10, f.company)
	if st.Status != PruefstandGeprueft || *st.TageBisFaellig != 25 {
		t.Fatalf("Vorwarnzeit 10: %+v", st)
	}
	if _, err := svc.InspectionStandingOf(ctx, f.gesperrt.ID, d(2026, 6, 15), 30, f.other); err == nil || err.Error() != "Asset nicht gefunden" {
		t.Fatalf("fremder Mandant: %v", err)
	}
}

func TestAssignmentIssueRespectsInspection(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	c := seedCompany(t, env)
	anna := seedEmployee(t, env, c, "Anna", true)

	gurt := createAsset(t, svc, c, "PSA-1", intPtr(12))
	recordInsp(t, svc, c, gurt.ID, InspectionInput{GeprueftAm: d(2026, 1, 10), Ergebnis: ErgebnisBestanden})
	failed := recordInsp(t, svc, c, gurt.ID, InspectionInput{GeprueftAm: d(2026, 3, 1), Ergebnis: ErgebnisNichtBestanden})

	// Gesperrt: abgelehnt, nichts geschrieben.
	_, err := svc.Issue(ctx, IssueInput{AssetID: gurt.ID, EmployeeID: anna, AusgegebenAm: d(2026, 3, 1)}, "", c)
	wantErr(t, "gesperrt", err, "gesperrt")
	if list, _ := svc.ListAssignments(ctx, c, AssignmentFilter{AssetID: &gurt.ID, IncludeStorniert: true}); len(list) != 0 {
		t.Fatalf("abgelehnte Ausgabe hat geschrieben: %+v", list)
	}
	// Nachgetragen VOR der durchgefallenen Pruefung: erlaubt, ohne Hinweis.
	early, err := svc.Issue(ctx, IssueInput{AssetID: gurt.ID, EmployeeID: anna, AusgegebenAm: d(2026, 2, 1)}, "", c)
	if err != nil {
		t.Fatalf("Ausgabe vor der durchgefallenen Pruefung: %v", err)
	}
	if len(early.Hinweise) != 0 {
		t.Fatalf("keine Hinweise erwartet: %q", early.Hinweise)
	}
	if _, err := svc.Return(ctx, early.ID, d(2026, 2, 20), "", "", c); err != nil {
		t.Fatal(err)
	}
	// Storno der durchgefallenen Pruefung hebt die Sperre auf.
	if _, err := svc.StornoInspection(ctx, failed.ID, "falsches Geraet", "", c); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Issue(ctx, IssueInput{AssetID: gurt.ID, EmployeeID: anna, AusgegebenAm: d(2026, 3, 2)}, "", c); err != nil {
		t.Fatalf("nach Storno der Sperre: %v", err)
	}

	// Maengel sperren (F.3.7, Entscheidung des Nutzers vom 2026-10-19) -
	// mit eigener Meldung; eine bestandene Wiederholung hebt die Sperre auf.
	defekt := createAsset(t, svc, c, "M-DEF", intPtr(12))
	recordInsp(t, svc, c, defekt.ID, InspectionInput{GeprueftAm: d(2026, 1, 10), Ergebnis: ErgebnisMaengel})
	_, err = svc.Issue(ctx, IssueInput{AssetID: defekt.ID, EmployeeID: anna, AusgegebenAm: d(2026, 1, 12)}, "", c)
	wantErr(t, "Maengel", err, "Asset ist gesperrt: letzte Prüfung mit Mängeln")
	recordInsp(t, svc, c, defekt.ID, InspectionInput{GeprueftAm: d(2026, 1, 20), Ergebnis: ErgebnisBestanden})
	if _, err := svc.Issue(ctx, IssueInput{AssetID: defekt.ID, EmployeeID: anna, AusgegebenAm: d(2026, 1, 21)}, "", c); err != nil {
		t.Fatalf("nach bestandener Wiederholung: %v", err)
	}

	// Hinweise: ueberfaellig (mit Datum); nie geprueft.
	alt := createAsset(t, svc, c, "E-ALT", intPtr(6))
	recordInsp(t, svc, c, alt.ID, InspectionInput{GeprueftAm: d(2025, 7, 1), Ergebnis: ErgebnisBestanden}) // faellig 01.01.2026
	x, err := svc.Issue(ctx, IssueInput{AssetID: alt.ID, EmployeeID: anna, AusgegebenAm: d(2026, 1, 5)}, "", c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(x.Hinweise, "|") != "Prüfung überfällig seit 01.01.2026" {
		t.Fatalf("Hinweis ueberfaellig: %q", x.Hinweise)
	}
	// Die Hinweise gehoeren nur zur Antwort der Ausgabe.
	if again, _ := svc.GetAssignment(ctx, x.ID, c); len(again.Hinweise) != 0 {
		t.Fatalf("Hinweise duerfen nicht gespeichert erscheinen: %q", again.Hinweise)
	}
	nie := createAsset(t, svc, c, "N-NIE", intPtr(12))
	y, err := svc.Issue(ctx, IssueInput{AssetID: nie.ID, EmployeeID: anna, AusgegebenAm: d(2026, 1, 5)}, "", c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(y.Hinweise, "|") != "Asset wurde noch nie geprüft" {
		t.Fatalf("Hinweis nie geprueft: %q", y.Hinweise)
	}
	// Ohne Pruefpflicht: kein Hinweis.
	z, err := svc.Issue(ctx, IssueInput{AssetID: createAsset(t, svc, c, "IT-1", nil).ID, EmployeeID: anna, AusgegebenAm: d(2026, 1, 5)}, "", c)
	if err != nil || len(z.Hinweise) != 0 {
		t.Fatalf("ohne Pruefpflicht: %+v, %v", z, err)
	}
}
