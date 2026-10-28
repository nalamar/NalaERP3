package fleet

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Fahrtenbuch-Service gegen echtes Postgres (Backlog G.3c.2). Feste Daten
// im Maerz 2026.

func seedLogbookVehicle(t *testing.T, env *testutil.IntegrationEnv, company, kennzeichen string, fahrtenbuch bool) *Vehicle {
	t.Helper()
	v, err := NewVehicleService(env.PG).Create(context.Background(), VehicleInput{Kennzeichen: kennzeichen, Art: ArtPkw, Erstzulassung: datePtr(2026, 2, 1), Fahrtenbuch: fahrtenbuch}, company, "")
	if err != nil {
		t.Fatalf("Fahrzeug: %v", err)
	}
	return v
}

func countTrips(t *testing.T, env *testutil.IntegrationEnv, vehicleID uuid.UUID) int {
	t.Helper()
	var n int
	if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM fleet_trips WHERE vehicle_id=$1`, vehicleID).Scan(&n); err != nil {
		t.Fatalf("zaehlen: %v", err)
	}
	return n
}

func TestTripRecordListStornoAndGaps(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedCompany(t, env)
	v := seedLogbookVehicle(t, env, company, "M-FB 10", true)
	anna := seedDriver(t, env, company, "Berg", true)
	ben := seedDriver(t, env, company, "Kurz", true)
	user := testutil.SeedAuthUser(t, env, "fahrt-"+uuid.NewString()[:8]+"@example.test", "pw-123456789", "fleet")
	s := NewTripService(env.PG)

	if st, err := s.ChainState(ctx, v.ID, company); err != nil || st.KmStand != nil || len(st.Luecken) != 0 {
		t.Fatalf("leer: %+v %v", st, err)
	}
	a, err := s.RecordTrip(ctx, TripInput{VehicleID: v.ID, EmployeeID: anna, Datum: day(2026, 3, 2), KmBeginn: 1000, KmEnde: 1100,
		Fahrtart: " Dienstlich ", Ziel: " Ulm ", Zweck: "Montage", Geschaeftspartner: "Fa. X"}, user, company)
	if err != nil {
		t.Fatalf("erste Fahrt: %v", err)
	}
	if a.Fahrtart != FahrtartDienstlich || a.Ziel != "Ulm" || a.Km != 100 || a.Kennzeichen != "M-FB 10" || a.MitarbeiterName != "Max Berg" ||
		a.Status != TripActive || a.CreatedByUserID == nil || *a.CreatedByUserID != user || a.CreatedAt.IsZero() || !a.Datum.Equal(day(2026, 3, 2)) {
		t.Fatalf("erste Fahrt: %+v", a)
	}
	b, err := s.RecordTrip(ctx, TripInput{VehicleID: v.ID, EmployeeID: ben, Datum: day(2026, 3, 3), KmBeginn: 1100, KmEnde: 1130, Fahrtart: FahrtartPrivat}, "", company)
	if err != nil || b.CreatedByUserID != nil {
		t.Fatalf("zweite Fahrt: %+v %v", b, err)
	}
	c, err := s.RecordTrip(ctx, TripInput{VehicleID: v.ID, EmployeeID: anna, Datum: day(2026, 3, 3), KmBeginn: 1130, KmEnde: 1150, Fahrtart: FahrtartArbeitsweg}, "", company)
	if err != nil {
		t.Fatalf("dritte Fahrt: %v", err)
	}
	// Die Kette gilt auch im Service: Luecke und Ueberlappen abgelehnt.
	for name, tc := range map[string]struct {
		in   TripInput
		want string
	}{
		"Luecke":      {TripInput{VehicleID: v.ID, EmployeeID: anna, Datum: day(2026, 3, 4), KmBeginn: 1160, KmEnde: 1200, Fahrtart: FahrtartPrivat}, "Kilometerstand Beginn muss 1150 km sein (Ende der vorigen Fahrt)"},
		"Ueberlapp":   {TripInput{VehicleID: v.ID, EmployeeID: anna, Datum: day(2026, 3, 4), KmBeginn: 1140, KmEnde: 1200, Fahrtart: FahrtartPrivat}, "Kilometerbereich überschneidet sich mit der Fahrt vom 03.03.2026 (1130–1150 km)"},
		"Datum frueh": {TripInput{VehicleID: v.ID, EmployeeID: anna, Datum: day(2026, 3, 2), KmBeginn: 1150, KmEnde: 1200, Fahrtart: FahrtartPrivat}, "Datum darf nicht vor der vorigen Fahrt (03.03.2026) liegen"},
	} {
		if _, err := s.RecordTrip(ctx, tc.in, "", company); err == nil || err.Error() != tc.want {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if n := countTrips(t, env, v.ID); n != 3 {
		t.Fatalf("abgelehnte Fahrten geschrieben: %d", n)
	}

	// Storno: die Fahrt bleibt, ihr Bereich wird zur Luecke.
	sb, err := s.StornoTrip(ctx, b.ID, " falsches Fahrzeug ", user, company)
	if err != nil || sb.Status != TripStorniert || sb.StornoReason != "falsches Fahrzeug" || sb.StorniertAt == nil || sb.StorniertByUserID == nil || *sb.StorniertByUserID != user {
		t.Fatalf("Storno: %+v %v", sb, err)
	}
	if _, err := s.StornoTrip(ctx, b.ID, "nochmal", "", company); err == nil || err.Error() != "Fahrt ist bereits storniert" {
		t.Fatalf("zweites Storno: %v", err)
	}
	st, err := s.ChainState(ctx, v.ID, company)
	if err != nil || st.KmStand == nil || *st.KmStand != 1150 || !reflect.DeepEqual(st.Luecken, []TripGap{{VonKm: 1100, BisKm: 1130}}) {
		t.Fatalf("Stand nach Storno: %+v %v", st, err)
	}
	// Ersatz fuellt die Luecke genau; derselbe Beginn ist wieder frei.
	ersatz, err := s.RecordTrip(ctx, TripInput{VehicleID: v.ID, EmployeeID: ben, Datum: day(2026, 3, 3), KmBeginn: 1100, KmEnde: 1130, Fahrtart: FahrtartPrivat, Notiz: "Ersatz"}, "", company)
	if err != nil {
		t.Fatalf("Ersatz: %v", err)
	}
	if st, _ := s.ChainState(ctx, v.ID, company); len(st.Luecken) != 0 {
		t.Fatalf("Luecke nach Ersatz: %+v", st)
	}

	// Liste: juengste zuerst, Stornierte nur auf Wunsch, Filter.
	ids := func(list []Trip) []uuid.UUID {
		out := make([]uuid.UUID, len(list))
		for i, x := range list {
			out[i] = x.ID
		}
		return out
	}
	all, err := s.ListTrips(ctx, company, TripFilter{VehicleID: &v.ID})
	// Am selben Tag entscheidet der Kilometerstand.
	if err != nil || !reflect.DeepEqual(ids(all), []uuid.UUID{c.ID, ersatz.ID, a.ID}) {
		t.Fatalf("Liste: %v %v", ids(all), err)
	}
	if withStorno, _ := s.ListTrips(ctx, company, TripFilter{IncludeStorniert: true}); len(withStorno) != 4 {
		t.Fatalf("mit Storno: %d", len(withStorno))
	}
	if own, _ := s.ListTrips(ctx, company, TripFilter{EmployeeID: &anna}); !reflect.DeepEqual(ids(own), []uuid.UUID{c.ID, a.ID}) {
		t.Fatalf("eigene: %v", ids(own))
	}
	von, bis := day(2026, 3, 3), day(2026, 3, 3)
	if span, _ := s.ListTrips(ctx, company, TripFilter{Von: &von, Bis: &bis}); len(span) != 2 {
		t.Fatalf("Zeitraum: %d", len(span))
	}
	frueh := day(2026, 3, 2)
	if early, _ := s.ListTrips(ctx, company, TripFilter{Bis: &frueh}); len(early) != 1 || early[0].ID != a.ID {
		t.Fatalf("bis 02.03.: %v", ids(early))
	}

	// Mandantentrennung: ein anderer Mandant sieht und storniert nichts.
	other := seedCompany(t, env)
	if _, err := s.GetTrip(ctx, a.ID, other); err == nil || err.Error() != "Fahrt nicht gefunden" {
		t.Fatalf("fremd lesen: %v", err)
	}
	if _, err := s.StornoTrip(ctx, a.ID, "x", "", other); err == nil || err.Error() != "Fahrt nicht gefunden" {
		t.Fatalf("fremd stornieren: %v", err)
	}
	if x, _ := s.GetTrip(ctx, a.ID, company); x.Status != TripActive {
		t.Fatalf("fremdes Storno hat gewirkt: %+v", x)
	}
	if list, _ := s.ListTrips(ctx, other, TripFilter{}); len(list) != 0 {
		t.Fatalf("fremde Liste: %d", len(list))
	}
	if _, err := s.ChainState(ctx, v.ID, other); err == nil || err.Error() != "Fahrzeug nicht gefunden" {
		t.Fatalf("fremder Stand: %v", err)
	}
}

func TestTripRecordErrorsWithoutEffect(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedCompany(t, env)
	other := seedCompany(t, env)
	v := seedLogbookVehicle(t, env, company, "M-FB 20", true)
	noBook := seedLogbookVehicle(t, env, company, "M-FB 21", false)
	foreignVehicle := seedLogbookVehicle(t, env, other, "M-FB 22", true)
	emp := seedDriver(t, env, company, "Fahrer", true)
	inactive := seedDriver(t, env, company, "Weg", false)
	foreignEmp := seedDriver(t, env, other, "Fremd", true)
	s := NewTripService(env.PG)
	trip := func(vehicle, employee uuid.UUID, d time.Time) TripInput {
		return TripInput{VehicleID: vehicle, EmployeeID: employee, Datum: d, KmBeginn: 100, KmEnde: 120, Fahrtart: FahrtartPrivat}
	}
	for name, tc := range map[string]struct {
		in   TripInput
		want string
	}{
		"ohne Fahrtenbuch":     {trip(noBook.ID, emp, day(2026, 3, 2)), "Für dieses Fahrzeug wird kein Fahrtenbuch geführt"},
		"fremdes Fahrzeug":     {trip(foreignVehicle.ID, emp, day(2026, 3, 2)), "Fahrzeug nicht gefunden"},
		"unbekanntes Fahrzeug": {trip(uuid.New(), emp, day(2026, 3, 2)), "Fahrzeug nicht gefunden"},
		"fremder Mitarbeiter":  {trip(v.ID, foreignEmp, day(2026, 3, 2)), "Mitarbeiter nicht gefunden"},
		"inaktiv":              {trip(v.ID, inactive, day(2026, 3, 2)), "Mitarbeiter ist nicht aktiv"},
		"vor Erstzulassung":    {trip(v.ID, emp, day(2026, 1, 31)), "Datum darf nicht vor der Erstzulassung liegen"},
		"Dienstfahrt ohne Ziel": {TripInput{VehicleID: v.ID, EmployeeID: emp, Datum: day(2026, 3, 2), KmBeginn: 1, KmEnde: 2, Fahrtart: FahrtartDienstlich, Zweck: "x"},
			"Reiseziel ist bei Dienstfahrten erforderlich"},
	} {
		if _, err := s.RecordTrip(ctx, tc.in, "", company); err == nil || err.Error() != tc.want {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for _, id := range []uuid.UUID{v.ID, noBook.ID, foreignVehicle.ID} {
		if n := countTrips(t, env, id); n != 0 {
			t.Fatalf("abgelehnte Fahrt geschrieben: %d", n)
		}
	}
	if _, err := s.GetTrip(ctx, uuid.New(), company); err == nil || err.Error() != "Fahrt nicht gefunden" {
		t.Fatalf("unbekannt lesen: %v", err)
	}
	if _, err := s.StornoTrip(ctx, uuid.New(), "x", "", company); err == nil || err.Error() != "Fahrt nicht gefunden" {
		t.Fatalf("unbekannt stornieren: %v", err)
	}
}

// Abmeldung und Fahrtenbuch: nachtragen bis zum Tag der Abmeldung,
// Abmelden nicht vor der letzten Fahrt, Storno auch danach.
func TestTripsAndDeregistration(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedCompany(t, env)
	v := seedLogbookVehicle(t, env, company, "M-FB 30", true)
	emp := seedDriver(t, env, company, "Fahrer", true)
	s := NewTripService(env.PG)
	vehicles := NewVehicleService(env.PG)
	a, err := s.RecordTrip(ctx, TripInput{VehicleID: v.ID, EmployeeID: emp, Datum: day(2026, 3, 10), KmBeginn: 0, KmEnde: 50, Fahrtart: FahrtartPrivat}, "", company)
	if err != nil {
		t.Fatalf("Fahrt: %v", err)
	}
	if _, err := vehicles.Deregister(ctx, v.ID, day(2026, 3, 9), "verkauft", company); err == nil || err.Error() != "Abmeldedatum darf nicht vor der letzten Fahrt liegen" {
		t.Fatalf("Abmelden vor der Fahrt: %v", err)
	}
	// Abmelden am Tag der letzten Fahrt ist zulaessig.
	if _, err := vehicles.Deregister(ctx, v.ID, day(2026, 3, 10), "verkauft", company); err != nil {
		t.Fatalf("Abmelden: %v", err)
	}
	if _, err := s.RecordTrip(ctx, TripInput{VehicleID: v.ID, EmployeeID: emp, Datum: day(2026, 3, 11), KmBeginn: 50, KmEnde: 60, Fahrtart: FahrtartPrivat}, "", company); err == nil || err.Error() != "Datum darf nicht nach der Abmeldung liegen" {
		t.Fatalf("nach der Abmeldung: %v", err)
	}
	if _, err := s.RecordTrip(ctx, TripInput{VehicleID: v.ID, EmployeeID: emp, Datum: day(2026, 3, 10), KmBeginn: 50, KmEnde: 60, Fahrtart: FahrtartPrivat}, "", company); err != nil {
		t.Fatalf("Nachtrag am Tag der Abmeldung: %v", err)
	}
	if _, err := s.StornoTrip(ctx, a.ID, "doppelt", "", company); err != nil {
		t.Fatalf("Storno am abgemeldeten Fahrzeug: %v", err)
	}

	// Ein stornierter Eintrag haelt die Abmeldung nicht auf.
	w := seedLogbookVehicle(t, env, company, "M-FB 31", true)
	x, err := s.RecordTrip(ctx, TripInput{VehicleID: w.ID, EmployeeID: emp, Datum: day(2026, 3, 10), KmBeginn: 0, KmEnde: 50, Fahrtart: FahrtartPrivat}, "", company)
	if err != nil {
		t.Fatalf("Fahrt: %v", err)
	}
	if _, err := s.StornoTrip(ctx, x.ID, "falsch", "", company); err != nil {
		t.Fatalf("Storno: %v", err)
	}
	if _, err := vehicles.Deregister(ctx, w.ID, day(2026, 3, 5), "verkauft", company); err != nil {
		t.Fatalf("Abmelden trotz stornierter Fahrt: %v", err)
	}
}

// Gleichzeitig erfasste Fahrten mit demselben Beginn: genau eine kommt
// durch, die Kette bleibt geschlossen.
func TestTripRecordConcurrent(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedCompany(t, env)
	v := seedLogbookVehicle(t, env, company, "M-FB 40", true)
	emp := seedDriver(t, env, company, "Fahrer", true)
	s := NewTripService(env.PG)
	const n = 6
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = s.RecordTrip(ctx, TripInput{VehicleID: v.ID, EmployeeID: emp, Datum: day(2026, 3, 2), KmBeginn: 500, KmEnde: 510 + i, Fahrtart: FahrtartPrivat}, "", company)
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else if !strings.HasPrefix(err.Error(), "Kilometerbereich überschneidet sich") {
			t.Fatalf("unerwarteter Fehler: %v", err)
		}
	}
	if ok != 1 || countTrips(t, env, v.ID) != 1 {
		t.Fatalf("erfolgreich %d, geschrieben %d", ok, countTrips(t, env, v.ID))
	}
}
