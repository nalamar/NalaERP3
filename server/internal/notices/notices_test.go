package notices

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/assets"
	"nalaerp3/internal/fleet"
	"nalaerp3/internal/hr"
)

// Hinweise ohne Datenbank (Backlog F.10.4.1): Quellen mit Fakes.

type fakeQualDue struct {
	entries []hr.QualificationDueEntry
	err     error
	calls   int
}

func (f *fakeQualDue) DueList(ctx context.Context, today time.Time, warnDays int, companyID string) ([]hr.QualificationDueEntry, error) {
	f.calls++
	return f.entries, f.err
}

type fakeStanding struct {
	standing []hr.EmployeeQualificationStanding
	gotEmp   uuid.UUID
	calls    int
}

func (f *fakeStanding) Standing(ctx context.Context, employeeID uuid.UUID, today time.Time, warnDays int, companyID string) ([]hr.EmployeeQualificationStanding, error) {
	f.calls++
	f.gotEmp = employeeID
	return f.standing, nil
}

type fakeAssetDue struct{ entries []assets.AssetDueEntry }

func (f fakeAssetDue) DueList(ctx context.Context, today time.Time, warnDays int, companyID string) ([]assets.AssetDueEntry, error) {
	return f.entries, nil
}

type fakeFleetDue struct{ entries []fleet.ScheduleDueEntry }

func (f fakeFleetDue) DueList(ctx context.Context, today time.Time, warnDays int, companyID string) ([]fleet.ScheduleDueEntry, error) {
	return f.entries, nil
}

func qualDue(status, first, last string) hr.QualificationDueEntry {
	e := hr.QualificationDueEntry{EmployeeID: uuid.New(), FirstName: first, LastName: last}
	e.Status = status
	return e
}

func qualSt(status string) hr.EmployeeQualificationStanding {
	var s hr.EmployeeQualificationStanding
	s.Status = status
	return s
}

func recipient(perms ...string) Recipient {
	set := map[string]bool{}
	for _, p := range perms {
		set[p] = true
	}
	return Recipient{UserID: "u1", CompanyID: "c1", Can: func(p string) bool { return set[p] }}
}

var today = time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)

func TestQualificationsDueCountsAndPermission(t *testing.T) {
	due := &fakeQualDue{entries: []hr.QualificationDueEntry{
		qualDue(hr.QualificationStandLaeuftAb, "Anna", "Adler"),
		qualDue(hr.QualificationStandAbgelaufen, "Bernd", "Berg"),
		qualDue(hr.QualificationStandLaeuftAb, "Clara", "Christ"),
	}}
	src := QualificationsDue{Svc: due}

	// Ohne qualifications.read: kein Hinweis, die Liste wird gar nicht geladen.
	if got, err := src.Notices(context.Background(), recipient("qualifications.self"), today); err != nil || len(got) != 0 || due.calls != 0 {
		t.Fatalf("ohne Recht: %v %+v (Aufrufe %d)", err, got, due.calls)
	}
	got, err := src.Notices(context.Background(), recipient("qualifications.read"), today)
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %+v", err, got)
	}
	n := got[0]
	if n.Kind != KindQualificationsDue || n.Total != 3 || n.Counts[hr.QualificationStandLaeuftAb] != 2 || n.Counts[hr.QualificationStandAbgelaufen] != 1 || n.Severity != SeverityKritisch {
		t.Fatalf("%+v", n)
	}
	// DSGVO: keine Namen im Hinweis - auch nicht im JSON, das spaeter in
	// eine Mail gehen koennte.
	raw, _ := json.Marshal(got)
	for _, name := range []string{"Anna", "Adler", "Bernd", "Berg", "Clara", "Christ"} {
		if strings.Contains(string(raw), name) {
			t.Fatalf("Name %q im Hinweis: %s", name, raw)
		}
	}

	// Nur bald faellig: Hinweis, nicht kritisch.
	due.entries = due.entries[:1]
	if got, _ := src.Notices(context.Background(), recipient("qualifications.read"), today); got[0].Severity != SeverityHinweis || got[0].Total != 1 {
		t.Fatalf("nur bald: %+v", got)
	}
	// Nichts faellig: kein Hinweis.
	due.entries = nil
	if got, _ := src.Notices(context.Background(), recipient("qualifications.read"), today); len(got) != 0 {
		t.Fatalf("leer: %+v", got)
	}
}

func TestOwnQualificationsNeedsLinkedEmployee(t *testing.T) {
	st := &fakeStanding{standing: []hr.EmployeeQualificationStanding{
		qualSt(hr.QualificationStandGueltig), qualSt(hr.QualificationStandFehlt), qualSt(hr.QualificationStandLaeuftAb),
	}}
	src := OwnQualifications{Svc: st}

	// Ohne verknuepften Mitarbeiter oder ohne Recht: nichts.
	if got, _ := src.Notices(context.Background(), recipient("qualifications.self"), today); len(got) != 0 || st.calls != 0 {
		t.Fatalf("ohne Mitarbeiter: %+v", got)
	}
	emp := uuid.New()
	r := recipient("time_entries.self")
	r.EmployeeID = &emp
	if got, _ := src.Notices(context.Background(), r, today); len(got) != 0 || st.calls != 0 {
		t.Fatalf("ohne Recht: %+v", got)
	}

	r = recipient("qualifications.self")
	r.EmployeeID = &emp
	got, err := src.Notices(context.Background(), r, today)
	if err != nil || len(got) != 1 || st.gotEmp != emp {
		t.Fatalf("%v %+v (Mitarbeiter %s)", err, got, st.gotEmp)
	}
	// Gueltiges zaehlt nicht.
	if n := got[0]; n.Kind != KindOwnQualifications || n.Total != 2 || n.Counts[hr.QualificationStandGueltig] != 0 || n.Severity != SeverityKritisch {
		t.Fatalf("%+v", n)
	}
	// Alles gueltig: kein Hinweis.
	st.standing = st.standing[:1]
	if got, _ := src.Notices(context.Background(), r, today); len(got) != 0 {
		t.Fatalf("alles gueltig: %+v", got)
	}
}

