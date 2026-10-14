package hr

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Betriebsweite Liste der offenen Punkte (Backlog F.2.6.1) gegen echtes
// Postgres. Fixture aus qualifications_integration_test.go und
// qualification_requirements_integration_test.go.

type dueFixture struct {
	f                                             qualFixture
	anna, bernd, carla, gone                      uuid.UUID
	pflicht, stapler, ersthelfer, brandschutz, al QualificationType
	kran                                          QualificationType
}

// newDueFixture baut einen Betrieb mit vier Mitarbeitern:
//
//   - Anna:  Pflicht ok; Ersthelfer abgelaufen (Anforderung); Stapler fehlt (Anforderung)
//   - Bernd: Pflicht fehlt (nur ein STORNIERTER Nachweis); Kran abgelaufen,
//     und zwar laenger als Annas Ersthelfer (Anforderung); Brandschutz
//     laeuft in 30 Tagen ab (keine Anforderung)
//   - Carla: alles gueltig - erscheint nicht
//   - Gone:  ausgeschieden, alles abgelaufen - erscheint nicht
//
// Stichtag ist der 13.10.2026.
func newDueFixture(t *testing.T, env *testutil.IntegrationEnv) dueFixture {
	t.Helper()
	ctx := context.Background()
	d := dueFixture{f: newQualFixture(t, env)}
	f := d.f
	d.pflicht = f.namedType(t, "ARBEITSSCHUTZ", "Arbeitsschutz", intPtr(12), true)
	d.stapler = f.namedType(t, "STAPLER_SCHEIN", "Stapler", nil, false)
	d.ersthelfer = f.namedType(t, "ERSTHELFER", "Ersthelfer", intPtr(24), false)
	d.brandschutz = f.namedType(t, "BRANDSCHUTZHELFER", "Brandschutzhelfer", intPtr(36), false)
	d.al = f.namedType(t, "ALT", "Alte Art", intPtr(12), false)
	d.kran = f.namedType(t, "KRANFUEHRER", "Kran", intPtr(12), false)

	mk := func(first, last string, active bool) uuid.UUID {
		e, err := f.emps.Create(ctx, EmployeeInput{FirstName: first, LastName: last, Active: &active}, f.company)
		if err != nil {
			t.Fatalf("Mitarbeiter anlegen: %v", err)
		}
		return e.ID
	}
	d.anna, d.bernd, d.carla, d.gone = mk("Anna", "Adler", true), mk("Bernd", "Berg", true), mk("Carla", "Clausen", true), mk("Gerd", "Gegangen", false)

	set := func(emp uuid.UUID, types ...uuid.UUID) {
		if _, err := f.svc.SetRequirements(ctx, emp, types, "", f.company); err != nil {
			t.Fatalf("Anforderungen setzen: %v", err)
		}
	}
	rec := func(emp uuid.UUID, typ uuid.UUID, erworben string) {
		f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{emp}, TypeID: typ, ErworbenAm: qDay(t, erworben)})
	}

	set(d.anna, d.stapler.ID, d.ersthelfer.ID)
	rec(d.anna, d.pflicht.ID, "2026-06-01")      // bis 2027-06-01
	rec(d.anna, d.ersthelfer.ID, "2024-01-01")   // bis 2026-01-01, abgelaufen
	rec(d.bernd, d.brandschutz.ID, "2023-11-12") // bis 2026-11-12, 30 Tage
	rec(d.bernd, d.kran.ID, "2024-06-01")        // bis 2025-06-01, 499 Tage ueberfaellig
	// Ein STORNIERTER Arbeitsschutz-Nachweis darf die Pflicht nicht erfuellen.
	wrong := f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{d.bernd}, TypeID: d.pflicht.ID, ErworbenAm: qDay(t, "2026-09-01")})
	if _, err := f.svc.Storno(ctx, wrong[0].ID, "Falscher Mitarbeiter", "", f.company); err != nil {
		t.Fatalf("Storno: %v", err)
	}

	set(d.carla, d.stapler.ID)
	rec(d.carla, d.pflicht.ID, "2026-09-01")
	rec(d.carla, d.stapler.ID, "2020-01-01")

	set(d.gone, d.stapler.ID)
	rec(d.gone, d.ersthelfer.ID, "2020-01-01")

	// Alte Art: Anforderung und abgelaufener Nachweis bei Bernd, dann stillgelegt.
	set(d.bernd, d.al.ID, d.kran.ID)
	rec(d.bernd, d.al.ID, "2022-01-01")
	f.deactivate(t, d.al)
	return d
}

