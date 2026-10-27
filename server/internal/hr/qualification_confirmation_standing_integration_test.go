package hr

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Zwischenbestaetigung im Stand und in der Faelligkeitsliste (Backlog
// F.10.3.2a) gegen echtes Postgres. Bestaetigungen werden hier direkt
// eingetragen; Erfassen und Stornieren ueber den Service folgt in F.10.3.2b.

func insertConfirmation(t *testing.T, env *testutil.IntegrationEnv, qualificationID uuid.UUID, am string, storniert bool) {
	t.Helper()
	status, reason := "active", ""
	var at *time.Time
	if storniert {
		now := time.Now()
		status, reason, at = "storniert", "Falsches Datum", &now
	}
	if _, err := env.PG.Exec(context.Background(), `
		INSERT INTO hr_qualification_confirmations (id, qualification_id, bestaetigt_am, bestaetigt_durch, status, storno_reason, storniert_at)
		VALUES ($1,$2,$3,'Schweissaufsicht',$4,$5,$6)`, uuid.New(), qualificationID, qDay(t, am), status, reason, at); err != nil {
		t.Fatalf("Bestaetigung eintragen: %v", err)
	}
}

func TestQualificationConfirmationsInStandingAndDueList(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	f := newQualFixture(t, env)

	weld, err := f.types.Create(ctx, QualificationTypeInput{Code: "SCHWEISSER", Name: "Schweißerprüfung", Kategorie: QualificationKategorieQualifikation,
		GueltigkeitMonate: intPtr(36), BestaetigungMonate: intPtr(6)}, f.company)
	if err != nil {
		t.Fatalf("Art anlegen: %v", err)
	}
	anna := f.employee(t, "Adler")
	if _, err := f.svc.SetRequirements(ctx, anna, []uuid.UUID{weld.ID}, "", f.company); err != nil {
		t.Fatalf("Anforderung: %v", err)
	}
	rec := f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{anna}, TypeID: weld.ID, ErworbenAm: qDay(t, "2026-01-15")})[0]
	insertConfirmation(t, env, rec.ID, "2026-07-10", false)
	// Storniert: wuerde die Frist bis 2027-07-05 verlaengern.
	insertConfirmation(t, env, rec.ID, "2027-01-05", true)

	standingOf := func(today string) EmployeeQualificationStanding {
		t.Helper()
		got, err := f.svc.Standing(ctx, anna, qDay(t, today), 60, f.company)
		if err != nil || len(got) != 1 {
			t.Fatalf("Stand: %v %+v", err, got)
		}
		return got[0]
	}
	dueOf := func(today string) []QualificationDueEntry {
		t.Helper()
		got, err := f.svc.DueList(ctx, qDay(t, today), 60, f.company)
		if err != nil {
			t.Fatalf("Faelligkeitsliste: %v", err)
		}
		return got
	}

	// Bestaetigung ueberfaellig: abgelaufen, obwohl das Zertifikat bis 2029 gilt.
	st := standingOf("2027-02-01")
	if st.Status != QualificationStandAbgelaufen || st.AblaufGrund != QualificationAblaufBestaetigung || *st.TageBisAblauf != -22 {
		t.Fatalf("Stand: %+v", st)
	}
	wantDate(t, "bestaetigung_faellig_am", st.BestaetigungFaelligAm, "2027-01-10")
	wantDate(t, "gueltig_bis", st.GueltigBis, "2029-01-15")
	due := dueOf("2027-02-01")
	if len(due) != 1 || due[0].EmployeeID != anna || due[0].Status != QualificationStandAbgelaufen || due[0].AblaufGrund != QualificationAblaufBestaetigung {
		t.Fatalf("Faelligkeitsliste: %+v", due)
	}

	// Innerhalb der Vorwarnzeit: laeuft ab.
	if st := standingOf("2026-12-01"); st.Status != QualificationStandLaeuftAb || *st.TageBisAblauf != 40 {
		t.Fatalf("Stand vor Faelligkeit: %+v", st)
	}
	if due := dueOf("2026-12-01"); len(due) != 1 || due[0].Status != QualificationStandLaeuftAb {
		t.Fatalf("Faelligkeitsliste vor Faelligkeit: %+v", due)
	}
	// Davor: gueltig, nichts in der Liste.
	if st := standingOf("2026-09-01"); st.Status != QualificationStandGueltig {
		t.Fatalf("Stand: %+v", st)
	}
	if due := dueOf("2026-09-01"); len(due) != 0 {
		t.Fatalf("Faelligkeitsliste: %+v", due)
	}

	// Eine weitere, rechtzeitige Bestaetigung verlaengert - in beiden Sichten.
	insertConfirmation(t, env, rec.ID, "2027-01-08", false)
	if st := standingOf("2027-02-01"); st.Status != QualificationStandGueltig {
		t.Fatalf("Stand nach Bestaetigung: %+v", st)
	} else {
		wantDate(t, "bestaetigung_faellig_am", st.BestaetigungFaelligAm, "2027-07-08")
	}
	if due := dueOf("2027-02-01"); len(due) != 0 {
		t.Fatalf("Faelligkeitsliste nach Bestaetigung: %+v", due)
	}

	// Ohne Intervall an der Art (geaendert) wirkt keine Bestaetigung mehr:
	// das Intervall gilt so, wie es heute an der Art steht.
	if _, err := f.types.Update(ctx, weld.ID, QualificationTypeUpdate{Name: weld.Name, Kategorie: weld.Kategorie, GueltigkeitMonate: weld.GueltigkeitMonate, Aktiv: true}, f.company); err != nil {
		t.Fatalf("Art aendern: %v", err)
	}
	if st := standingOf("2028-01-01"); st.Status != QualificationStandGueltig || st.BestaetigungFaelligAm != nil || st.AblaufGrund != "" {
		t.Fatalf("Stand ohne Intervall: %+v", st)
	}
}

