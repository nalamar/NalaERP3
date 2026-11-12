package production

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Hinweise der Montagetermine und Wochenuebersicht je Team (Backlog
// H.3.6c): Team doppelt, Abwesenheit im Team, Fahrzeug gesperrt, vor
// Fertigstellung, nach Liefertermin - jeweils mit Faellen ohne Hinweis.
func TestAssemblyHintsAndWeeks(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	p := seedPosition(t, env)
	other := seedPosition(t, env)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := env.PG.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	newOrder := func(liefertermin string) uuid.UUID {
		t.Helper()
		in := OrderInput{ProjectID: p.project, ElevationID: p.elevation, SingleElevationID: p.variant, Stueckzahl: 1}
		if liefertermin != "" {
			in.Liefertermin = dayPtr(liefertermin)
		}
		o, err := svc.Create(ctx, in, "", p.company)
		if err != nil {
			t.Fatal(err)
		}
		return o.ID
	}
	oLate, oFree, oOps := newOrder("2030-03-01"), newOrder(""), newOrder("2030-03-26")

	// Arbeitsgaenge: oLate offen in der Woche des Montagebeginns; oOps
	// offen davor und erledigt danach.
	wc := uuid.New()
	exec(`INSERT INTO production_work_centers (id, company_id, name, stunden_je_tag) VALUES ($1,$2,'Montagevorbereitung',8)`, wc, p.company)
	insOp := `INSERT INTO production_order_operations (id, order_id, pos, name, work_center_id, geplante_woche) VALUES ($1,$2,$3,'Schritt',$4,$5)`
	exec(insOp, uuid.New(), oLate, 1, wc, "2030-03-04")
	exec(insOp, uuid.New(), oOps, 1, wc, "2030-03-18")
	doneOp := uuid.New()
	exec(insOp, doneOp, oOps, 2, wc, "2030-04-01")
	exec(`INSERT INTO production_operation_feedback (id, company_id, operation_id) VALUES ($1,$2,$3)`, uuid.New(), p.company, doneOp)

	// Teams, Mitglieder, Abwesenheiten (ohne Namen in den Hinweisen).
	teamA, teamB, teamC := uuid.New(), uuid.New(), uuid.New()
	exec(`INSERT INTO hr_teams (id, name, company_id) VALUES ($1,'Alpha',$4), ($2,'beta',$4), ($3,'Gamma',$4), ($5,'Fremd',$6)`,
		teamA, teamB, teamC, p.company, uuid.New(), other.company)
	emp := func(team uuid.UUID, active bool) uuid.UUID {
		id := uuid.New()
		exec(`INSERT INTO hr_employees (id, first_name, last_name, company_id, team_id, active) VALUES ($1,'Erika','Muster',$2,$3,$4)`, id, p.company, team, active)
		return id
	}
	empA, empInactive, empB := emp(teamA, true), emp(teamA, false), emp(teamB, true)
	insAbs := `INSERT INTO hr_absences (id, employee_id, typ, start_date, end_date, gemeldet_von, status, storniert_at, storno_reason) VALUES ($1,$2,'sick',$3,$4,'personal',$5,$6,$7)`
	exec(insAbs, uuid.New(), empA, "2030-03-06", "2030-03-08", "approved", nil, nil)
	exec(insAbs, uuid.New(), empA, "2030-03-11", "2030-03-11", "storniert", time.Now(), "Irrtum")
	exec(insAbs, uuid.New(), empInactive, "2030-03-12", "2030-03-12", "approved", nil, nil)
	exec(insAbs, uuid.New(), empB, "2030-03-11", "2030-03-12", "approved", nil, nil)

	// Fahrzeuge: gesperrt (juengste HU mit Maengeln); frei (Maengel, danach
	// bestanden); frei (UVV nicht bestanden storniert, inaktive HU mit
	// Maengeln, Wartung); gesperrt (UVV nicht bestanden).
	vLocked, vPassed, vOther, vUVV := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, v := range []uuid.UUID{vLocked, vPassed, vOther, vUVV} {
		exec(`INSERT INTO fleet_vehicles (id, company_id, kennzeichen, art) VALUES ($1,$2,$3,'transporter')`, v, p.company, "MH-"+uuid.NewString()[:6])
	}
	insSched := `INSERT INTO fleet_vehicle_schedules (id, vehicle_id, art, aktiv) VALUES ($1,$2,$3,$4)`
	insCompl := `INSERT INTO fleet_vehicle_completions (id, schedule_id, erledigt_am, ergebnis, status, storno_reason, storniert_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	sched := func(v uuid.UUID, art string, aktiv bool, completions ...[3]string) {
		id := uuid.New()
		exec(insSched, id, v, art, aktiv)
		for _, c := range completions {
			if c[2] == "storniert" {
				exec(insCompl, uuid.New(), id, c[0], c[1], "storniert", "Irrtum", time.Now())
			} else {
				exec(insCompl, uuid.New(), id, c[0], c[1], "active", "", nil)
			}
		}
	}
	sched(vLocked, "hu", true, [3]string{"2026-01-10", "bestanden"}, [3]string{"2026-06-10", "maengel"})
	sched(vPassed, "hu", true, [3]string{"2026-06-10", "maengel"}, [3]string{"2026-06-20", "bestanden"})
	sched(vOther, "uvv", true, [3]string{"2026-05-10", "bestanden"}, [3]string{"2026-06-10", "nicht_bestanden", "storniert"})
	sched(vOther, "hu", false, [3]string{"2026-06-10", "maengel"})
	sched(vOther, "wartung", true, [3]string{"2026-06-10", "erledigt"})
	sched(vUVV, "uvv", true, [3]string{"2026-06-10", "nicht_bestanden"})

	create := func(order uuid.UUID, von, bis string, team, vehicle *uuid.UUID) *Appointment {
		t.Helper()
		a, err := svc.CreateAppointment(ctx, order, AppointmentInput{Von: dayPtr(von), Bis: dayPtr(bis), TeamID: team, VehicleID: vehicle}, "", p.company)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	all := create(oLate, "2030-03-04", "2030-03-06", &teamA, &vLocked)
	// Anlegen liefert die Hinweise; noch ohne zweiten Termin des Teams.
	if want := []string{HintTeamAbwesend, HintFahrzeugGesperrt, HintVorFertigstellung, HintNachLiefertermin}; !reflect.DeepEqual(all.Hinweise, want) {
		t.Fatalf("angelegt: %v, erwartet %v", all.Hinweise, want)
	}
	overlap := create(oFree, "2030-03-06", "2030-03-10", &teamA, &vPassed)
	none := create(oFree, "2030-03-11", "2030-03-12", &teamA, &vOther)
	opsBefore := create(oOps, "2030-03-25", "2030-03-26", nil, nil)
	opsSame := create(oOps, "2030-03-18", "2030-03-27", nil, nil)
	past := create(oFree, "2020-01-06", "2020-01-07", nil, &vUVV)
	beta := create(oFree, "2030-03-20", "2030-03-20", &teamB, nil)
	cancelled := create(oFree, "2030-03-20", "2030-03-20", &teamB, &vLocked)
	if _, err := svc.CancelAppointment(ctx, oFree, cancelled.ID, "verlegt", "", p.company); err != nil {
		t.Fatal(err)
	}
	// Termin eines anderen Mandanten (fuer die Uebersicht).
	oOther, err := svc.Create(ctx, OrderInput{ProjectID: other.project, ElevationID: other.elevation, SingleElevationID: other.variant, Stueckzahl: 1}, "", other.company)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateAppointment(ctx, oOther.ID, AppointmentInput{Von: dayPtr("2030-03-11"), Bis: dayPtr("2030-03-12")}, "", other.company); err != nil {
		t.Fatal(err)
	}

	hints := map[uuid.UUID][]string{}
	for _, o := range []uuid.UUID{oLate, oFree, oOps} {
		list, err := svc.ListAppointments(ctx, o, p.company)
		if err != nil {
			t.Fatal(err)
		}
		for _, x := range list {
			hints[x.ID] = x.Hinweise
		}
	}
	for _, c := range []struct {
		name string
		id   uuid.UUID
		want []string
	}{
		{"alle Hinweise", all.ID, []string{HintTeamDoppelt, HintTeamAbwesend, HintFahrzeugGesperrt, HintVorFertigstellung, HintNachLiefertermin}},
		{"Ueberschneidung, Fahrzeug wieder bestanden", overlap.ID, []string{HintTeamDoppelt, HintTeamAbwesend}},
		{"angrenzend, Abwesenheit storniert/inaktiv/anderes Team, Fahrzeug frei", none.ID, []string{}},
		{"erledigter Arbeitsgang spaeter, Ende am Liefertermin", opsBefore.ID, []string{}},
		{"offener Arbeitsgang in der Woche, nach Liefertermin", opsSame.ID, []string{HintVorFertigstellung, HintNachLiefertermin}},
		{"Beginn in der Vergangenheit, UVV nicht bestanden", past.ID, []string{HintFahrzeugGesperrt, HintVorFertigstellung}},
		{"stornierter Termin zaehlt nicht doppelt", beta.ID, []string{}},
		{"stornierter Termin ohne Hinweise", cancelled.ID, []string{}},
	} {
		if got, ok := hints[c.id]; !ok || got == nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %#v, erwartet %v", c.name, got, c.want)
		}
	}
	upd, err := svc.UpdateAppointment(ctx, oFree, overlap.ID, AppointmentInput{Von: dayPtr("2030-03-09"), Bis: dayPtr("2030-03-10"), TeamID: &teamA}, "", p.company)
	if err != nil || !reflect.DeepEqual(upd.Hinweise, []string{}) {
		t.Fatalf("geaendert ohne Ueberschneidung: %+v %v", upd, err)
	}

	// Wochenuebersicht.
	if _, err := svc.AssemblyWeeks(ctx, day("2030-03-10"), day("2030-03-01"), p.company); err == nil {
		t.Fatal("bis vor von angenommen")
	}
	if _, err := svc.AssemblyWeeks(ctx, day("2030-01-07"), day("2030-07-08"), p.company); err == nil {
		t.Fatal("27 Wochen angenommen")
	}
	w, err := svc.AssemblyWeeks(ctx, day("2030-03-12"), day("2030-03-20"), p.company)
	if err != nil || w.Von.Format("2006-01-02") != "2030-03-11" || w.Bis.Format("2006-01-02") != "2030-03-24" || len(w.Teams) != 4 {
		t.Fatalf("Uebersicht: %+v %v", w, err)
	}
	ids := func(r AssemblyTeamRow) []uuid.UUID {
		out := []uuid.UUID{}
		for _, x := range r.Termine {
			out = append(out, x.ID)
		}
		return out
	}
	for i, c := range []struct {
		team *uuid.UUID
		name string
		want []uuid.UUID
	}{
		{&teamA, "Alpha", []uuid.UUID{none.ID}},
		{&teamB, "beta", []uuid.UUID{beta.ID}},
		{&teamC, "Gamma", []uuid.UUID{}},
		{nil, "", []uuid.UUID{opsSame.ID}},
	} {
		r := w.Teams[i]
		if (r.TeamID == nil) != (c.team == nil) || (c.team != nil && *r.TeamID != *c.team) || r.TeamName != c.name || !reflect.DeepEqual(ids(r), c.want) {
			t.Errorf("Zeile %d: %v %q %v, erwartet %q %v", i, r.TeamID, r.TeamName, ids(r), c.name, c.want)
		}
	}
	if x := w.Teams[3].Termine[0]; x.OrderNummer == "" || x.ProjectNummer != "P-1" || x.ProjectName != "Testprojekt" ||
		!reflect.DeepEqual(x.Hinweise, []string{HintVorFertigstellung, HintNachLiefertermin}) {
		t.Fatalf("Termin in der Uebersicht: %+v", x)
	}
	// Ohne Termine ohne Team keine Zeile "ohne Team".
	if w, err = svc.AssemblyWeeks(ctx, day("2030-03-11"), day("2030-03-11"), p.company); err != nil || len(w.Teams) != 3 {
		t.Fatalf("ohne Termine ohne Team: %+v %v", w, err)
	}
	if w, err = svc.AssemblyWeeks(ctx, day("2030-03-11"), day("2030-03-11"), other.company); err != nil || len(w.Teams) != 2 || w.Teams[1].TeamID != nil {
		t.Fatalf("anderer Mandant: %+v %v", w, err)
	}
}
