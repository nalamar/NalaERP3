package hr

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Bestaetigungen ueber den Service gegen echtes Postgres (Backlog
// F.10.3.2b).

func confirmationFixture(t *testing.T, env *testutil.IntegrationEnv) (qualFixture, QualificationType, EmployeeQualification) {
	t.Helper()
	f := newQualFixture(t, env)
	weld, err := f.types.Create(context.Background(), QualificationTypeInput{Code: "SCHWEISSER", Name: "Schweißerprüfung", Kategorie: QualificationKategorieQualifikation,
		GueltigkeitMonate: intPtr(36), BestaetigungMonate: intPtr(6)}, f.company)
	if err != nil {
		t.Fatalf("Art anlegen: %v", err)
	}
	rec := f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{f.employee(t, "Adler")}, TypeID: weld.ID, ErworbenAm: qDay(t, "2026-01-15")})[0]
	return f, *weld, rec
}

func TestQualificationConfirmationCreateStornoAndList(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	f, _, rec := confirmationFixture(t, env)
	userID := testutil.SeedAuthUser(t, env, "quali-confirm@example.invalid", "geheim-123456", "hr")

	first, err := f.svc.CreateConfirmation(ctx, rec.ID, QualificationConfirmationInput{BestaetigtAm: qDay(t, "2026-07-10"), BestaetigtDurch: "  Schweissaufsicht ", Note: "Baustelle Nord"}, userID, f.company)
	if err != nil {
		t.Fatalf("Erfassen: %v", err)
	}
	if first.QualificationID != rec.ID || first.Status != QualificationRecordActive || first.BestaetigtDurch != "Schweissaufsicht" ||
		first.BestaetigtAm.Format("2006-01-02") != "2026-07-10" || first.Note != "Baustelle Nord" || first.CreatedByUserID == nil || *first.CreatedByUserID != userID {
		t.Fatalf("nicht wie erfasst: %+v", first)
	}
	// Derselbe Tag ein zweites Mal ist ein Doppelklick.
	if _, err := f.svc.CreateConfirmation(ctx, rec.ID, QualificationConfirmationInput{BestaetigtAm: qDay(t, "2026-07-10"), BestaetigtDurch: "Schweissaufsicht"}, "", f.company); err == nil || err.Error() != "Bestätigung für diesen Tag ist bereits erfasst" {
		t.Fatalf("Doppelt: %v", err)
	}
	second, err := f.svc.CreateConfirmation(ctx, rec.ID, QualificationConfirmationInput{BestaetigtAm: qDay(t, "2026-08-20"), BestaetigtDurch: "Arbeitgeber"}, "", f.company)
	if err != nil {
		t.Fatalf("zweite Bestaetigung: %v", err)
	}

	list, err := f.svc.ListConfirmations(ctx, rec.ID, false, f.company)
	if err != nil || len(list) != 2 || list[0].ID != second.ID || list[1].ID != first.ID {
		t.Fatalf("Liste (juengste zuerst): %v %+v", err, list)
	}

	// Stornieren: einmalig, mit Begruendung; danach nicht mehr in der Standardliste.
	st, err := f.svc.StornoConfirmation(ctx, first.ID, "Falsches Datum", userID, f.company)
	if err != nil {
		t.Fatalf("Storno: %v", err)
	}
	if st.Status != QualificationRecordStorniert || st.StornoReason != "Falsches Datum" || st.StorniertAt == nil || st.StorniertByUserID == nil || *st.StorniertByUserID != userID ||
		st.BestaetigtAm.Format("2006-01-02") != "2026-07-10" || st.BestaetigtDurch != "Schweissaufsicht" {
		t.Fatalf("Storno: %+v", st)
	}
	if _, err := f.svc.StornoConfirmation(ctx, first.ID, "nochmal", "", f.company); err == nil || err.Error() != "Bestätigung ist bereits storniert" {
		t.Fatalf("zweites Storno: %v", err)
	}
	if list, _ := f.svc.ListConfirmations(ctx, rec.ID, false, f.company); len(list) != 1 || list[0].ID != second.ID {
		t.Fatalf("Liste ohne stornierte: %+v", list)
	}
	if list, _ := f.svc.ListConfirmations(ctx, rec.ID, true, f.company); len(list) != 2 {
		t.Fatalf("Liste mit stornierten: %+v", list)
	}
	// Nach dem Storno darf derselbe Tag neu erfasst werden.
	if _, err := f.svc.CreateConfirmation(ctx, rec.ID, QualificationConfirmationInput{BestaetigtAm: qDay(t, "2026-07-10"), BestaetigtDurch: "Schweissaufsicht"}, "", f.company); err != nil {
		t.Fatalf("neu nach Storno: %v", err)
	}

	// Erfasste Bestaetigungen wirken im Stand (Regel aus F.10.3.2a):
	// 2026-07-10 und 2026-08-20 sind rechtzeitig, faellig 2027-02-20.
	standing, err := f.svc.Standing(ctx, rec.EmployeeID, qDay(t, "2026-10-01"), 60, f.company)
	if err != nil || len(standing) != 1 {
		t.Fatalf("Stand: %v %+v", err, standing)
	}
	wantDate(t, "bestaetigung_faellig_am", standing[0].BestaetigungFaelligAm, "2027-02-20")

	// Eine verspaetete Bestaetigung wird erfasst; sie zaehlt nur nicht.
	other := f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{f.employee(t, "Berg")}, TypeID: rec.TypeID, ErworbenAm: qDay(t, "2026-01-15")})[0]
	if _, err := f.svc.CreateConfirmation(ctx, other.ID, QualificationConfirmationInput{BestaetigtAm: qDay(t, "2026-07-16"), BestaetigtDurch: "Arbeitgeber"}, "", f.company); err != nil {
		t.Fatalf("verspaetete Bestaetigung: %v", err)
	}
	standing, err = f.svc.Standing(ctx, other.EmployeeID, qDay(t, "2026-10-01"), 60, f.company)
	if err != nil || len(standing) != 1 || standing[0].Status != QualificationStandAbgelaufen {
		t.Fatalf("Stand nach verspaeteter Bestaetigung: %v %+v", err, standing)
	}
	wantDate(t, "bestaetigung_faellig_am", standing[0].BestaetigungFaelligAm, "2026-07-15")
}

