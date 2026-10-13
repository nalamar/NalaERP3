package hr

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Anforderungen je Mitarbeiter und der Stand eines Mitarbeiters (Backlog
// F.2.4.3) gegen echtes Postgres. Fixture aus
// qualifications_integration_test.go.

func (f qualFixture) namedType(t *testing.T, code, name string, months *int, pflicht bool) QualificationType {
	t.Helper()
	kategorie := QualificationKategorieQualifikation
	if months != nil {
		kategorie = QualificationKategorieUnterweisung
	}
	qt, err := f.types.Create(context.Background(), QualificationTypeInput{Code: code, Name: name, Kategorie: kategorie, GueltigkeitMonate: months, PflichtFuerAlle: pflicht}, f.company)
	if err != nil {
		t.Fatalf("Art anlegen: %v", err)
	}
	return *qt
}

func (f qualFixture) deactivate(t *testing.T, qt QualificationType) {
	t.Helper()
	if _, err := f.types.Update(context.Background(), qt.ID, QualificationTypeUpdate{Name: qt.Name, Kategorie: qt.Kategorie, GueltigkeitMonate: qt.GueltigkeitMonate, PflichtFuerAlle: qt.PflichtFuerAlle, Aktiv: false}, f.company); err != nil {
		t.Fatalf("Art stilllegen: %v", err)
	}
}

func requirementCodes(reqs []QualificationRequirement) []string {
	out := make([]string, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, r.Code)
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestQualificationSetRequirementsReplacesTheSet(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	f := newQualFixture(t, env)
	emp := f.employee(t, "Anforderungen")
	a := f.namedType(t, "A_STAPLER", "Stapler", nil, false)
	b := f.namedType(t, "B_ERSTHELFER", "Ersthelfer", intPtr(24), false)
	c := f.namedType(t, "C_KRAN", "Kran", nil, false)
	userID := testutil.SeedAuthUser(t, env, "quali-req@example.invalid", "geheim-123456", "hr")

	// Ein neuer Mitarbeiter hat keine Anforderungen.
	if list, err := f.svc.ListRequirements(ctx, emp, f.company); err != nil || len(list) != 0 {
		t.Fatalf("erwartet keine Anforderungen, bekommen %+v, %v", list, err)
	}

	// Ein Kollege mit eigenen Anforderungen: was im Folgenden fuer emp
	// geaendert, ersetzt und geleert wird, darf ihn nicht beruehren.
	colleague := f.employee(t, "AnforderungenKollege")
	if _, err := f.svc.SetRequirements(ctx, colleague, []uuid.UUID{a.ID, b.ID}, "", f.company); err != nil {
		t.Fatalf("Anforderungen des Kollegen setzen: %v", err)
	}
	t.Cleanup(func() {
		list, err := f.svc.ListRequirements(ctx, colleague, f.company)
		if err != nil {
			t.Errorf("Anforderungen des Kollegen lesen: %v", err)
			return
		}
		if want := []string{"B_ERSTHELFER", "A_STAPLER"}; !sameStrings(requirementCodes(list), want) {
			t.Errorf("Aenderungen an einem Mitarbeiter haben die Anforderungen eines anderen veraendert: erwartet %v, bekommen %v", want, requirementCodes(list))
		}
	})

	// Setzen; ein doppelt genannter Eintrag zaehlt einmal. Sortiert nach Name.
	got, err := f.svc.SetRequirements(ctx, emp, []uuid.UUID{a.ID, b.ID, a.ID}, userID, f.company)
	if err != nil {
		t.Fatalf("Setzen: %v", err)
	}
	if want := []string{"B_ERSTHELFER", "A_STAPLER"}; !sameStrings(requirementCodes(got), want) {
		t.Fatalf("erwartet %v (nach Name: Ersthelfer, Stapler), bekommen %v", want, requirementCodes(got))
	}
	if got[0].Name != "Ersthelfer" || got[0].Kategorie != QualificationKategorieUnterweisung || !got[0].TypeAktiv {
		t.Fatalf("Angaben zur Art fehlen: %+v", got[0])
	}
	var createdBy *string
	var createdAtB time.Time
	if err := env.PG.QueryRow(ctx, `SELECT created_by_user_id, created_at FROM hr_employee_qualification_requirements WHERE employee_id=$1 AND type_id=$2`, emp, b.ID).Scan(&createdBy, &createdAtB); err != nil {
		t.Fatalf("Anforderung lesen: %v", err)
	}
	if createdBy == nil || *createdBy != userID {
		t.Fatalf("erwartet created_by_user_id=%s, bekommen %v", userID, createdBy)
	}

	// Ersetzen: A faellt weg, C kommt dazu, B bleibt - und zwar dieselbe
	// Zeile, nicht geloescht und neu angelegt.
	got, err = f.svc.SetRequirements(ctx, emp, []uuid.UUID{c.ID, b.ID}, "", f.company)
	if err != nil {
		t.Fatalf("Ersetzen: %v", err)
	}
	if want := []string{"B_ERSTHELFER", "C_KRAN"}; !sameStrings(requirementCodes(got), want) {
		t.Fatalf("erwartet %v, bekommen %v", want, requirementCodes(got))
	}
	var createdAtBAfter time.Time
	var createdByAfter *string
	if err := env.PG.QueryRow(ctx, `SELECT created_at, created_by_user_id FROM hr_employee_qualification_requirements WHERE employee_id=$1 AND type_id=$2`, emp, b.ID).Scan(&createdAtBAfter, &createdByAfter); err != nil {
		t.Fatalf("Anforderung lesen: %v", err)
	}
	if !createdAtBAfter.Equal(createdAtB) || createdByAfter == nil || *createdByAfter != userID {
		t.Fatalf("eine unveraendert gebliebene Anforderung wurde neu angelegt (created_at %v -> %v)", createdAtB, createdAtBAfter)
	}

	// Leere Liste entfernt alle.
	got, err = f.svc.SetRequirements(ctx, emp, nil, "", f.company)
	if err != nil {
		t.Fatalf("Leeren: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("erwartet keine Anforderungen, bekommen %v", requirementCodes(got))
	}
}

func TestQualificationSetRequirementsValidatesAndIsAtomic(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	f := newQualFixture(t, env)
	other := newQualFixture(t, env)
	emp := f.employee(t, "AnforderungenAtomar")
	a := f.namedType(t, "A_STAPLER", "Stapler", nil, false)
	b := f.namedType(t, "B_ERSTHELFER", "Ersthelfer", intPtr(24), false)
	old := f.namedType(t, "ALT", "Alte Art", nil, false)
	foreign := other.namedType(t, "FREMD", "Fremde Art", nil, false)

	if _, err := f.svc.SetRequirements(ctx, emp, []uuid.UUID{a.ID, old.ID}, "", f.company); err != nil {
		t.Fatalf("Ausgangslage: %v", err)
	}
	assertUnchanged := func(t *testing.T) {
		t.Helper()
		list, err := f.svc.ListRequirements(ctx, emp, f.company)
		if err != nil {
			t.Fatalf("Liste: %v", err)
		}
		if want := []string{"ALT", "A_STAPLER"}; !sameStrings(requirementCodes(list), want) {
			t.Fatalf("abgelehnte Aenderung hat die Anforderungen veraendert: erwartet %v, bekommen %v", want, requirementCodes(list))
		}
	}

	// Jede dieser Listen wuerde A entfernen und B hinzufuegen - und
	// scheitert an einem Eintrag. Nichts davon darf wirksam werden.
	_, err := f.svc.SetRequirements(ctx, emp, []uuid.UUID{b.ID, uuid.New()}, "", f.company)
	wantEmployeeError(t, err, "Qualifikationsart nicht gefunden")
	assertUnchanged(t)
	_, err = f.svc.SetRequirements(ctx, emp, []uuid.UUID{b.ID, foreign.ID}, "", f.company)
	wantEmployeeError(t, err, "Qualifikationsart nicht gefunden")
	assertUnchanged(t)

	// Stillgelegte Art: als NEUE Anforderung abgelehnt ...
	stale := f.namedType(t, "STILL", "Stillgelegte Art", nil, false)
	f.deactivate(t, stale)
	_, err = f.svc.SetRequirements(ctx, emp, []uuid.UUID{a.ID, old.ID, stale.ID}, "", f.company)
	wantEmployeeError(t, err, "Qualifikationsart ist stillgelegt")
	assertUnchanged(t)

	// ... aber eine BESTEHENDE Anforderung auf eine inzwischen stillgelegte
	// Art darf stehen bleiben, und die Liste bleibt speicherbar.
	f.deactivate(t, old)
	got, err := f.svc.SetRequirements(ctx, emp, []uuid.UUID{a.ID, old.ID, b.ID}, "", f.company)
	if err != nil {
		t.Fatalf("Liste mit bestehender Anforderung auf stillgelegte Art muss speicherbar sein, bekommen %v", err)
	}
	if want := []string{"ALT", "B_ERSTHELFER", "A_STAPLER"}; !sameStrings(requirementCodes(got), want) {
		t.Fatalf("erwartet %v, bekommen %v", want, requirementCodes(got))
	}
	for _, r := range got {
		if r.Code == "ALT" && r.TypeAktiv {
			t.Fatal("stillgelegte Art muss in der Liste als nicht aktiv erscheinen")
		}
	}

	// Mandant und Mitarbeiter.
	_, err = f.svc.SetRequirements(ctx, emp, []uuid.UUID{a.ID}, "", other.company)
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
	_, err = f.svc.SetRequirements(ctx, uuid.New(), []uuid.UUID{a.ID}, "", f.company)
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
	_, err = f.svc.ListRequirements(ctx, emp, other.company)
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
	if list, _ := f.svc.ListRequirements(ctx, emp, f.company); len(list) != 3 {
		t.Fatalf("Zugriff aus fremdem Mandanten hat die Anforderungen veraendert: %v", requirementCodes(list))
	}
}

// Der Stand eines Mitarbeiters fuehrt Nachweise, eigene Anforderungen und
// "Pflicht fuer alle" zusammen.
func TestQualificationStanding(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	f := newQualFixture(t, env)
	other := newQualFixture(t, env)
	emp := f.employee(t, "Stand")
	today := qDay(t, "2026-10-13")

	pflicht := f.namedType(t, "ARBEITSSCHUTZ", "Arbeitsschutz", intPtr(12), true)
	stapler := f.namedType(t, "STAPLER_SCHEIN", "Stapler", nil, false)
	ersthelfer := f.namedType(t, "ERSTHELFER", "Ersthelfer", intPtr(24), false)
	kran := f.namedType(t, "KRANFUEHRER", "Kran", nil, false)
	brandschutz := f.namedType(t, "BRANDSCHUTZHELFER", "Brandschutzhelfer", intPtr(36), false)
	alt := f.namedType(t, "ALT", "Alte Art", intPtr(12), false)

	// Anforderungen der Stelle: Stapler (vorhanden), Ersthelfer
	// (abgelaufen), Kran (fehlt), dazu eine spaeter stillgelegte Art.
	if _, err := f.svc.SetRequirements(ctx, emp, []uuid.UUID{stapler.ID, ersthelfer.ID, kran.ID, alt.ID}, "", f.company); err != nil {
		t.Fatalf("Anforderungen setzen: %v", err)
	}
	f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{emp}, TypeID: stapler.ID, ErworbenAm: qDay(t, "2019-04-01")})
	f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{emp}, TypeID: ersthelfer.ID, ErworbenAm: qDay(t, "2024-06-01")}) // bis 2026-06-01
	// Nicht gefordert, aber vorhanden und in 30 Tagen faellig.
	f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{emp}, TypeID: brandschutz.ID, ErworbenAm: qDay(t, "2023-11-12")}) // bis 2026-11-12
	// Ein stornierter Arbeitsschutz-Nachweis zaehlt nicht.
	wrong := f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{emp}, TypeID: pflicht.ID, ErworbenAm: qDay(t, "2026-09-01")})
	if _, err := f.svc.Storno(ctx, wrong[0].ID, "Falscher Mitarbeiter", "", f.company); err != nil {
		t.Fatalf("Storno: %v", err)
	}
	// Die alte Art: abgelaufener Nachweis und Anforderung - dann stillgelegt.
	f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{emp}, TypeID: alt.ID, ErworbenAm: qDay(t, "2022-01-01")})
	f.deactivate(t, alt)

	got, err := f.svc.Standing(ctx, emp, today, DefaultQualificationWarnDays, f.company)
	if err != nil {
		t.Fatalf("Stand: %v", err)
	}
	type row struct {
		code, status string
		erforderlich bool
	}
	want := []row{
		// fehlt - nach Name: Arbeitsschutz (Pflicht fuer alle), Kran (Anforderung)
		{"ARBEITSSCHUTZ", QualificationStandFehlt, true},
		{"KRANFUEHRER", QualificationStandFehlt, true},
		// abgelaufen
		{"ERSTHELFER", QualificationStandAbgelaufen, true},
		// laeuft ab
		{"BRANDSCHUTZHELFER", QualificationStandLaeuftAb, false},
		// gueltig
		{"STAPLER_SCHEIN", QualificationStandGueltig, true},
	}
	if len(got) != len(want) {
		t.Fatalf("erwartet %d Eintraege, bekommen %d: %+v", len(want), len(got), got)
	}
	for i, w := range want {
		g := got[i]
		if g.Code != w.code || g.Status != w.status || g.Erforderlich != w.erforderlich {
			t.Fatalf("Position %d: erwartet %+v, bekommen code=%s status=%s erforderlich=%v", i, w, g.Code, g.Status, g.Erforderlich)
		}
		if g.Name == "" || g.Kategorie == "" || g.TypeID == uuid.Nil {
			t.Fatalf("Position %d: Angaben zur Art fehlen: %+v", i, g)
		}
	}
	// Die stillgelegte Art erscheint nicht - weder als abgelaufen noch als fehlend.
	for _, g := range got {
		if g.Code == "ALT" {
			t.Fatalf("stillgelegte Art darf im Stand nicht erscheinen: %+v", g)
		}
	}
	// Tage bis Ablauf: Brandschutz in 30 Tagen, Ersthelfer seit 134 Tagen ueberfaellig.
	if d := got[3].TageBisAblauf; d == nil || *d != 30 {
		t.Fatalf("Brandschutzhelfer: erwartet 30 Tage, bekommen %v", d)
	}
	if d := got[2].TageBisAblauf; d == nil || *d != -134 {
		t.Fatalf("Ersthelfer: erwartet -134 Tage, bekommen %v", d)
	}
	// Die Nachweise der stillgelegten Art bleiben in der Historie.
	if hist, err := f.svc.ListForEmployee(ctx, emp, false, f.company); err != nil || len(hist) != 4 {
		t.Fatalf("Historie: erwartet 4 aktive Nachweise (inkl. der stillgelegten Art), bekommen %d, %v", len(hist), err)
	}

	// Der Stichtag wird uebergeben, nicht intern ermittelt. Ein Stichtag,
	// der NICHT das Datum ist, an dem der Test laeuft: drei Monate spaeter
	// ist der Brandschutzhelfer abgelaufen (bis 12.11.2026).
	t.Run("Stichtag ist ein Parameter", func(t *testing.T) {
		got, err := f.svc.Standing(ctx, emp, qDay(t, "2027-01-15"), DefaultQualificationWarnDays, f.company)
		if err != nil {
			t.Fatalf("Stand: %v", err)
		}
		status := map[string]string{}
		days := map[string]*int{}
		for _, g := range got {
			status[g.Code], days[g.Code] = g.Status, g.TageBisAblauf
		}
		if status["BRANDSCHUTZHELFER"] != QualificationStandAbgelaufen || days["BRANDSCHUTZHELFER"] == nil || *days["BRANDSCHUTZHELFER"] != -64 {
			t.Fatalf("am 15.01.2027: erwartet Brandschutzhelfer abgelaufen seit 64 Tagen, bekommen %s/%v", status["BRANDSCHUTZHELFER"], days["BRANDSCHUTZHELFER"])
		}
		if status["STAPLER_SCHEIN"] != QualificationStandGueltig {
			t.Fatalf("unbefristeter Stapler-Schein muss gueltig bleiben, bekommen %s", status["STAPLER_SCHEIN"])
		}
	})

	t.Run("Vorwarnzeit ist ein Parameter", func(t *testing.T) {
		got, err := f.svc.Standing(ctx, emp, today, 10, f.company)
		if err != nil {
			t.Fatalf("Stand: %v", err)
		}
		for _, g := range got {
			if g.Code == "BRANDSCHUTZHELFER" && g.Status != QualificationStandGueltig {
				t.Fatalf("bei 10 Tagen Vorwarnzeit ist ein Ablauf in 30 Tagen noch gueltig, bekommen %s", g.Status)
			}
		}
	})

	t.Run("Pflicht fuer alle gilt nur fuer aktive Mitarbeiter", func(t *testing.T) {
		no := false
		gone, err := f.emps.Create(ctx, EmployeeInput{FirstName: "Quali", LastName: "Ausgeschieden", Active: &no}, f.company)
		if err != nil {
			t.Fatalf("Mitarbeiter anlegen: %v", err)
		}
		if _, err := f.svc.SetRequirements(ctx, gone.ID, []uuid.UUID{kran.ID}, "", f.company); err != nil {
			t.Fatalf("Anforderung setzen: %v", err)
		}
		got, err := f.svc.Standing(ctx, gone.ID, today, DefaultQualificationWarnDays, f.company)
		if err != nil {
			t.Fatalf("Stand: %v", err)
		}
		// Die eigene Anforderung bleibt sichtbar, die Pflicht fuer alle nicht.
		if len(got) != 1 || got[0].Code != "KRANFUEHRER" || got[0].Status != QualificationStandFehlt {
			t.Fatalf("erwartet nur die eigene Anforderung, bekommen %+v", got)
		}
	})

	t.Run("neuer Mitarbeiter ohne alles: nur die Pflicht fuer alle fehlt", func(t *testing.T) {
		fresh := f.employee(t, "StandNeu")
		got, err := f.svc.Standing(ctx, fresh, today, DefaultQualificationWarnDays, f.company)
		if err != nil {
			t.Fatalf("Stand: %v", err)
		}
		if len(got) != 1 || got[0].Code != "ARBEITSSCHUTZ" || got[0].Status != QualificationStandFehlt {
			t.Fatalf("erwartet genau ARBEITSSCHUTZ fehlt, bekommen %+v", got)
		}
	})

	t.Run("Mandantentrennung", func(t *testing.T) {
		_, err := f.svc.Standing(ctx, emp, today, DefaultQualificationWarnDays, other.company)
		wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
		_, err = f.svc.Standing(ctx, uuid.New(), today, DefaultQualificationWarnDays, f.company)
		wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
		// Die Pflicht-Art des einen Mandanten gilt nicht fuer Mitarbeiter des anderen.
		foreignEmp := other.employee(t, "StandFremd")
		got, err := other.svc.Standing(ctx, foreignEmp, today, DefaultQualificationWarnDays, other.company)
		if err != nil {
			t.Fatalf("Stand: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("Mitarbeiter eines Mandanten ohne Katalog darf keinen Stand haben, bekommen %+v", got)
		}
	})
}