func TestQualificationDueList(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	d := newDueFixture(t, env)
	today := qDay(t, "2026-10-13")

	got, err := d.f.svc.DueList(ctx, today, DefaultQualificationWarnDays, d.f.company)
	if err != nil {
		t.Fatalf("Liste: %v", err)
	}

	type row struct {
		emp    uuid.UUID
		code   string
		status string
		days   *int
	}
	want := []row{
		// fehlt - nach Nachname: Adler vor Berg
		{d.anna, "STAPLER_SCHEIN", QualificationStandFehlt, nil},
		{d.bernd, "ARBEITSSCHUTZ", QualificationStandFehlt, nil},
		// abgelaufen - am laengsten ueberfaellig zuerst, NICHT nach Name:
		// Berg vor Adler
		{d.bernd, "KRANFUEHRER", QualificationStandAbgelaufen, intPtr(-499)},
		{d.anna, "ERSTHELFER", QualificationStandAbgelaufen, intPtr(-285)},
		// laeuft ab
		{d.bernd, "BRANDSCHUTZHELFER", QualificationStandLaeuftAb, intPtr(30)},
	}
	if len(got) != len(want) {
		t.Fatalf("erwartet %d offene Punkte, bekommen %d: %+v", len(want), len(got), got)
	}
	for i, w := range want {
		g := got[i]
		if g.EmployeeID != w.emp || g.Code != w.code || g.Status != w.status {
			t.Fatalf("Position %d: erwartet %s/%s, bekommen %s %s/%s", i, w.code, w.status, g.LastName, g.Code, g.Status)
		}
		switch {
		case w.days == nil && g.TageBisAblauf != nil:
			t.Fatalf("Position %d: erwartet keine Tagesangabe, bekommen %d", i, *g.TageBisAblauf)
		case w.days != nil && (g.TageBisAblauf == nil || *g.TageBisAblauf != *w.days):
			t.Fatalf("Position %d: erwartet %d Tage, bekommen %v", i, *w.days, g.TageBisAblauf)
		}
		if g.FirstName == "" || g.LastName == "" || g.Name == "" || !g.Erforderlich && g.Status == QualificationStandFehlt {
			t.Fatalf("Position %d: Angaben fehlen: %+v", i, g)
		}
	}
	for _, g := range got {
		if g.EmployeeID == d.carla || g.EmployeeID == d.gone {
			t.Fatalf("erwartet weder Carla (alles gueltig) noch Ausgeschiedene: %+v", g)
		}
		if g.Code == "ALT" {
			t.Fatalf("stillgelegte Art darf nicht erscheinen: %+v", g)
		}
		if g.Status == QualificationStandGueltig {
			t.Fatalf("gueltige Eintraege gehoeren nicht in die Liste: %+v", g)
		}
	}

	t.Run("Vorwarnzeit ist ein Parameter", func(t *testing.T) {
		got, err := d.f.svc.DueList(ctx, today, 10, d.f.company)
		if err != nil {
			t.Fatalf("Liste: %v", err)
		}
		for _, g := range got {
			if g.Code == "BRANDSCHUTZHELFER" {
				t.Fatalf("bei 10 Tagen Vorwarnzeit ist ein Ablauf in 30 Tagen nicht faellig: %+v", g)
			}
		}
		if len(got) != 4 {
			t.Fatalf("erwartet 4 offene Punkte, bekommen %d", len(got))
		}
	})

	// Ein Stichtag, der sicher nicht heute ist.
	t.Run("Stichtag ist ein Parameter", func(t *testing.T) {
		got, err := d.f.svc.DueList(ctx, qDay(t, "2027-05-15"), DefaultQualificationWarnDays, d.f.company)
		if err != nil {
			t.Fatalf("Liste: %v", err)
		}
		status := map[string]string{}
		for _, g := range got {
			status[g.LastName+"/"+g.Code] = g.Status
		}
		// Brandschutz ist dann abgelaufen, Annas Arbeitsschutz (bis
		// 01.06.2027) laeuft ab.
		if status["Berg/BRANDSCHUTZHELFER"] != QualificationStandAbgelaufen || status["Adler/ARBEITSSCHUTZ"] != QualificationStandLaeuftAb {
			t.Fatalf("am 15.05.2027 unerwartet: %v", status)
		}
	})

	t.Run("anderer Mandant", func(t *testing.T) {
		other := newQualFixture(t, env)
		got, err := other.svc.DueList(ctx, today, DefaultQualificationWarnDays, other.company)
		if err != nil {
			t.Fatalf("Liste: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("Liste eines anderen Mandanten muss leer sein, bekommen %+v", got)
		}
	})
}

// Waechter: die betriebsweite Liste und der Stand eines Mitarbeiters
// muessen fuer jeden aktiven Mitarbeiter dieselben offenen Punkte zeigen.
// Laufen die beiden auseinander, mahnt die Personalabteilung etwas an,
// das der Mitarbeiter bei sich nicht sieht - oder umgekehrt.
func TestQualificationDueListMatchesStanding(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	d := newDueFixture(t, env)

	for _, stichtag := range []string{"2026-10-13", "2027-05-15", "2020-06-01"} {
		today := qDay(t, stichtag)
		list, err := d.f.svc.DueList(ctx, today, DefaultQualificationWarnDays, d.f.company)
		if err != nil {
			t.Fatalf("Liste: %v", err)
		}
		fromList := map[string]string{}
		for _, g := range list {
			fromList[g.EmployeeID.String()+"/"+g.Code] = g.Status
		}
		fromStanding := map[string]string{}
		for _, emp := range []uuid.UUID{d.anna, d.bernd, d.carla} {
			st, err := d.f.svc.Standing(ctx, emp, today, DefaultQualificationWarnDays, d.f.company)
			if err != nil {
				t.Fatalf("Stand: %v", err)
			}
			for _, s := range st {
				if s.Status != QualificationStandGueltig {
					fromStanding[emp.String()+"/"+s.Code] = s.Status
				}
			}
		}
		if len(fromList) != len(fromStanding) {
			t.Fatalf("Stichtag %s: Liste hat %d offene Punkte, der Stand der Mitarbeiter %d\nListe: %v\nStand: %v", stichtag, len(fromList), len(fromStanding), fromList, fromStanding)
		}
		for k, v := range fromStanding {
			if fromList[k] != v {
				t.Fatalf("Stichtag %s, %s: Stand %s, Liste %q", stichtag, k, v, fromList[k])
			}
		}
	}
}
