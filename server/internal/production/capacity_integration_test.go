package production

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Auslastung je Arbeitsplatz und Woche (Backlog H.2.4).
func TestCapacity(t *testing.T) {
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
	// Bayern; 25.12.2026 (Fr) gesetzlich, 24.12. (Do) Betriebsruhe.
	exec(`INSERT INTO hr_calendar_settings (company_id, bundesland) VALUES ($1, 'BY')`, p.company)
	exec(`INSERT INTO hr_company_days (id, company_id, datum, name) VALUES ($1, $2, '2026-12-24', 'Heiligabend')`, uuid.New(), p.company)

	wc := func(company, name string, stunden float64) uuid.UUID {
		t.Helper()
		x, err := svc.CreateWorkCenter(ctx, WorkCenterInput{Name: name, StundenJeTag: stunden}, company)
		if err != nil {
			t.Fatal(err)
		}
		return x.ID
	}
	a, b, c := wc(p.company, "A Zuschnitt", 8), wc(p.company, "B Montage", 7.5), wc(p.company, "C leer", 8)
	wc(other.company, "Fremd", 8)
	order := func(stueck int) uuid.UUID {
		t.Helper()
		o, err := svc.Create(ctx, OrderInput{ProjectID: p.project, ElevationID: p.elevation, SingleElevationID: p.variant, Stueckzahl: stueck}, "", p.company)
		if err != nil {
			t.Fatal(err)
		}
		return o.ID
	}
	op := func(orderID, wcID uuid.UUID, ruest, jeStueck float64, woche string) uuid.UUID {
		t.Helper()
		var w *time.Time
		if woche != "" {
			w = dayPtr(woche)
		}
		x, err := svc.AddOperation(ctx, orderID, OperationInput{Name: "x", WorkCenterID: wcID, RuestzeitMin: ruest, MinutenJeStueck: jeStueck, GeplanteWoche: w}, "", p.company)
		if err != nil {
			t.Fatal(err)
		}
		return x.ID
	}
	o1, o2, o3, o4 := order(4), order(1), order(1), order(1)
	op(o1, a, 0, 600, "2026-12-02")  // 2400 = genau voll (5 x 480)
	op(o1, a, 30, 100, "2026-12-23") // 430 in der Weihnachtswoche
	op(o1, a, 15, 0, "")             // ungeplant
	op(o1, a, 0, 9999, "2027-03-01") // ausserhalb des Zeitraums
	op(o1, b, 0, 10, "2026-12-21")   // 40 auf B, B wird deaktiviert
	op(o3, a, 9999, 0, "2026-11-30") // fertig: zaehlt nicht
	op(o4, a, 9999, 0, "2026-11-30") // storniert: zaehlt nicht
	// Rueckmeldungen per Direkt-SQL (H.3.5c): erledigte zaehlen nicht,
	// zurueckgenommene weiter.
	done, doneU, reverted := op(o2, a, 0, 500, "2026-12-07"), op(o2, a, 7, 0, ""), op(o2, a, 0, 50, "2026-12-14")
	for _, id := range []uuid.UUID{done, doneU} {
		exec(`INSERT INTO production_operation_feedback (id, company_id, operation_id) VALUES ($1,$2,$3)`, uuid.New(), p.company, id)
	}
	exec(`INSERT INTO production_operation_feedback (id, company_id, operation_id, zurueckgenommen_at, ruecknahme_reason)
		VALUES ($1,$2,$3,now(),'versehentlich')`, uuid.New(), p.company, reverted)
	off := false
	for id, name := range map[uuid.UUID]string{b: "B Montage", c: "C leer"} {
		if _, err := svc.UpdateWorkCenter(ctx, id, WorkCenterInput{Name: name, StundenJeTag: map[uuid.UUID]float64{b: 7.5, c: 8}[id]}, &off, p.company); err != nil {
			t.Fatal(err)
		}
	}
	// Auftrag eines fremden Mandanten mit Verweis auf A (nur per Direkt-SQL
	// moeglich): zaehlt nicht.
	foreign := uuid.New()
	exec(`INSERT INTO production_orders (id, company_id, nummer, project_id, elevation_id, single_elevation_id, stueckzahl)
		VALUES ($1,$2,'FA-FREMD',$3,$4,$5,1)`, foreign, other.company, other.project, other.elevation, other.variant)
	exec(`INSERT INTO production_order_operations (id, order_id, pos, name, work_center_id, ruestzeit_min, geplante_woche)
		VALUES ($1,$2,1,'fremd',$3,9999,'2026-11-30')`, uuid.New(), foreign, a)
	exec(`UPDATE production_orders SET status='fertig', position_nummer='3', inhalt_hash='h', freigegeben_at=now(), begonnen_at=now(), fertig_at=now() WHERE id=$1`, o3)
	if _, err := svc.Cancel(ctx, o4, "doppelt", "", p.company); err != nil {
		t.Fatal(err)
	}

	type cell struct {
		woche         string
		tage          int
		verf, geplant float64
		hinweise      string
	}
	check := func(capa *Capacity, wcID uuid.UUID, aktiv bool, ungeplant float64, want []cell) {
		t.Helper()
		for _, r := range capa.Arbeitsplaetze {
			if r.WorkCenterID != wcID {
				continue
			}
			if r.Aktiv != aktiv || r.UngeplantMin != ungeplant || len(r.Wochen) != len(want) {
				t.Fatalf("%s: aktiv %v, ungeplant %v, %d Wochen", r.Name, r.Aktiv, r.UngeplantMin, len(r.Wochen))
			}
			for i, w := range want {
				g := r.Wochen[i]
				got := cell{g.Woche.Format("2006-01-02"), g.Arbeitstage, g.VerfuegbarMin, g.GeplantMin, strings.Join(g.Hinweise, ",")}
				if got != w {
					t.Errorf("%s Woche %d: %+v, erwartet %+v", r.Name, i, got, w)
				}
			}
			return
		}
		t.Fatalf("Arbeitsplatz %s fehlt", wcID)
	}

	capa, err := svc.Capacity(ctx, day("2026-11-25"), day("2026-12-23"), p.company)
	if err != nil {
		t.Fatal(err)
	}
	if capa.Von.Format("2006-01-02") != "2026-11-23" || capa.Bis.Format("2006-01-02") != "2026-12-21" {
		t.Fatalf("Zeitraum: %v bis %v", capa.Von, capa.Bis)
	}
	names := []string{}
	for _, r := range capa.Arbeitsplaetze {
		names = append(names, r.Name)
	}
	if strings.Join(names, ",") != "A Zuschnitt,B Montage" {
		t.Fatalf("Arbeitsplätze: %v (C deaktiviert ohne Arbeit, Fremd anderer Mandant)", names)
	}
	check(capa, a, true, 15, []cell{
		{"2026-11-23", 5, 2400, 0, ""},
		{"2026-11-30", 5, 2400, 2400, ""},
		{"2026-12-07", 5, 2400, 0, ""},
		{"2026-12-14", 5, 2400, 50, ""},
		{"2026-12-21", 3, 1440, 430, ""},
	})
	check(capa, b, false, 0, []cell{
		{"2026-11-23", 5, 2250, 0, ""},
		{"2026-11-30", 5, 2250, 0, ""},
		{"2026-12-07", 5, 2250, 0, ""},
		{"2026-12-14", 5, 2250, 0, ""},
		{"2026-12-21", 3, 1350, 40, ""},
	})

	// Eine Minute mehr: Ueberlast.
	op(o2, a, 1, 0, "2026-11-30")
	capa, err = svc.Capacity(ctx, day("2026-11-30"), day("2026-11-30"), p.company)
	if err != nil {
		t.Fatal(err)
	}
	check(capa, a, true, 15, []cell{{"2026-11-30", 5, 2400, 2401, HintUeberlast}})

	// Ohne Bundesland nur die bundesweiten Feiertage: 25.12. frei, der
	// 24.12. ist im anderen Mandanten kein Betriebsruhetag.
	capa, err = svc.Capacity(ctx, day("2026-12-21"), day("2026-12-21"), other.company)
	if err != nil {
		t.Fatal(err)
	}
	if len(capa.Arbeitsplaetze) != 1 || capa.Arbeitsplaetze[0].Name != "Fremd" || capa.Arbeitsplaetze[0].Wochen[0].Arbeitstage != 4 ||
		capa.Arbeitsplaetze[0].Wochen[0].GeplantMin != 0 {
		t.Fatalf("anderer Mandant: %+v", capa.Arbeitsplaetze)
	}

	// Zeitraum: rueckwaerts und zu lang abgelehnt, 26 Wochen gehen.
	_, err = svc.Capacity(ctx, day("2026-12-07"), day("2026-11-30"), p.company)
	wantErr(t, "bis vor von", err, "Ungültiger Zeitraum: bis liegt vor von")
	_, err = svc.Capacity(ctx, day("2026-11-30"), day("2026-11-30").AddDate(0, 0, 7*26), p.company)
	wantErr(t, "27 Wochen", err, "Zeitraum ist zu groß (höchstens 26 Wochen)")
	if capa, err = svc.Capacity(ctx, day("2026-11-30"), day("2026-11-30").AddDate(0, 0, 7*25), p.company); err != nil || len(capa.Arbeitsplaetze[0].Wochen) != 26 {
		t.Fatalf("26 Wochen: %v", err)
	}
	// Ueber den Jahreswechsel: Neujahr (Fr) gesetzlich.
	capa, err = svc.Capacity(ctx, day("2026-12-28"), day("2026-12-28"), p.company)
	if err != nil || capa.Arbeitsplaetze[0].Wochen[0].Arbeitstage != 4 {
		t.Fatalf("Woche mit Neujahr: %+v %v", capa, err)
	}
}
