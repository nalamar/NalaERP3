package hr

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Nachweise gegen echtes Postgres (Backlog F.2.4.2). Jeder Test arbeitet
// in einem eigenen, frisch angelegten Mandanten (seedCatalogCompany).
//
// Alle Erwerbsdaten liegen fest in der Vergangenheit, damit kein Fall
// versehentlich an "in der Zukunft" scheitert.

type qualFixture struct {
	company string
	svc     *QualificationService
	types   *QualificationTypeService
	emps    *EmployeeService
}

func newQualFixture(t *testing.T, env *testutil.IntegrationEnv) qualFixture {
	t.Helper()
	return qualFixture{
		company: seedCatalogCompany(t, env),
		svc:     NewQualificationService(env.PG),
		types:   NewQualificationTypeService(env.PG),
		emps:    NewEmployeeService(env.PG),
	}
}

func (f qualFixture) employee(t *testing.T, lastName string) uuid.UUID {
	t.Helper()
	e, err := f.emps.Create(context.Background(), EmployeeInput{FirstName: "Quali", LastName: lastName}, f.company)
	if err != nil {
		t.Fatalf("Mitarbeiter anlegen: %v", err)
	}
	return e.ID
}

func (f qualFixture) qualType(t *testing.T, code string, months *int) QualificationType {
	t.Helper()
	kategorie := QualificationKategorieQualifikation
	if months != nil {
		kategorie = QualificationKategorieUnterweisung
	}
	qt, err := f.types.Create(context.Background(), QualificationTypeInput{Code: code, Name: "Art " + code, Kategorie: kategorie, GueltigkeitMonate: months}, f.company)
	if err != nil {
		t.Fatalf("Art anlegen: %v", err)
	}
	return *qt
}

func (f qualFixture) mustCreate(t *testing.T, in QualificationInput) []EmployeeQualification {
	t.Helper()
	out, err := f.svc.Create(context.Background(), in, "", f.company)
	if err != nil {
		t.Fatalf("Nachweis erfassen: %v", err)
	}
	return out
}

func wantDate(t *testing.T, what string, got *time.Time, want string) {
	t.Helper()
	switch {
	case want == "" && got != nil:
		t.Fatalf("%s: erwartet kein Datum, bekommen %s", what, got.Format("2006-01-02"))
	case want != "" && (got == nil || got.Format("2006-01-02") != want):
		t.Fatalf("%s: erwartet %s, bekommen %v", what, want, got)
	}
}

