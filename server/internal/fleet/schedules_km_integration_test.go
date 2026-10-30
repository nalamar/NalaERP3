package fleet

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Faelligkeit nach km im Stand und in der Faelligkeitsliste (Backlog
// G.3e.1): der km-Stand kommt aus nicht stornierten Fahrten und
// Erledigungen desselben Fahrzeugs.

func TestScheduleKmDueFromTripsAndCompletions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedCompany(t, env)
	v := seedLogbookVehicle(t, env, company, "M-KM 1", true)
	other := seedLogbookVehicle(t, env, company, "M-KM 2", true)
	driver := seedDriver(t, env, company, "Berg", true)
	svc := NewScheduleService(env.PG)
	trips := NewTripService(env.PG)

	wartung, err := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminWartung, StartKm: kmp(5000)}, "", company)
	if err != nil {
		t.Fatalf("Wartung: %v", err)
	}
	uvv, err := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminUVV, Starttermin: datePtr(2027, 6, 30)}, "", company)
	if err != nil {
		t.Fatalf("UVV: %v", err)
	}
	otherWartung, err := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: other.ID, Art: TerminWartung, StartKm: kmp(100)}, "", company)
	if err != nil {
		t.Fatalf("Wartung anderes Fahrzeug: %v", err)
	}

	standing := func(vehicleID, scheduleID uuid.UUID) ScheduleStanding {
		t.Helper()
		l, err := svc.ListSchedules(ctx, vehicleID, false, today(), DefaultScheduleWarnDays, company)
		if err != nil {
			t.Fatalf("Stand: %v", err)
		}
		for _, sc := range l {
			if sc.ID == scheduleID {
				return sc.Standing
			}
		}
		t.Fatalf("Termin %s fehlt", scheduleID)
		return ScheduleStanding{}
	}
	inDue := func(scheduleID uuid.UUID) *ScheduleStanding {
		t.Helper()
		l, err := svc.DueList(ctx, today(), DefaultScheduleWarnDays, company)
		if err != nil {
			t.Fatalf("Liste: %v", err)
		}
		for _, d := range l {
			if d.ScheduleID == scheduleID {
				st := d.Standing
				return &st
			}
		}
		return nil
	}
	expect := func(step, status string, km, rest *int) {
		t.Helper()
		st := standing(v.ID, wartung.ID)
		if st.Status != status || !sameInt(st.KmStand, km) || !sameInt(st.KmBisFaellig, rest) {
			t.Fatalf("%s: %+v (km %v, rest %v)", step, st, deref(st.KmStand), deref(st.KmBisFaellig))
		}
		due := inDue(wartung.ID)
		wantDue := status == TerminstandFaelligBald || status == TerminstandUeberfaellig
		if (due != nil) != wantDue || (due != nil && (due.Status != st.Status || !sameInt(due.KmBisFaellig, st.KmBisFaellig) || !sameInt(due.KmStand, st.KmStand))) {
			t.Fatalf("%s: Liste %+v, Stand %+v", step, due, st)
		}
	}
	trip := func(datumTag, von, bis int) *Trip {
		t.Helper()
		tr, err := trips.RecordTrip(ctx, TripInput{VehicleID: v.ID, EmployeeID: driver, Datum: day(2026, 3, datumTag), KmBeginn: von, KmEnde: bis, Fahrtart: FahrtartPrivat}, "", company)
		if err != nil {
			t.Fatalf("Fahrt %d-%d: %v", von, bis, err)
		}
		return tr
	}

	expect("ohne km-Stand", TerminstandOhneTermin, nil, nil)
	trip(2, 1000, 3999)
	expect("knapp vor der Vorwarnung", TerminstandOK, kmp(3999), kmp(1001))
	trip(3, 3999, 4000)
	expect("Vorwarnung", TerminstandFaelligBald, kmp(4000), kmp(1000))
	last := trip(4, 4000, 5001)
	expect("ueberschritten", TerminstandUeberfaellig, kmp(5001), kmp(-1))
	if _, err := trips.StornoTrip(ctx, last.ID, "falsch erfasst", "", company); err != nil {
		t.Fatalf("Storno Fahrt: %v", err)
	}
	expect("Fahrt storniert", TerminstandFaelligBald, kmp(4000), kmp(1000))

	// Eine Erledigung eines ANDEREN Termins mit hoeherem km-Stand zaehlt.
	c, err := svc.RecordCompletion(ctx, CompletionInput{ScheduleID: uvv.ID, ErledigtAm: day(2026, 3, 10), Ergebnis: ErgebnisBestanden, KmStand: kmp(6000)}, "", company)
	if err != nil {
		t.Fatalf("UVV erledigt: %v", err)
	}
	expect("km aus Erledigung", TerminstandUeberfaellig, kmp(6000), kmp(-1000))
	if st := standing(v.ID, uvv.ID); st.Status != TerminstandOK || !sameInt(st.KmStand, kmp(6000)) || st.KmBisFaellig != nil {
		t.Fatalf("UVV ohne Folge-km: %+v", st)
	}
	if _, err := svc.StornoCompletion(ctx, c.ID, "falscher km-Stand", "", company); err != nil {
		t.Fatalf("Storno Erledigung: %v", err)
	}
	expect("Erledigung storniert", TerminstandFaelligBald, kmp(4000), kmp(1000))

	// Die Wartung selbst erledigt: Folge-km aus dem Intervall gilt.
	if _, err := svc.RecordCompletion(ctx, CompletionInput{ScheduleID: wartung.ID, ErledigtAm: day(2026, 3, 5), Ergebnis: ErgebnisErledigt, KmStand: kmp(4000), NaechsterKm: kmp(20000)}, "", company); err != nil {
		t.Fatalf("Wartung erledigt: %v", err)
	}
	expect("nach der Wartung", TerminstandOK, kmp(4000), kmp(16000))

	// Das andere Fahrzeug hat keinen km-Stand - nichts von M-KM 1 faerbt ab.
	if st := standing(other.ID, otherWartung.ID); st.Status != TerminstandOhneTermin || st.KmStand != nil || st.KmBisFaellig != nil {
		t.Fatalf("anderes Fahrzeug: %+v", st)
	}
	if inDue(otherWartung.ID) != nil {
		t.Fatal("anderes Fahrzeug in der Liste")
	}
}

func sameInt(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
