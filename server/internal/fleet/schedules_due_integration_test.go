package fleet

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Betriebsweite Faelligkeitsliste (Backlog G.2.2.3). Alle Termine sind
// relativ zu heute gewaehlt, damit der Test an jedem Tag dasselbe ergibt.

type dueWorld struct {
	company  string
	vehicles []uuid.UUID
}

// newDueWorld baut:
//
//   - M-A 1: HU gesperrt (Maengel, Nachpruefung in 20 Tagen - zum
//     Monatsende), UVV
//     ueberfaellig seit 5 Tagen, Wartung ok (in 100 Tagen), Leasing ohne
//     Termin, Versicherung und "Tachograph" in 10 Tagen, "Alt" stillgelegt
//     und ueberfaellig
//   - M-B 1: UVV ueberfaellig seit 20 Tagen, HU in 3 Tagen
//   - M-C 1: Wartung in 3 Tagen (vor M-B 1 angelegt)
//   - M-E 1: HU nicht bestanden, ohne Nachpruefungstermin; UVV und
//     Wartung in 7 Tagen
//   - M-X 1: abgemeldet, UVV ueberfaellig
//   - ein fremder Mandant mit ueberfaelliger UVV
func newDueWorld(t *testing.T, env *testutil.IntegrationEnv) dueWorld {
	t.Helper()
	ctx := context.Background()
	svc := NewScheduleService(env.PG)
	now := today()
	rel := func(days int) *time.Time { d := now.AddDate(0, 0, days); return &d }
	w := dueWorld{company: seedCompany(t, env)}
	mk := func(company string, v *Vehicle, in ScheduleInput) *Schedule {
		t.Helper()
		in.VehicleID = v.ID
		sc, err := svc.CreateSchedule(ctx, in, "", company)
		if err != nil {
			t.Fatalf("Termin %s: %v", in.Art, err)
		}
		return sc
	}
	record := func(sc *Schedule, ergebnis string, next *time.Time) {
		t.Helper()
		if _, err := svc.RecordCompletion(ctx, CompletionInput{ScheduleID: sc.ID, ErledigtAm: now, Ergebnis: ergebnis, NaechsterTermin: next}, "", w.company); err != nil {
			t.Fatalf("Erledigung: %v", err)
		}
	}

	// M-C 1 vor M-B 1 angelegt: ihr gleich faelliger Termin muss trotzdem
	// nach Kennzeichen sortiert hinter M-B 1 stehen.
	c := seedVehicle(t, env, w.company, "M-C 1")
	mk(w.company, c, ScheduleInput{Art: TerminWartung, Starttermin: rel(3)})
	a := seedVehicle(t, env, w.company, "M-A 1")
	b := seedVehicle(t, env, w.company, "M-B 1")
	e := seedVehicle(t, env, w.company, "M-E 1")
	x := seedVehicle(t, env, w.company, "M-X 1")
	w.vehicles = []uuid.UUID{a.ID, b.ID, c.ID, e.ID, x.ID}

	huA := mk(w.company, a, ScheduleInput{Art: TerminHU, Starttermin: rel(400)})
	record(huA, ErgebnisMaengel, rel(20))
	mk(w.company, a, ScheduleInput{Art: TerminUVV, Starttermin: rel(-5)})
	mk(w.company, a, ScheduleInput{Art: TerminWartung, Starttermin: rel(100)})
	mk(w.company, a, ScheduleInput{Art: TerminLeasing})
	mk(w.company, a, ScheduleInput{Art: TerminVersicherung, Starttermin: rel(10)})
	mk(w.company, a, ScheduleInput{Art: TerminSonstige, Bezeichnung: "Tachograph", Starttermin: rel(10)})
	alt := mk(w.company, a, ScheduleInput{Art: TerminSonstige, Bezeichnung: "Alt", Starttermin: rel(-30)})
	if _, err := svc.UpdateSchedule(ctx, alt.ID, ScheduleUpdate{Starttermin: rel(-30), Aktiv: false}, w.company); err != nil {
		t.Fatalf("stilllegen: %v", err)
	}

	mk(w.company, b, ScheduleInput{Art: TerminUVV, Starttermin: rel(-20)})
	// HU am Monatsende: "in 3 Tagen" geht nur, wenn das ein Monatsende ist -
	// deshalb Wartung statt HU fuer den nahen Termin.
	mk(w.company, b, ScheduleInput{Art: TerminWartung, Starttermin: rel(3)})

	huE := mk(w.company, e, ScheduleInput{Art: TerminHU})
	record(huE, ErgebnisNichtBestanden, nil)
	// Gleicher Tag, gleiches Fahrzeug: UVV vor Wartung, obwohl die Wartung
	// zuerst angelegt ist.
	mk(w.company, e, ScheduleInput{Art: TerminWartung, Starttermin: rel(7)})
	mk(w.company, e, ScheduleInput{Art: TerminUVV, Starttermin: rel(7)})

	mk(w.company, x, ScheduleInput{Art: TerminUVV, Starttermin: rel(-40)})
	if _, err := NewVehicleService(env.PG).Deregister(ctx, x.ID, now, "verkauft", w.company); err != nil {
		t.Fatalf("Abmelden: %v", err)
	}

	foreign := seedCompany(t, env)
	fv := seedVehicle(t, env, foreign, "M-F 1")
	mk(foreign, fv, ScheduleInput{Art: TerminUVV, Starttermin: rel(-50)})
	return w
}