// gueltig_bis wird beim Erfassen festgelegt und GESPEICHERT. Aendert der
// Betrieb spaeter die Frist der Art, gilt ein bereits ausgestellter
// Nachweis nicht rueckwirkend anders.
func TestQualificationCreateStoresValidUntil(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	f := newQualFixture(t, env)
	emp := f.employee(t, "Gueltigkeit")
	annual := f.qualType(t, "ARBEITSSCHUTZ", intPtr(12))
	unlimited := f.qualType(t, "STAPLER_SCHEIN", nil)
	userID := testutil.SeedAuthUser(t, env, "quali-create@example.invalid", "geheim-123456", "hr")

	// Aus der Frist berechnet.
	created, err := f.svc.Create(ctx, QualificationInput{
		EmployeeIDs: []uuid.UUID{emp}, TypeID: annual.ID, ErworbenAm: qDay(t, "2026-03-15"),
		Aussteller: "Fachkraft für Arbeitssicherheit", NachweisNr: "U-2026-017", Note: "Jahresunterweisung",
	}, userID, f.company)
	if err != nil {
		t.Fatalf("Erfassen: %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("erwartet einen Nachweis, bekommen %d", len(created))
	}
	rec := created[0]
	wantDate(t, "gueltig_bis (berechnet)", rec.GueltigBis, "2027-03-15")
	if rec.EmployeeID != emp || rec.TypeID != annual.ID || rec.Status != QualificationRecordActive || rec.ErworbenAm.Format("2006-01-02") != "2026-03-15" {
		t.Fatalf("Nachweis nicht wie erfasst: %+v", rec)
	}
	if rec.CreatedByUserID == nil || *rec.CreatedByUserID != userID {
		t.Fatalf("erwartet created_by_user_id=%s, bekommen %v", userID, rec.CreatedByUserID)
	}

	// Erneut laden: die Rueckgabe allein beweist nicht, was gespeichert ist.
	got, err := f.svc.Get(ctx, rec.ID, f.company)
	if err != nil {
		t.Fatalf("Laden: %v", err)
	}
	wantDate(t, "gueltig_bis (gespeichert)", got.GueltigBis, "2027-03-15")
	if got.Aussteller != "Fachkraft für Arbeitssicherheit" || got.NachweisNr != "U-2026-017" || got.Note != "Jahresunterweisung" {
		t.Fatalf("Angaben nicht gespeichert: %+v", got)
	}

	// Der Betrieb verdoppelt die Frist der Art: der alte Nachweis bleibt,
	// ein neuer bekommt die neue Frist.
	if _, err := f.types.Update(ctx, annual.ID, QualificationTypeUpdate{Name: annual.Name, Kategorie: annual.Kategorie, GueltigkeitMonate: intPtr(24), Aktiv: true}, f.company); err != nil {
		t.Fatalf("Frist aendern: %v", err)
	}
	after, err := f.svc.Get(ctx, rec.ID, f.company)
	if err != nil {
		t.Fatalf("Laden: %v", err)
	}
	wantDate(t, "gueltig_bis nach Friständerung", after.GueltigBis, "2027-03-15")
	renewed := f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{emp}, TypeID: annual.ID, ErworbenAm: qDay(t, "2026-09-01")})
	wantDate(t, "gueltig_bis mit neuer Frist", renewed[0].GueltigBis, "2028-09-01")

	// Ausdruecklich angegeben: gilt, auch gegen die Frist der Art.
	explicit := f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{emp}, TypeID: annual.ID, ErworbenAm: qDay(t, "2026-01-10"), GueltigBis: qDayPtr(t, "2026-07-10")})
	wantDate(t, "gueltig_bis (angegeben)", explicit[0].GueltigBis, "2026-07-10")

	// Art ohne Frist: unbefristet - es sei denn, das Zertifikat nennt ein Datum.
	noLimit := f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{emp}, TypeID: unlimited.ID, ErworbenAm: qDay(t, "2020-05-05")})
	wantDate(t, "gueltig_bis (unbefristet)", noLimit[0].GueltigBis, "")
	limited := f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{emp}, TypeID: unlimited.ID, ErworbenAm: qDay(t, "2021-05-05"), GueltigBis: qDayPtr(t, "2031-05-05")})
	wantDate(t, "gueltig_bis (angegeben bei unbefristeter Art)", limited[0].GueltigBis, "2031-05-05")

	// Monatsende ueber den echten Schreibpfad: 29.02.2024 plus 24 Monate
	// (die Frist der Art steht inzwischen auf 24).
	monthEnd := f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{emp}, TypeID: annual.ID, ErworbenAm: qDay(t, "2024-02-29")})
	wantDate(t, "gueltig_bis (29.02. plus 24 Monate)", monthEnd[0].GueltigBis, "2026-02-28")
}

