package fleet

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Termine gegen echtes Postgres (Backlog G.2.2.1). Erledigungen werden
// hier noch per SQL angelegt; ihr Service folgt in G.2.2.2.

func seedVehicle(t *testing.T, env *testutil.IntegrationEnv, company, kennzeichen string) *Vehicle {
	t.Helper()
	v, err := NewVehicleService(env.PG).Create(context.Background(), VehicleInput{Kennzeichen: kennzeichen, Art: ArtTransporter}, company, "")
	if err != nil {
		t.Fatalf("Fahrzeug: %v", err)
	}
	return v
}

func seedCompletion(t *testing.T, env *testutil.IntegrationEnv, scheduleID uuid.UUID, am, ergebnis string, naechster *string, status string) {
	t.Helper()
	reason, at := "", any(nil)
	if status == CompletionStorniert {
		reason, at = "falsch", time.Now()
	}
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO fleet_vehicle_completions (id, schedule_id, erledigt_am, ergebnis, naechster_termin, status, storno_reason, storniert_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, uuid.New(), scheduleID, am, ergebnis, naechster, status, reason, at); err != nil {
		t.Fatalf("Erledigung: %v", err)
	}
}

func TestScheduleCreateDefaultsAndNormalization(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewScheduleService(env.PG)
	company := seedCompany(t, env)
	v := seedVehicle(t, env, company, "M-S 1")
	actor := testutil.SeedAuthUser(t, env, "sched-"+uuid.NewString()[:8]+"@example.test", "pw-123456789", "fleet")

	hu, err := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: " hu ", Starttermin: datePtr(2027, 5, 7), Notiz: " Plakette 05/27 "}, actor, company)
	if err != nil {
		t.Fatalf("HU: %v", err)
	}
	if hu.Art != TerminHU || hu.Starttermin == nil || !hu.Starttermin.Equal(day(2027, 5, 31)) || hu.IntervallMonate == nil || *hu.IntervallMonate != 24 || hu.Notiz != "Plakette 05/27" || !hu.Aktiv {
		t.Fatalf("HU: %+v", hu)
	}
	var createdBy *string
	if err := env.PG.QueryRow(ctx, `SELECT created_by_user_id FROM fleet_vehicle_schedules WHERE id=$1`, hu.ID).Scan(&createdBy); err != nil || createdBy == nil || *createdBy != actor {
		t.Fatalf("Urheber: %v %v", createdBy, err)
	}
	uvv, err := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminUVV, Starttermin: datePtr(2026, 11, 15)}, "", company)
	if err != nil || uvv.IntervallMonate == nil || *uvv.IntervallMonate != 12 || !uvv.Starttermin.Equal(day(2026, 11, 15)) {
		t.Fatalf("UVV: %+v %v", uvv, err)
	}
	wartung, err := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminWartung, StartKm: intPtr(30000), IntervallKm: intPtr(30000)}, "", company)
	if err != nil || wartung.IntervallMonate != nil || wartung.StartKm == nil || *wartung.StartKm != 30000 {
		t.Fatalf("Wartung ohne Vorschlag: %+v %v", wartung, err)
	}
	vers, err := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminVersicherung, IntervallMonate: intPtr(6)}, "", company)
	if err != nil || *vers.IntervallMonate != 6 {
		t.Fatalf("eigenes Intervall: %+v %v", vers, err)
	}
	tacho, err := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminSonstige, Bezeichnung: " Tachograph "}, "", company)
	if err != nil || tacho.Bezeichnung != "Tachograph" || tacho.IntervallMonate != nil {
		t.Fatalf("sonstige: %+v %v", tacho, err)
	}

	for name, err := range map[string]error{
		"zweite HU": func() error {
			_, e := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminHU}, "", company)
			return e
		}(),
		"Tacho doppelt": func() error {
			_, e := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminSonstige, Bezeichnung: "TACHOGRAPH"}, "", company)
			return e
		}(),
	} {
		if err == nil || err.Error() != "Termin dieser Art existiert bereits für das Fahrzeug" {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// Ein anderes Fahrzeug darf dieselbe Art haben.
	other := seedVehicle(t, env, company, "M-S 2")
	if _, err := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: other.ID, Art: TerminHU}, "", company); err != nil {
		t.Fatalf("HU am zweiten Fahrzeug: %v", err)
	}
}

