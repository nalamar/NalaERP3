package production

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Montagetermine am Fertigungsauftrag (Backlog H.3.6b): anlegen mit
// Projekt vom Auftrag, Bezuege des Mandanten, aendern, stornieren,
// Liste, Protokoll.
func TestAssemblyAppointments(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	p := seedPosition(t, env)
	other := seedPosition(t, env)
	actor := testutil.SeedAuthUser(t, env, "mo-"+uuid.NewString()[:8]+"@example.com", "Passwort-123!", "production")
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := env.PG.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	newOrder := func() uuid.UUID {
		t.Helper()
		o, err := svc.Create(ctx, OrderInput{ProjectID: p.project, ElevationID: p.elevation, SingleElevationID: p.variant, Stueckzahl: 1}, "", p.company)
		if err != nil {
			t.Fatal(err)
		}
		return o.ID
	}
	order, order2 := newOrder(), newOrder()
	team, vehicle, offVehicle, foreignTeam, foreignVehicle := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	exec(`INSERT INTO hr_teams (id, name, company_id) VALUES ($1,'Montage Nord',$2), ($3,'Fremd',$4)`, team, p.company, foreignTeam, other.company)
	insVehicle := `INSERT INTO fleet_vehicles (id, company_id, kennzeichen, art, abgemeldet_am, abgemeldet_grund) VALUES ($1,$2,$3,'transporter',$4,$5)`
	exec(insVehicle, vehicle, p.company, "MO-"+uuid.NewString()[:6], nil, "")
	exec(insVehicle, offVehicle, p.company, "MO-"+uuid.NewString()[:6], "2026-12-08", "verkauft")
	exec(insVehicle, foreignVehicle, other.company, "MO-"+uuid.NewString()[:6], nil, "")
	in := func(von, bis string, f func(*AppointmentInput)) AppointmentInput {
		x := AppointmentInput{Von: dayPtr(von), Bis: dayPtr(bis), TeamID: &team, VehicleID: &vehicle, Baustelle: " Musterstr. 1 ", Notiz: "Kran"}
		if f != nil {
			f(&x)
		}
		return x
	}
	countRows := func() int {
		t.Helper()
		var n int
		if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM production_assembly_appointments WHERE company_id=$1`, p.company).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// Fehlerfaelle ohne Wirkung.
	for _, c := range []struct {
		name    string
		orderID uuid.UUID
		in      AppointmentInput
		company string
		want    error
	}{
		{"unbekannter Auftrag", uuid.New(), in("2026-12-07", "2026-12-09", nil), p.company, ErrNotFound},
		{"anderer Mandant", order, in("2026-12-07", "2026-12-09", nil), other.company, ErrNotFound},
		{"bis vor von", order, in("2026-12-07", "2026-12-06", nil), p.company, errMontageZeitraum},
		{"Team anderer Mandant", order, in("2026-12-07", "2026-12-09", func(x *AppointmentInput) { x.TeamID = &foreignTeam }), p.company, errAssemblyTeamNotFound},
		{"Fahrzeug anderer Mandant", order, in("2026-12-07", "2026-12-09", func(x *AppointmentInput) { x.VehicleID = &foreignVehicle }), p.company, errAssemblyVehicleMissing},
		{"Fahrzeug bis Montageende abgemeldet", order, in("2026-12-07", "2026-12-08", func(x *AppointmentInput) { x.VehicleID = &offVehicle }), p.company, errAssemblyVehicleOff},
	} {
		if _, err := svc.CreateAppointment(ctx, c.orderID, c.in, actor, c.company); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, erwartet %v", c.name, err, c.want)
		}
	}
	if n := countRows(); n != 0 {
		t.Fatalf("nach Fehlern %d Termine", n)
	}

	a1, err := svc.CreateAppointment(ctx, order, in("2026-12-07", "2026-12-09", nil), actor, p.company)
	if err != nil || a1.ProjectID != p.project || a1.OrderID == nil || *a1.OrderID != order || a1.TeamName != "Montage Nord" ||
		a1.Kennzeichen == "" || a1.Baustelle != "Musterstr. 1" || a1.Notiz != "Kran" || a1.Hinweise == nil || a1.StorniertAt != nil ||
		a1.Von.Format("2006-01-02") != "2026-12-07" || a1.Bis.Format("2006-01-02") != "2026-12-09" {
		t.Fatalf("angelegt: %+v %v", a1, err)
	}
	// Abgemeldet erst nach dem Montageende: erlaubt.
	a2, err := svc.CreateAppointment(ctx, order, in("2026-12-01", "2026-12-07", func(x *AppointmentInput) { x.VehicleID, x.TeamID = &offVehicle, nil }), actor, p.company)
	if err != nil || a2.TeamID != nil || a2.TeamName != "" {
		t.Fatalf("vor der Abmeldung: %+v %v", a2, err)
	}
	var by *string
	if err := env.PG.QueryRow(ctx, `SELECT created_by_user_id FROM production_assembly_appointments WHERE id=$1`, a1.ID).Scan(&by); err != nil || by == nil || *by != actor {
		t.Fatalf("angelegt von: %v %v", by, err)
	}
	list, err := svc.ListAppointments(ctx, order, p.company)
	if err != nil || len(list) != 2 || list[0].ID != a2.ID || list[1].ID != a1.ID {
		t.Fatalf("Liste: %+v %v", list, err)
	}
	if list, err = svc.ListAppointments(ctx, order2, p.company); err != nil || list == nil || len(list) != 0 {
		t.Fatalf("Liste ohne Termine: %#v %v", list, err)
	}
	if _, err := svc.ListAppointments(ctx, order, other.company); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Liste anderer Mandant: %v", err)
	}

	// Aendern.
	if _, err := svc.UpdateAppointment(ctx, order, a1.ID, in("2026-12-07", "2026-12-01", nil), actor, p.company); !errors.Is(err, errMontageZeitraum) {
		t.Fatalf("aendern mit Fehler: %v", err)
	}
	if _, err := svc.UpdateAppointment(ctx, order2, a1.ID, in("2026-12-07", "2026-12-10", nil), actor, p.company); !errors.Is(err, ErrAppointmentNotFound) {
		t.Fatalf("aendern ueber anderen Auftrag: %v", err)
	}
	if _, err := svc.UpdateAppointment(ctx, order, uuid.New(), in("2026-12-07", "2026-12-10", nil), actor, p.company); !errors.Is(err, ErrAppointmentNotFound) {
		t.Fatalf("aendern unbekannt: %v", err)
	}
	if _, err := svc.UpdateAppointment(ctx, order, a1.ID, in("2026-12-07", "2026-12-10", nil), actor, other.company); !errors.Is(err, ErrNotFound) {
		t.Fatalf("aendern anderer Mandant: %v", err)
	}
	x, err := svc.UpdateAppointment(ctx, order, a1.ID, in("2026-12-08", "2026-12-10", func(x *AppointmentInput) { x.TeamID, x.Baustelle = nil, "" }), actor, p.company)
	if err != nil || x.TeamID != nil || x.Baustelle != "" || x.Von.Format("2006-01-02") != "2026-12-08" || x.Bis.Format("2006-01-02") != "2026-12-10" || x.ProjectID != p.project {
		t.Fatalf("geaendert: %+v %v", x, err)
	}

	// Stornieren.
	if _, err := svc.CancelAppointment(ctx, order, a1.ID, "  ", actor, p.company); err == nil {
		t.Fatal("Storno ohne Begruendung angenommen")
	}
	if _, err := svc.CancelAppointment(ctx, order, a1.ID, "x", actor, other.company); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Storno anderer Mandant: %v", err)
	}
	if x, err = svc.CancelAppointment(ctx, order, a1.ID, " verschoben ", actor, p.company); err != nil || x.StorniertAt == nil || x.StornoReason != "verschoben" {
		t.Fatalf("storniert: %+v %v", x, err)
	}
	if _, err := svc.CancelAppointment(ctx, order, a1.ID, "nochmal", actor, p.company); !errors.Is(err, errAppointmentStorniert) {
		t.Fatalf("doppelt storniert: %v", err)
	}
	if _, err := svc.UpdateAppointment(ctx, order, a1.ID, in("2026-12-07", "2026-12-10", nil), actor, p.company); !errors.Is(err, errAppointmentCancelled) {
		t.Fatalf("storniert aendern: %v", err)
	}
	var stornoBy *string
	if err := env.PG.QueryRow(ctx, `SELECT storniert_by_user_id FROM production_assembly_appointments WHERE id=$1`, a1.ID).Scan(&stornoBy); err != nil || stornoBy == nil || *stornoBy != actor {
		t.Fatalf("storniert von: %v %v", stornoBy, err)
	}

	// Stornierter Auftrag: kein neuer Termin, vorhandene lassen sich
	// stornieren.
	if _, err := svc.Cancel(ctx, order2, "doppelt", "", p.company); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateAppointment(ctx, order2, in("2026-12-07", "2026-12-07", nil), actor, p.company); !errors.Is(err, errMontageStatus) {
		t.Fatalf("stornierter Auftrag: %v", err)
	}
	if _, err := svc.Cancel(ctx, order, "Kunde abgesprungen", "", p.company); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateAppointment(ctx, order, a2.ID, in("2026-12-01", "2026-12-02", nil), actor, p.company); !errors.Is(err, errMontageStatus) {
		t.Fatalf("aendern bei storniertem Auftrag: %v", err)
	}
	if _, err := svc.CancelAppointment(ctx, order, a2.ID, "Auftrag storniert", actor, p.company); err != nil {
		t.Fatalf("Storno bei storniertem Auftrag: %v", err)
	}

	var created, updated, cancelled int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FILTER (WHERE action='assembly_created'), count(*) FILTER (WHERE action='assembly_updated'),
		count(*) FILTER (WHERE action='assembly_cancelled') FROM entity_change_log WHERE entity_type='production_order' AND entity_id=$1`,
		order.String()).Scan(&created, &updated, &cancelled); err != nil || created != 2 || updated != 1 || cancelled != 2 {
		t.Fatalf("Protokoll: %d/%d/%d %v", created, updated, cancelled, err)
	}
}