// Eine Unterweisung fuer die ganze Werkstatt: ein Aufruf, je Mitarbeiter
// ein Nachweis, ganz oder gar nicht.
func TestQualificationCreateForGroupIsAtomic(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	f := newQualFixture(t, env)
	other := newQualFixture(t, env)
	a, b, c := f.employee(t, "GruppeA"), f.employee(t, "GruppeB"), f.employee(t, "GruppeC")
	foreign := other.employee(t, "Fremd")
	annual := f.qualType(t, "ARBEITSSCHUTZ", intPtr(12))
	count := func(t *testing.T) int {
		t.Helper()
		var n int
		if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM hr_employee_qualifications WHERE type_id=$1`, annual.ID).Scan(&n); err != nil {
			t.Fatalf("Nachweise zaehlen: %v", err)
		}
		return n
	}
	in := QualificationInput{TypeID: annual.ID, ErworbenAm: qDay(t, "2026-03-15"), Aussteller: "Meister"}

	// Scheitert EINER, entsteht fuer KEINEN ein Nachweis.
	in.EmployeeIDs = []uuid.UUID{a, uuid.New(), b}
	_, err := f.svc.Create(ctx, in, "", f.company)
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
	in.EmployeeIDs = []uuid.UUID{a, foreign}
	_, err = f.svc.Create(ctx, in, "", f.company)
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
	if n := count(t); n != 0 {
		t.Fatalf("gescheiterte Gruppenerfassung hat %d Nachweise hinterlassen", n)
	}

	// Ein doppelt genannter Mitarbeiter bekommt einen Nachweis.
	in.EmployeeIDs = []uuid.UUID{c, a, b, a}
	created, err := f.svc.Create(ctx, in, "", f.company)
	if err != nil {
		t.Fatalf("Gruppenerfassung: %v", err)
	}
	if len(created) != 3 || count(t) != 3 {
		t.Fatalf("erwartet 3 Nachweise, bekommen %d (in der DB %d)", len(created), count(t))
	}
	seenEmp, seenID := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	for _, q := range created {
		seenEmp[q.EmployeeID], seenID[q.ID] = true, true
		wantDate(t, "gueltig_bis in der Gruppe", q.GueltigBis, "2027-03-15")
		if q.Aussteller != "Meister" {
			t.Fatalf("Angaben nicht fuer jeden uebernommen: %+v", q)
		}
	}
	if len(seenEmp) != 3 || len(seenID) != 3 || !seenEmp[a] || !seenEmp[b] || !seenEmp[c] {
		t.Fatalf("erwartet je Mitarbeiter einen eigenen Nachweis: %+v", created)
	}

	// Dieselbe Liste noch einmal (Doppelklick): abgelehnt, nichts kommt dazu
	// - auch nicht fuer einen neuen Mitarbeiter in derselben Liste.
	d := f.employee(t, "GruppeD")
	in.EmployeeIDs = []uuid.UUID{d, a}
	_, err = f.svc.Create(ctx, in, "", f.company)
	wantEmployeeError(t, err, "Nachweis für diesen Tag ist bereits erfasst")
	if n := count(t); n != 3 {
		t.Fatalf("abgelehnte Doppelerfassung hat Nachweise hinzugefuegt: %d", n)
	}

	// Die Doppelpruefung gilt je ART: am selben Tag darf derselbe
	// Mitarbeiter eine ANDERE Art erwerben (Unterweisung und Beauftragung
	// am selben Vormittag sind der Normalfall).
	otherType := f.qualType(t, "STAPLER_UNTERWEISUNG", intPtr(12))
	sameDay := QualificationInput{EmployeeIDs: []uuid.UUID{a}, TypeID: otherType.ID, ErworbenAm: qDay(t, "2026-03-15")}
	if _, err := f.svc.Create(ctx, sameDay, "", f.company); err != nil {
		t.Fatalf("andere Art am selben Tag muss zulaessig sein, bekommen %v", err)
	}

	// Eine Wiederholung an einem anderen Tag ist ein neuer Nachweis.
	in.EmployeeIDs, in.ErworbenAm = []uuid.UUID{a}, qDay(t, "2026-09-15")
	if _, err := f.svc.Create(ctx, in, "", f.company); err != nil {
		t.Fatalf("Wiederholung an anderem Tag: %v", err)
	}
}

func TestQualificationCreateChecksTypeAndCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	f := newQualFixture(t, env)
	other := newQualFixture(t, env)
	emp := f.employee(t, "ArtPruefung")
	own := f.qualType(t, "ERSTHELFER", intPtr(24))
	foreignType := other.qualType(t, "ERSTHELFER", intPtr(24))
	in := QualificationInput{EmployeeIDs: []uuid.UUID{emp}, ErworbenAm: qDay(t, "2026-03-15")}

	// Art eines anderen Mandanten und unbekannte Art.
	in.TypeID = foreignType.ID
	_, err := f.svc.Create(ctx, in, "", f.company)
	wantEmployeeError(t, err, "Qualifikationsart nicht gefunden")
	in.TypeID = uuid.New()
	_, err = f.svc.Create(ctx, in, "", f.company)
	wantEmployeeError(t, err, "Qualifikationsart nicht gefunden")

	// Eigener Mitarbeiter und eigene Art, aber aus Sicht des ANDEREN
	// Mandanten: nichts davon gehoert ihm.
	in.TypeID = own.ID
	_, err = f.svc.Create(ctx, in, "", other.company)
	wantEmployeeError(t, err, "Qualifikationsart nicht gefunden")

	first := f.mustCreate(t, in)

	// Stillgelegte Art: keine NEUEN Nachweise, die bestehenden bleiben.
	if _, err := f.types.Update(ctx, own.ID, QualificationTypeUpdate{Name: own.Name, Kategorie: own.Kategorie, GueltigkeitMonate: own.GueltigkeitMonate, Aktiv: false}, f.company); err != nil {
		t.Fatalf("Art stilllegen: %v", err)
	}
	in.ErworbenAm = qDay(t, "2026-04-15")
	_, err = f.svc.Create(ctx, in, "", f.company)
	wantEmployeeError(t, err, "Qualifikationsart ist stillgelegt")
	list, err := f.svc.ListForEmployee(ctx, emp, false, f.company)
	if err != nil {
		t.Fatalf("Liste: %v", err)
	}
	if len(list) != 1 || list[0].ID != first[0].ID {
		t.Fatalf("bestehender Nachweis einer stillgelegten Art muss bleiben: %+v", list)
	}
}

func sameQualificationFacts(a, b *EmployeeQualification) bool {
	bisEqual := (a.GueltigBis == nil) == (b.GueltigBis == nil) && (a.GueltigBis == nil || a.GueltigBis.Equal(*b.GueltigBis))
	return a.ID == b.ID && a.EmployeeID == b.EmployeeID && a.TypeID == b.TypeID && a.ErworbenAm.Equal(b.ErworbenAm) && bisEqual &&
		a.Aussteller == b.Aussteller && a.NachweisNr == b.NachweisNr && a.Note == b.Note && a.CreatedAt.Equal(b.CreatedAt)
}

func TestQualificationStornoKeepsFactsAndIsOneTime(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	f := newQualFixture(t, env)
	emp := f.employee(t, "Storno")
	annual := f.qualType(t, "ARBEITSSCHUTZ", intPtr(12))
	userID := testutil.SeedAuthUser(t, env, "quali-storno@example.invalid", "geheim-123456", "hr")
	in := QualificationInput{EmployeeIDs: []uuid.UUID{emp}, TypeID: annual.ID, ErworbenAm: qDay(t, "2026-03-15"), Aussteller: "Meister", NachweisNr: "U-1", Note: "falsches Datum"}
	original := f.mustCreate(t, in)[0]

	before := time.Now().Add(-time.Minute)
	storniert, err := f.svc.Storno(ctx, original.ID, "  Datum falsch erfasst  ", userID, f.company)
	if err != nil {
		t.Fatalf("Storno: %v", err)
	}
	if storniert.Status != QualificationRecordStorniert || storniert.StornoReason != "Datum falsch erfasst" {
		t.Fatalf("erwartet storniert mit getrimmter Begruendung, bekommen %+v", storniert)
	}
	if storniert.StorniertAt == nil || storniert.StorniertAt.Before(before) {
		t.Fatalf("erwartet aktuellen Storno-Zeitpunkt, bekommen %v", storniert.StorniertAt)
	}
	if storniert.StorniertByUserID == nil || *storniert.StorniertByUserID != userID {
		t.Fatalf("erwartet storniert_by_user_id=%s, bekommen %v", userID, storniert.StorniertByUserID)
	}
	// Der Nachweis bleibt nachlesbar: der Storno schreibt nur die Storno-Spalten.
	if !sameQualificationFacts(&original, storniert) {
		t.Fatalf("Storno hat erfasste Angaben veraendert:\nvorher  %+v\nnachher %+v", original, storniert)
	}

	// Einmalig.
	_, err = f.svc.Storno(ctx, original.ID, "Zweiter Versuch", "", f.company)
	wantEmployeeError(t, err, "Nachweis ist bereits storniert")
	again, err := f.svc.Get(ctx, original.ID, f.company)
	if err != nil {
		t.Fatalf("Laden: %v", err)
	}
	if again.StornoReason != "Datum falsch erfasst" || !again.StorniertAt.Equal(*storniert.StorniertAt) || again.StorniertByUserID == nil {
		t.Fatalf("zweiter Storno hat den ersten ueberschrieben: %+v", again)
	}

	// Nach dem Storno ist der Tag wieder frei: die Korrektur wird neu erfasst.
	in.ErworbenAm = qDay(t, "2026-03-16")
	corrected := f.mustCreate(t, in)[0]
	in.ErworbenAm = qDay(t, "2026-03-15")
	if _, err := f.svc.Create(ctx, in, "", f.company); err != nil {
		t.Fatalf("derselbe Tag muss nach dem Storno wieder erfassbar sein, bekommen %v", err)
	}

	// Liste: Standard ohne stornierte, juengste zuerst; mit Schalter alle.
	active, err := f.svc.ListForEmployee(ctx, emp, false, f.company)
	if err != nil {
		t.Fatalf("Liste: %v", err)
	}
	if len(active) != 2 || active[0].ID != corrected.ID {
		t.Fatalf("Standardliste: erwartet 2 aktive, juengster zuerst; bekommen %+v", active)
	}
	for _, q := range active {
		if q.Status != QualificationRecordActive {
			t.Fatalf("Standardliste enthaelt einen stornierten Nachweis: %+v", q)
		}
	}
	all, err := f.svc.ListForEmployee(ctx, emp, true, f.company)
	if err != nil {
		t.Fatalf("Liste mit stornierten: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("Liste mit stornierten: erwartet 3, bekommen %d", len(all))
	}
}

func TestQualificationIsScopedToCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	f := newQualFixture(t, env)
	other := newQualFixture(t, env)
	emp := f.employee(t, "Mandant")
	colleague := f.employee(t, "MandantKollege")
	annual := f.qualType(t, "ARBEITSSCHUTZ", intPtr(12))
	rec := f.mustCreate(t, QualificationInput{EmployeeIDs: []uuid.UUID{emp}, TypeID: annual.ID, ErworbenAm: qDay(t, "2026-03-15")})[0]

	_, err := f.svc.Get(ctx, rec.ID, other.company)
	wantEmployeeError(t, err, "Nachweis nicht gefunden")
	_, err = f.svc.Get(ctx, uuid.New(), f.company)
	wantEmployeeError(t, err, "Nachweis nicht gefunden")
	_, err = f.svc.Storno(ctx, rec.ID, "Fremdzugriff", "", other.company)
	wantEmployeeError(t, err, "Nachweis nicht gefunden")
	_, err = f.svc.Storno(ctx, uuid.New(), "Unbekannt", "", f.company)
	wantEmployeeError(t, err, "Nachweis nicht gefunden")
	_, err = f.svc.ListForEmployee(ctx, emp, true, other.company)
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
	_, err = f.svc.ListForEmployee(ctx, uuid.New(), true, f.company)
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")

	still, err := f.svc.Get(ctx, rec.ID, f.company)
	if err != nil {
		t.Fatalf("Laden: %v", err)
	}
	if still.Status != QualificationRecordActive {
		t.Fatalf("fremder Mandant hat den Nachweis storniert: %+v", still)
	}
	// Die Liste eines Mitarbeiters enthaelt nur dessen Nachweise.
	if list, err := f.svc.ListForEmployee(ctx, colleague, true, f.company); err != nil || len(list) != 0 {
		t.Fatalf("Liste des Kollegen muss leer sein: %+v, %v", list, err)
	}
}

// Derselbe Nachweis, mehrfach gleichzeitig abgeschickt: genau einer wird
// erfasst. Ohne die Sperre auf den Mitarbeiterzeilen koennten mehrere
// Aufrufe die Doppelpruefung zugleich passieren - eine Eindeutigkeit in
// der Datenbank gibt es hier nicht, die sie auffinge.
func TestQualificationConcurrentCreateStoresExactlyOne(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	f := newQualFixture(t, env)
	a, b := f.employee(t, "GleichzeitigA"), f.employee(t, "GleichzeitigB")
	annual := f.qualType(t, "ARBEITSSCHUTZ", intPtr(12))
	in := QualificationInput{TypeID: annual.ID, ErworbenAm: qDay(t, "2026-03-15")}

	const n = 8
	errs := make([]error, n)
	var wg sync.WaitGroup
	gate := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			call := in
			// Abwechselnde Reihenfolge der Mitarbeiter: die Sperren muessen
			// trotzdem in fester Reihenfolge genommen werden, sonst
			// blockieren sich zwei Aufrufe gegenseitig.
			if i%2 == 0 {
				call.EmployeeIDs = []uuid.UUID{a, b}
			} else {
				call.EmployeeIDs = []uuid.UUID{b, a}
			}
			<-gate
			_, errs[i] = f.svc.Create(ctx, call, "", f.company)
		}(i)
	}
	close(gate)
	wg.Wait()

	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
			continue
		}
		if err.Error() != "Nachweis für diesen Tag ist bereits erfasst" {
			t.Fatalf("erwartet fachliche Meldung, bekommen %q", err.Error())
		}
	}
	if ok != 1 {
		t.Fatalf("erwartet genau eine erfolgreiche Erfassung, bekommen %d", ok)
	}
	var count int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM hr_employee_qualifications WHERE type_id=$1`, annual.ID).Scan(&count); err != nil {
		t.Fatalf("Nachweise zaehlen: %v", err)
	}
	if count != 2 {
		t.Fatalf("erwartet genau zwei Nachweise (einer je Mitarbeiter), bekommen %d", count)
	}
}