// Die Bestaetigungen eines Mitarbeiters gelten nur fuer seine Nachweise,
// auch wenn ein Kollege dieselbe Art hat.
func TestQualificationConfirmationsBelongToTheirQualification(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	f := newQualFixture(t, env)
	weld, err := f.types.Create(ctx, QualificationTypeInput{Code: "SCHWEISSER", Name: "Schweißerprüfung", Kategorie: QualificationKategorieQualifikation,
		GueltigkeitMonate: intPtr(36), BestaetigungMonate: intPtr(6)}, f.company)
	if err != nil {
		t.Fatalf("Art anlegen: %v", err)
	}
	anna, bernd := f.employee(t, "Adler"), f.employee(t, "Berg")
	recs := f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{anna, bernd}, TypeID: weld.ID, ErworbenAm: qDay(t, "2026-01-15")})
	var annaRec uuid.UUID
	for _, r := range recs {
		if r.EmployeeID == anna {
			annaRec = r.ID
		}
	}
	insertConfirmation(t, env, annaRec, "2026-07-10", false)

	due, err := f.svc.DueList(ctx, qDay(t, "2026-08-01"), 0, f.company)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].EmployeeID != bernd || due[0].Status != QualificationStandAbgelaufen {
		t.Fatalf("nur Bernd ist ueberfaellig: %+v", due)
	}
	st, err := f.svc.Standing(ctx, bernd, qDay(t, "2026-08-01"), 0, f.company)
	if err != nil || len(st) != 1 || st[0].Status != QualificationStandAbgelaufen {
		t.Fatalf("Bernd: %v %+v", err, st)
	}
	st, err = f.svc.Standing(ctx, anna, qDay(t, "2026-08-01"), 0, f.company)
	if err != nil || len(st) != 1 || st[0].Status != QualificationStandGueltig {
		t.Fatalf("Anna: %v %+v", err, st)
	}
}
