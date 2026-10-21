package hr

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Nachweisdokumente (Backlog F.10.1): Dateinamen und die Mandantenpruefung
// des Service selbst (die Route prueft vorher ebenfalls).

func TestCleanFilename(t *testing.T) {
	for in, want := range map[string]string{
		`C:\Scans\zertifikat.pdf`: "zertifikat.pdf",
		"../../foto.jpg":          "foto.jpg",
		"/etc/passwd":             "passwd",
		" liste.png ":             "liste.png",
		"":                        "nachweis",
		"/":                       "nachweis",
		".":                       "nachweis",
	} {
		if got := cleanFilename(in); got != want {
			t.Errorf("%q: %q, erwartet %q", in, got, want)
		}
	}
}

func TestQualificationDocumentsListChecksCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedWorkdaysCompany(t, env)
	other := seedWorkdaysCompany(t, env)
	emp, err := NewEmployeeService(env.PG).Create(ctx, EmployeeInput{FirstName: "Do", LastName: "Kument"}, company)
	if err != nil {
		t.Fatal(err)
	}
	types := NewQualificationTypeService(env.PG)
	typ, err := types.Create(ctx, QualificationTypeInput{Code: "DOK_TEST", Name: "Dokumenttest", Kategorie: "qualifikation"}, company)
	if err != nil {
		t.Fatal(err)
	}
	quals := NewQualificationService(env.PG)
	recs, err := quals.Create(ctx, QualificationInput{EmployeeIDs: []uuid.UUID{emp.ID}, TypeID: typ.ID, ErworbenAm: leaveDay(2026, 3, 1)}, "", company)
	if err != nil {
		t.Fatal(err)
	}
	docs := NewQualificationDocumentService(quals, nil, "")
	if list, err := docs.List(ctx, recs[0].ID, company); err != nil || len(list) != 0 {
		t.Fatalf("eigener Mandant: %v, %v", list, err)
	}
	_, err = docs.List(ctx, recs[0].ID, other)
	wantEmployeeError(t, err, "Nachweis nicht gefunden")
	// Ohne MongoDB: Hochladen meldet das klar, statt abzustuerzen.
	_, err = docs.Upload(ctx, recs[0].ID, strings.NewReader("%PDF-1.4\n"), "x.pdf", "", company)
	wantEmployeeError(t, err, "MongoDB nicht konfiguriert")
}