func dueSummary(list []ScheduleDueEntry) []string {
	out := make([]string, len(list))
	for i, d := range list {
		days := "-"
		if d.Standing.TageBisFaellig != nil {
			days = strconv.Itoa(*d.Standing.TageBisFaellig)
		}
		label := d.Art
		if d.Bezeichnung != "" {
			label = d.Bezeichnung
		}
		out[i] = d.Kennzeichen + " " + label + " " + d.Standing.Status + " " + days
	}
	return out
}

func TestScheduleDueListOrderAndFilter(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewScheduleService(env.PG)
	w := newDueWorld(t, env)

	list, err := svc.DueList(ctx, today(), DefaultScheduleWarnDays, w.company)
	if err != nil {
		t.Fatalf("Liste: %v", err)
	}
	// HU-Nachpruefung "in 20 Tagen" liegt am Monatsende.
	huDays := int(EndOfMonth(today().AddDate(0, 0, 20)).Sub(today()).Hours() / 24)
	want := []string{
		"M-A 1 hu gesperrt " + strconv.Itoa(huDays),
		"M-E 1 hu gesperrt -",
		"M-B 1 uvv ueberfaellig -20",
		"M-A 1 uvv ueberfaellig -5",
		"M-B 1 wartung faellig_bald 3",
		"M-C 1 wartung faellig_bald 3",
		"M-E 1 uvv faellig_bald 7",
		"M-E 1 wartung faellig_bald 7",
		"M-A 1 versicherung faellig_bald 10",
		"M-A 1 Tachograph faellig_bald 10",
	}
	if got := dueSummary(list); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("Liste:\n%s\nerwartet:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if list[0].VehicleBezeichnung != "" || list[0].ScheduleID == uuid.Nil {
		t.Fatalf("Angaben: %+v", list[0])
	}

	// Kuerzere Vorwarnzeit: die Termine in 10 Tagen fallen heraus.
	short, _ := svc.DueList(ctx, today(), 5, w.company)
	if len(short) != 6 {
		t.Fatalf("Vorwarnzeit 5: %v", dueSummary(short))
	}
	// Leerer Mandant: leere Liste, nicht nil.
	empty, err := svc.DueList(ctx, today(), 30, seedCompany(t, env))
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("leer: %v %v", empty, err)
	}
	if _, err := svc.DueList(ctx, today(), 30, " "); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("ohne Mandant: %v", err)
	}
}

// Die Liste ist der Stand der einzelnen Fahrzeuge, gefiltert - beide
// duerfen nicht auseinanderlaufen (wie TestInspectionDueListMatchesStanding).
func TestScheduleDueListMatchesStanding(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewScheduleService(env.PG)
	w := newDueWorld(t, env)
	for _, warn := range []int{0, 5, 30, 400} {
		list, err := svc.DueList(ctx, today(), warn, w.company)
		if err != nil {
			t.Fatalf("Liste: %v", err)
		}
		fromList := make([]string, 0, len(list))
		for _, d := range list {
			fromList = append(fromList, d.ScheduleID.String()+"="+d.Standing.Status)
		}
		fromStanding := make([]string, 0)
		for _, vid := range w.vehicles {
			var abgemeldet *time.Time
			if err := env.PG.QueryRow(ctx, `SELECT abgemeldet_am FROM fleet_vehicles WHERE id=$1`, vid).Scan(&abgemeldet); err != nil {
				t.Fatalf("Fahrzeug: %v", err)
			}
			if abgemeldet != nil {
				continue
			}
			scs, err := svc.ListSchedules(ctx, vid, false, today(), warn, w.company)
			if err != nil {
				t.Fatalf("Stand: %v", err)
			}
			for _, sc := range scs {
				switch sc.Standing.Status {
				case TerminstandGesperrt, TerminstandUeberfaellig, TerminstandFaelligBald:
					fromStanding = append(fromStanding, sc.ID.String()+"="+sc.Standing.Status)
				}
			}
		}
		sort.Strings(fromList)
		sort.Strings(fromStanding)
		if strings.Join(fromList, ",") != strings.Join(fromStanding, ",") {
			t.Fatalf("Vorwarnzeit %d: Liste %v, Stand %v", warn, fromList, fromStanding)
		}
	}
}