func TestAssetAndFleetSources(t *testing.T) {
	assetEntry := func(status string) assets.AssetDueEntry {
		var e assets.AssetDueEntry
		e.Status = status
		return e
	}
	a := AssetInspectionsDue{Svc: fakeAssetDue{entries: []assets.AssetDueEntry{assetEntry(assets.PruefstandFaelligBald), assetEntry(assets.PruefstandNieGeprueft)}}}
	if got, _ := a.Notices(context.Background(), recipient("fleet.read"), today); len(got) != 0 {
		t.Fatalf("ohne assets.read: %+v", got)
	}
	got, _ := a.Notices(context.Background(), recipient("assets.read"), today)
	if len(got) != 1 || got[0].Kind != KindAssetInspectionDue || got[0].Total != 2 || got[0].Severity != SeverityKritisch {
		t.Fatalf("Betriebsmittel: %+v", got)
	}

	f := FleetSchedulesDue{Svc: fakeFleetDue{entries: []fleet.ScheduleDueEntry{{Standing: fleet.ScheduleStanding{Status: fleet.TerminstandFaelligBald}}}}}
	if got, _ := f.Notices(context.Background(), recipient("assets.read"), today); len(got) != 0 {
		t.Fatalf("ohne fleet.read: %+v", got)
	}
	got, _ = f.Notices(context.Background(), recipient("fleet.read"), today)
	if len(got) != 1 || got[0].Kind != KindFleetSchedulesDue || got[0].Severity != SeverityHinweis || got[0].Counts[fleet.TerminstandFaelligBald] != 1 {
		t.Fatalf("Fuhrpark: %+v", got)
	}
	f.Svc = fakeFleetDue{entries: []fleet.ScheduleDueEntry{{Standing: fleet.ScheduleStanding{Status: fleet.TerminstandGesperrt}}}}
	if got, _ := f.Notices(context.Background(), recipient("fleet.read"), today); got[0].Severity != SeverityKritisch {
		t.Fatalf("gesperrt: %+v", got)
	}
}

func TestServiceOrdersCriticalFirstAndPropagatesErrors(t *testing.T) {
	bald := FleetSchedulesDue{Svc: fakeFleetDue{entries: []fleet.ScheduleDueEntry{{Standing: fleet.ScheduleStanding{Status: fleet.TerminstandFaelligBald}}}}}
	kritisch := QualificationsDue{Svc: &fakeQualDue{entries: []hr.QualificationDueEntry{qualDue(hr.QualificationStandFehlt, "A", "B")}}}
	svc := NewService(bald, kritisch)
	got, err := svc.For(context.Background(), recipient("fleet.read", "qualifications.read"), today)
	if err != nil || len(got) != 2 || got[0].Kind != KindQualificationsDue || got[1].Kind != KindFleetSchedulesDue {
		t.Fatalf("%v %+v", err, got)
	}
	// Ohne Rechte: leere Liste, nicht nil (JSON []).
	got, err = svc.For(context.Background(), recipient(), today)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("ohne Rechte: %v %#v", err, got)
	}
	// Fehler einer Quelle: kein Teilergebnis.
	broken := QualificationsDue{Svc: &fakeQualDue{err: errors.New("db weg")}}
	if got, err := NewService(bald, broken).For(context.Background(), recipient("fleet.read", "qualifications.read"), today); err == nil || got != nil {
		t.Fatalf("Fehler erwartet: %v %+v", err, got)
	}
}

// NewDefaultService enthaelt alle vier Quellen in fester Reihenfolge - so
// liefern Dashboard und E-Mail-Versand dieselben Hinweise.
func TestNewDefaultServiceHasAllSources(t *testing.T) {
	q := struct {
		*fakeQualDue
		*fakeStanding
	}{&fakeQualDue{}, &fakeStanding{}}
	s := NewDefaultService(q, fakeAssetDue{}, fakeFleetDue{})
	if len(s.sources) != 4 {
		t.Fatalf("Quellen: %d", len(s.sources))
	}
	if _, ok := s.sources[0].(QualificationsDue); !ok {
		t.Fatalf("1: %T", s.sources[0])
	}
	if _, ok := s.sources[1].(OwnQualifications); !ok {
		t.Fatalf("2: %T", s.sources[1])
	}
	if _, ok := s.sources[2].(AssetInspectionsDue); !ok {
		t.Fatalf("3: %T", s.sources[2])
	}
	if _, ok := s.sources[3].(FleetSchedulesDue); !ok {
		t.Fatalf("4: %T", s.sources[3])
	}
}