func TestQualificationConfirmationCreateRejects(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	f, _, rec := confirmationFixture(t, env)
	ok := QualificationConfirmationInput{BestaetigtAm: qDay(t, "2026-07-10"), BestaetigtDurch: "Schweissaufsicht"}

	// Art ohne Zwischenbestaetigung.
	plain := f.qualType(t, "STAPLER", nil)
	plainRec := f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{rec.EmployeeID}, TypeID: plain.ID, ErworbenAm: qDay(t, "2026-01-15")})[0]

	// Stornierter Nachweis.
	stornoRec := f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{rec.EmployeeID}, TypeID: rec.TypeID, ErworbenAm: qDay(t, "2026-02-01")})[0]
	if _, err := f.svc.Storno(ctx, stornoRec.ID, "Falsch erfasst", "", f.company); err != nil {
		t.Fatalf("Nachweis stornieren: %v", err)
	}

	future := WorkDateOf(time.Now()).AddDate(0, 0, 2)
	cases := []struct {
		name    string
		id      uuid.UUID
		in      QualificationConfirmationInput
		wantErr string
	}{
		{"unbekannter Nachweis", uuid.New(), ok, "Nachweis nicht gefunden"},
		{"Art ohne Intervall", plainRec.ID, ok, "Qualifikationsart sieht keine Zwischenbestätigung vor"},
		{"Nachweis storniert", stornoRec.ID, ok, "Nachweis ist storniert"},
		{"vor dem Erwerb", rec.ID, QualificationConfirmationInput{BestaetigtAm: qDay(t, "2026-01-14"), BestaetigtDurch: "x"}, "Bestätigungsdatum darf nicht vor dem Erwerbsdatum liegen"},
		{"in der Zukunft", rec.ID, QualificationConfirmationInput{BestaetigtAm: future, BestaetigtDurch: "x"}, "Bestätigungsdatum darf nicht in der Zukunft liegen"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := f.svc.CreateConfirmation(ctx, tc.id, tc.in, "", f.company)
			if got != nil || err == nil || err.Error() != tc.wantErr {
				t.Fatalf("erwartet %q, bekommen %+v %v", tc.wantErr, got, err)
			}
		})
	}
	for _, id := range []uuid.UUID{rec.ID, plainRec.ID, stornoRec.ID} {
		if list, err := f.svc.ListConfirmations(ctx, id, true, f.company); err != nil || len(list) != 0 {
			t.Fatalf("nichts erfasst erwartet: %v %+v", err, list)
		}
	}
}

// Ein anderer Mandant sieht, erfasst und storniert nichts am Nachweis.
func TestQualificationConfirmationsAreScopedToCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	f, _, rec := confirmationFixture(t, env)
	other := newQualFixture(t, env)
	c, err := f.svc.CreateConfirmation(ctx, rec.ID, QualificationConfirmationInput{BestaetigtAm: qDay(t, "2026-07-10"), BestaetigtDurch: "Schweissaufsicht"}, "", f.company)
	if err != nil {
		t.Fatalf("Erfassen: %v", err)
	}

	if _, err := other.svc.CreateConfirmation(ctx, rec.ID, QualificationConfirmationInput{BestaetigtAm: qDay(t, "2026-07-11"), BestaetigtDurch: "Fremd"}, "", other.company); err == nil || err.Error() != "Nachweis nicht gefunden" {
		t.Fatalf("fremdes Erfassen: %v", err)
	}
	if _, err := other.svc.ListConfirmations(ctx, rec.ID, true, other.company); err == nil || err.Error() != "Nachweis nicht gefunden" {
		t.Fatalf("fremde Liste: %v", err)
	}
	if _, err := other.svc.StornoConfirmation(ctx, c.ID, "Fremdzugriff", "", other.company); err == nil || err.Error() != "Bestätigung nicht gefunden" {
		t.Fatalf("fremdes Storno: %v", err)
	}
	list, err := f.svc.ListConfirmations(ctx, rec.ID, true, f.company)
	if err != nil || len(list) != 1 || list[0].Status != QualificationRecordActive {
		t.Fatalf("unveraendert erwartet: %v %+v", err, list)
	}
}