func TestScheduleCreateErrors(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewScheduleService(env.PG)
	company := seedCompany(t, env)
	foreignCompany := seedCompany(t, env)
	v := seedVehicle(t, env, company, "M-E 1")
	foreign := seedVehicle(t, env, foreignCompany, "M-E 1")
	gone := seedVehicle(t, env, company, "M-E 2")
	if _, err := NewVehicleService(env.PG).Deregister(ctx, gone.ID, today(), "verkauft", company); err != nil {
		t.Fatalf("Abmelden: %v", err)
	}
	for _, tc := range []struct {
		name    string
		in      ScheduleInput
		company string
		want    string
	}{
		{"ohne Mandant", ScheduleInput{VehicleID: v.ID, Art: TerminHU}, " ", "Mandant erforderlich"},
		{"ohne Fahrzeug", ScheduleInput{Art: TerminHU}, company, "vehicle_id erforderlich"},
		{"Art unbekannt", ScheduleInput{VehicleID: v.ID, Art: "tuev"}, company, "Ungültige Terminart"},
		{"sonstige ohne Bezeichnung", ScheduleInput{VehicleID: v.ID, Art: TerminSonstige, Bezeichnung: "  "}, company, "Bezeichnung erforderlich"},
		{"Intervall 0", ScheduleInput{VehicleID: v.ID, Art: TerminUVV, IntervallMonate: intPtr(0)}, company, "Intervall muss größer als 0 sein"},
		{"Fahrzeug unbekannt", ScheduleInput{VehicleID: uuid.New(), Art: TerminHU}, company, "Fahrzeug nicht gefunden"},
		{"fremdes Fahrzeug", ScheduleInput{VehicleID: foreign.ID, Art: TerminHU}, company, "Fahrzeug nicht gefunden"},
		{"abgemeldet", ScheduleInput{VehicleID: gone.ID, Art: TerminHU}, company, "Fahrzeug ist abgemeldet"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.CreateSchedule(ctx, tc.in, "", tc.company); err == nil || err.Error() != tc.want {
				t.Fatalf("erwartet %q, bekommen %v", tc.want, err)
			}
		})
	}
	var n int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM fleet_vehicle_schedules WHERE vehicle_id = ANY($1)`, []uuid.UUID{v.ID, foreign.ID, gone.ID}).Scan(&n); err != nil || n != 0 {
		t.Fatalf("nach Fehlern %d Termine (%v)", n, err)
	}
}

func TestScheduleUpdateAndStanding(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewScheduleService(env.PG)
	company := seedCompany(t, env)
	foreignCompany := seedCompany(t, env)
	v := seedVehicle(t, env, company, "M-U 1")
	now := today()
	soon := now.AddDate(0, 0, 10)
	far := now.AddDate(1, 0, 0)

	uvv, _ := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminUVV, Starttermin: &soon}, "", company)
	hu, _ := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminHU, Starttermin: &far}, "", company)
	leasing, _ := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminLeasing}, "", company)
	tacho, _ := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminSonstige, Bezeichnung: "Tachograph", Starttermin: &soon}, "", company)

	// Liste nach Art (HU, UVV, ..., sonstige) mit Stand.
	list, err := svc.ListSchedules(ctx, v.ID, false, now, DefaultScheduleWarnDays, company)
	if err != nil {
		t.Fatalf("Liste: %v", err)
	}
	got := make([]string, len(list))
	for i, s := range list {
		got[i] = s.Art + "=" + s.Standing.Status
	}
	if strings.Join(got, ",") != "hu=ok,uvv=faellig_bald,leasing=ohne_termin,sonstige=faellig_bald" {
		t.Fatalf("Liste: %v", got)
	}

	// Erledigungen: die juengste nicht stornierte loest den Starttermin ab;
	// eine UVV mit Maengeln sperrt.
	next := far.Format("2006-01-02")
	seedCompletion(t, env, uvv.ID, now.AddDate(0, 0, -5).Format("2006-01-02"), ErgebnisBestanden, &next, CompletionActive)
	seedCompletion(t, env, uvv.ID, now.AddDate(0, 0, -1).Format("2006-01-02"), ErgebnisNichtBestanden, nil, CompletionStorniert)
	seedCompletion(t, env, hu.ID, now.AddDate(0, 0, -1).Format("2006-01-02"), ErgebnisMaengel, nil, CompletionActive)
	list, _ = svc.ListSchedules(ctx, v.ID, false, now, DefaultScheduleWarnDays, company)
	if list[0].Standing.Status != TerminstandGesperrt || list[1].Standing.Status != TerminstandOK || list[1].Standing.FaelligAm == nil || !list[1].Standing.FaelligAm.Equal(dateOnly(far)) {
		t.Fatalf("mit Erledigungen: %+v / %+v", list[0].Standing, list[1].Standing)
	}

	// Aendern ersetzt alles; HU-Starttermin zum Monatsende; stilllegen.
	upd, err := svc.UpdateSchedule(ctx, hu.ID, ScheduleUpdate{Starttermin: datePtr(2030, 2, 3), Notiz: " neu ", Aktiv: true}, company)
	if err != nil || !upd.Starttermin.Equal(day(2030, 2, 28)) || upd.IntervallMonate != nil || upd.Notiz != "neu" {
		t.Fatalf("Aendern: %+v %v", upd, err)
	}
	if _, err := svc.UpdateSchedule(ctx, tacho.ID, ScheduleUpdate{Aktiv: false}, company); err != nil {
		t.Fatalf("stilllegen: %v", err)
	}
	if l, _ := svc.ListSchedules(ctx, v.ID, false, now, 30, company); len(l) != 3 {
		t.Fatalf("aktive: %d", len(l))
	}
	if l, _ := svc.ListSchedules(ctx, v.ID, true, now, 30, company); len(l) != 4 || l[3].Aktiv {
		t.Fatalf("mit stillgelegten: %+v", l)
	}
	if again, err := svc.UpdateSchedule(ctx, tacho.ID, ScheduleUpdate{Aktiv: true}, company); err != nil || !again.Aktiv {
		t.Fatalf("wieder aktiv: %+v %v", again, err)
	}
	if _, err := svc.UpdateSchedule(ctx, leasing.ID, ScheduleUpdate{IntervallKm: intPtr(0), Aktiv: true}, company); err == nil || err.Error() != "Intervall muss größer als 0 sein" {
		t.Fatalf("Intervall 0: %v", err)
	}

	// Mandant und Abmeldung.
	if _, err := svc.UpdateSchedule(ctx, hu.ID, ScheduleUpdate{Aktiv: true}, foreignCompany); err == nil || err.Error() != "Termin nicht gefunden" {
		t.Fatalf("fremder Mandant: %v", err)
	}
	if _, err := svc.GetSchedule(ctx, hu.ID, foreignCompany); err == nil || err.Error() != "Termin nicht gefunden" {
		t.Fatalf("fremder Mandant lesen: %v", err)
	}
	if _, err := svc.ListSchedules(ctx, v.ID, true, now, 30, foreignCompany); err == nil || err.Error() != "Fahrzeug nicht gefunden" {
		t.Fatalf("fremde Liste: %v", err)
	}
	if _, err := NewVehicleService(env.PG).Deregister(ctx, v.ID, now, "verkauft", company); err != nil {
		t.Fatalf("Abmelden: %v", err)
	}
	if _, err := svc.UpdateSchedule(ctx, hu.ID, ScheduleUpdate{Aktiv: true, Notiz: "x"}, company); err == nil || err.Error() != "Fahrzeug ist abgemeldet" {
		t.Fatalf("Aendern am abgemeldeten: %v", err)
	}
	if l, err := svc.ListSchedules(ctx, v.ID, false, now, 30, company); err != nil || len(l) != 4 {
		t.Fatalf("abgemeldetes Fahrzeug lesbar: %d %v", len(l), err)
	}
	if got, _ := svc.GetSchedule(ctx, hu.ID, company); got.Notiz != "neu" {
		t.Fatalf("abgelehnte Aenderung gespeichert: %+v", got)
	}
}
