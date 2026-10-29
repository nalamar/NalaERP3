package fleet

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Fahrzeugkosten gegen echtes Postgres (Backlog G.3d.1). Feste Daten im
// Maerz 2026.

func countCosts(t *testing.T, env *testutil.IntegrationEnv, vehicleIDs ...uuid.UUID) int {
	t.Helper()
	var n int
	if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM fleet_costs WHERE vehicle_id = ANY($1)`, vehicleIDs).Scan(&n); err != nil {
		t.Fatalf("zaehlen: %v", err)
	}
	return n
}

func costAmounts(list []Cost) string {
	out := make([]string, len(list))
	for i, c := range list {
		out[i] = c.Kostenart + ":" + strconv.FormatFloat(c.BetragNetto, 'f', -1, 64)
	}
	return strings.Join(out, ",")
}

func TestCostRecordListSummaryStorno(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedCompany(t, env)
	ccVehicle := seedCostCenter(t, env, company)
	ccOther := seedCostCenter(t, env, company)
	vs := NewVehicleService(env.PG)
	v, err := vs.Create(ctx, VehicleInput{Kennzeichen: "M-K 1", Art: ArtPkw, KostenstelleID: &ccVehicle}, company, "")
	if err != nil {
		t.Fatal(err)
	}
	w := seedLogbookVehicle(t, env, company, "M-K 2", false)
	user := testutil.SeedAuthUser(t, env, "kosten-"+uuid.NewString()[:8]+"@example.test", "pw-123456789", "fleet")
	s := NewCostService(env.PG)

	// Ohne Kostenstelle die des Fahrzeugs, festgehalten.
	a, err := s.RecordCost(ctx, CostInput{VehicleID: v.ID, Datum: day(2026, 3, 2), Kostenart: " Kraftstoff ", BetragNetto: 80.12,
		Lieferant: " Tanke ", Belegnummer: "B-1", KmStand: intPtr(12000), Liter: floatPtr(45.5), Notiz: " voll "}, user, company)
	if err != nil {
		t.Fatalf("erfassen: %v", err)
	}
	if a.Kostenart != KostenartKraftstoff || a.BetragNetto != 80.12 || a.Lieferant != "Tanke" || a.Belegnummer != "B-1" || a.KmStand == nil || *a.KmStand != 12000 ||
		a.Liter == nil || *a.Liter != 45.5 || a.KostenstelleID == nil || *a.KostenstelleID != ccVehicle || a.Notiz != "voll" || a.Kennzeichen != "M-K 1" ||
		a.Status != CostActive || a.CreatedByUserID == nil || *a.CreatedByUserID != user || a.CreatedAt.IsZero() || !a.Datum.Equal(day(2026, 3, 2)) {
		t.Fatalf("Beleg: %+v", a)
	}
	// Ausdrueckliche Kostenstelle.
	b, err := s.RecordCost(ctx, CostInput{VehicleID: v.ID, Datum: day(2026, 3, 5), Kostenart: KostenartReparatur, BetragNetto: 300, KostenstelleID: &ccOther}, "", company)
	if err != nil || b.KostenstelleID == nil || *b.KostenstelleID != ccOther || b.CreatedByUserID != nil || b.KmStand != nil || b.Liter != nil {
		t.Fatalf("zweiter Beleg: %+v %v", b, err)
	}
	// Spaetere Aenderung am Fahrzeug verschiebt keine gebuchten Kosten.
	if _, err := vs.Update(ctx, v.ID, VehicleInput{Kennzeichen: "M-K 1", Art: ArtPkw, KostenstelleID: &ccOther}, company); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetCost(ctx, a.ID, company); got == nil || *got.KostenstelleID != ccVehicle {
		t.Fatalf("Kostenstelle nach Fahrzeugaenderung: %+v", got)
	}
	// Ohne Kostenstelle am Fahrzeug: keine.
	c, err := s.RecordCost(ctx, CostInput{VehicleID: w.ID, Datum: day(2026, 3, 3), Kostenart: KostenartKraftstoff, BetragNetto: 0.1}, "", company)
	if err != nil || c.KostenstelleID != nil {
		t.Fatalf("ohne Kostenstelle: %+v %v", c, err)
	}
	if _, err := s.RecordCost(ctx, CostInput{VehicleID: w.ID, Datum: day(2026, 3, 4), Kostenart: KostenartKraftstoff, BetragNetto: 0.2}, "", company); err != nil {
		t.Fatal(err)
	}

	list := func(f CostFilter) string {
		t.Helper()
		out, err := s.ListCosts(ctx, company, f)
		if err != nil {
			t.Fatal(err)
		}
		return costAmounts(out)
	}
	von, bis := day(2026, 3, 3), day(2026, 3, 4)
	for name, tc := range map[string]struct {
		f    CostFilter
		want string
	}{
		"alle":      {CostFilter{}, "reparatur:300,kraftstoff:0.2,kraftstoff:0.1,kraftstoff:80.12"},
		"Fahrzeug":  {CostFilter{VehicleID: &v.ID}, "reparatur:300,kraftstoff:80.12"},
		"Kostenart": {CostFilter{Kostenart: KostenartReparatur}, "reparatur:300"},
		"Zeitraum":  {CostFilter{Von: &von, Bis: &bis}, "kraftstoff:0.2,kraftstoff:0.1"},
		"nur von":   {CostFilter{Von: &bis}, "reparatur:300,kraftstoff:0.2"},
		"nur bis":   {CostFilter{Bis: &von}, "kraftstoff:0.1,kraftstoff:80.12"},
	} {
		if got := list(tc.f); got != tc.want {
			t.Errorf("Liste %s: %s", name, got)
		}
	}

	sum, err := s.SummarizeCosts(ctx, company, CostFilter{})
	if err != nil {
		t.Fatal(err)
	}
	// Reihenfolge nach Kostenarten; 0.1+0.2 in Cent exakt.
	if len(sum.Summen) != 2 || sum.Summen[0].Kostenart != KostenartKraftstoff || sum.Summen[0].Anzahl != 3 || roundCents(sum.Summen[0].BetragNetto) != 8042 ||
		sum.Summen[1].Kostenart != KostenartReparatur || sum.Summen[1].Anzahl != 1 || sum.Gesamt != 380.42 {
		t.Fatalf("Auswertung: %+v", sum)
	}
	if sw, _ := s.SummarizeCosts(ctx, company, CostFilter{VehicleID: &w.ID}); sw == nil || sw.Gesamt != 0.3 || len(sw.Summen) != 1 {
		t.Fatalf("Auswertung Fahrzeug: %+v", sw)
	}
	if sz, _ := s.SummarizeCosts(ctx, company, CostFilter{Von: &von, Bis: &bis}); sz == nil || sz.Gesamt != 0.3 {
		t.Fatalf("Auswertung Zeitraum: %+v", sz)
	}
	if se, _ := s.SummarizeCosts(ctx, company, CostFilter{Kostenart: KostenartLeasing}); se == nil || se.Gesamt != 0 || len(se.Summen) != 0 {
		t.Fatalf("leere Auswertung: %+v", se)
	}

	// Storno: einmalig, zaehlt nicht mehr, bleibt sichtbar.
	if _, err := s.StornoCost(ctx, b.ID, "doppelt", "", "anderer-mandant"); err == nil || err.Error() != "Kosten nicht gefunden" {
		t.Fatalf("fremdes Storno: %v", err)
	}
	st, err := s.StornoCost(ctx, b.ID, " doppelt ", user, company)
	if err != nil || st.Status != CostStorniert || st.StornoReason != "doppelt" || st.StorniertAt == nil || st.StorniertByUserID == nil || *st.StorniertByUserID != user {
		t.Fatalf("Storno: %+v %v", st, err)
	}
	if _, err := s.StornoCost(ctx, b.ID, "nochmal", "", company); err == nil || err.Error() != "Kosten sind bereits storniert" {
		t.Fatalf("zweites Storno: %v", err)
	}
	if again, _ := s.GetCost(ctx, b.ID, company); again == nil || again.StornoReason != "doppelt" {
		t.Fatalf("zweites Storno hat ueberschrieben: %+v", again)
	}
	if _, err := s.StornoCost(ctx, uuid.New(), "x", "", company); err == nil || err.Error() != "Kosten nicht gefunden" {
		t.Fatalf("unbekannt: %v", err)
	}
	if got := list(CostFilter{VehicleID: &v.ID}); got != "kraftstoff:80.12" {
		t.Fatalf("nach Storno: %s", got)
	}
	if got := list(CostFilter{VehicleID: &v.ID, IncludeStorniert: true}); got != "reparatur:300,kraftstoff:80.12" {
		t.Fatalf("mit stornierten: %s", got)
	}
	if sv, _ := s.SummarizeCosts(ctx, company, CostFilter{VehicleID: &v.ID, IncludeStorniert: true}); sv == nil || sv.Gesamt != 80.12 {
		t.Fatalf("Auswertung ohne Storno: %+v", sv)
	}

	// Gesamt in Cent gerundet: 0.29*100 ist als float knapp unter 29.
	z := seedLogbookVehicle(t, env, company, "M-K 3", false)
	if _, err := s.RecordCost(ctx, CostInput{VehicleID: z.ID, Datum: day(2026, 3, 6), Kostenart: KostenartWartung, BetragNetto: 0.29}, "", company); err != nil {
		t.Fatal(err)
	}
	if sz, _ := s.SummarizeCosts(ctx, company, CostFilter{VehicleID: &z.ID}); sz == nil || sz.Gesamt != 0.29 {
		t.Fatalf("Gesamt gerundet: %+v", sz)
	}

	// Am abgemeldeten Fahrzeug (Schlussrechnung) zulaessig.
	if _, err := vs.Deregister(ctx, w.ID, day(2026, 3, 5), "verkauft", company); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordCost(ctx, CostInput{VehicleID: w.ID, Datum: day(2026, 3, 20), Kostenart: KostenartSteuer, BetragNetto: 12}, "", company); err != nil {
		t.Fatalf("nach Abmeldung: %v", err)
	}
}

func TestCostRecordErrorsHaveNoEffectAndTenantsAreSeparate(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	one, two := seedCompany(t, env), seedCompany(t, env)
	vOne := seedLogbookVehicle(t, env, one, "M-KI 1", false)
	vTwo := seedLogbookVehicle(t, env, two, "M-KI 1", false)
	ccTwo := seedCostCenter(t, env, two)
	s := NewCostService(env.PG)
	ok := CostInput{VehicleID: vOne.ID, Datum: day(2026, 3, 2), Kostenart: KostenartWartung, BetragNetto: 10}
	with := func(f func(*CostInput)) CostInput { c := ok; f(&c); return c }

	for want, in := range map[string]CostInput{
		"Fahrzeug nicht gefunden":     with(func(c *CostInput) { c.VehicleID = vTwo.ID }),
		"Kostenstelle nicht gefunden": with(func(c *CostInput) { c.KostenstelleID = &ccTwo }),
		"Ungültige Kostenart":         with(func(c *CostInput) { c.Kostenart = "x" }),
	} {
		if _, err := s.RecordCost(ctx, in, "", one); err == nil || err.Error() != want {
			t.Errorf("%s: %v", want, err)
		}
	}
	if n := countCosts(t, env, vOne.ID, vTwo.ID); n != 0 {
		t.Fatalf("nach Fehlern %d Belege", n)
	}

	x, err := s.RecordCost(ctx, ok, "", one)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetCost(ctx, x.ID, two); err == nil || err.Error() != "Kosten nicht gefunden" {
		t.Fatalf("fremder Abruf: %v", err)
	}
	if l, _ := s.ListCosts(ctx, two, CostFilter{IncludeStorniert: true}); len(l) != 0 {
		t.Fatalf("fremde Liste: %+v", l)
	}
	if sum, _ := s.SummarizeCosts(ctx, two, CostFilter{}); sum == nil || sum.Gesamt != 0 {
		t.Fatalf("fremde Auswertung: %+v", sum)
	}
	if _, err := s.StornoCost(ctx, x.ID, "x", "", two); err == nil || err.Error() != "Kosten nicht gefunden" {
		t.Fatalf("fremdes Storno: %v", err)
	}
	if still, _ := s.GetCost(ctx, x.ID, one); still == nil || still.Status != CostActive {
		t.Fatalf("anderer Mandant hat storniert: %+v", still)
	}
}

// CHECKs und RESTRICT der Migration 105 gegen Direkt-SQL.
func TestCostConstraints(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedCompany(t, env)
	v := seedLogbookVehicle(t, env, company, "M-KC 1", false)
	cc := seedCostCenter(t, env, company)
	insert := func(cols, vals string, args ...any) error {
		_, err := env.PG.Exec(ctx, `INSERT INTO fleet_costs (id, vehicle_id, datum, kostenart, betrag_netto`+cols+`) VALUES ($1, $2, '2026-03-02', `+vals+`)`,
			append([]any{uuid.New(), v.ID}, args...)...)
		return err
	}
	for name, tc := range map[string]struct {
		cols, vals string
		args       []any
		ok         bool
	}{
		"gueltig":                {"", "'wartung', 10", nil, true},
		"Kraftstoff mit Litern":  {", liter", "'kraftstoff', 10, 5", nil, true},
		"Kostenart":              {"", "'bussgeld', 10", nil, false},
		"Betrag 0":               {"", "'wartung', 0", nil, false},
		"Betrag negativ":         {"", "'wartung', -1", nil, false},
		"sonstiges ohne Name":    {"", "'sonstiges', 10", nil, false},
		"sonstiges mit Name":     {", bezeichnung", "'sonstiges', 10, 'Vignette'", nil, true},
		"km negativ":             {", km_stand", "'wartung', 10, -1", nil, false},
		"Liter bei Wartung":      {", liter", "'wartung', 10, 5", nil, false},
		"Liter 0":                {", liter", "'kraftstoff', 10, 0", nil, false},
		"Status":                 {", status", "'wartung', 10, 'x'", nil, false},
		"Storno ohne Grund":      {", status, storniert_at", "'wartung', 10, 'storniert', now()", nil, false},
		"Storno vollstaendig":    {", status, storniert_at, storno_reason", "'wartung', 10, 'storniert', now(), 'x'", nil, true},
		"aktiv mit Storno-Zeit":  {", storniert_at", "'wartung', 10, now()", nil, false},
		"Kostenstelle unbekannt": {", kostenstelle_id", "'wartung', 10, 'gibt-es-nicht'", nil, false},
		"Kostenstelle":           {", kostenstelle_id", "'wartung', 10, $3", []any{cc}, true},
	} {
		err := insert(tc.cols, tc.vals, tc.args...)
		if (err == nil) != tc.ok {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := env.PG.Exec(ctx, `DELETE FROM fleet_vehicles WHERE id=$1`, v.ID); err == nil {
		t.Fatal("Fahrzeug mit Kosten liess sich loeschen")
	}
	if _, err := env.PG.Exec(ctx, `DELETE FROM cost_centers WHERE id=$1`, cc); err == nil {
		t.Fatal("Kostenstelle mit Kosten liess sich loeschen")
	}
}
