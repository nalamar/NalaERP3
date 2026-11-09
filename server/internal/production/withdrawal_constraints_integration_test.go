package production

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/materials"
	"nalaerp3/internal/testutil"
)

// Die Datenbank schuetzt Entnahmen und den Reservierungsstatus
// "verbraucht" (Migration 113, Backlog H.3.1) auch gegen Direkt-SQL.
func TestWithdrawalTablesConstraints(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	p := seedPosition(t, env)
	ok := func(name, sql string, args ...any) {
		t.Helper()
		if _, err := env.PG.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: abgelehnt: %v", name, err)
		}
	}
	bad := func(name, sql string, args ...any) {
		t.Helper()
		if _, err := env.PG.Exec(ctx, sql, args...); err == nil {
			t.Fatalf("%s: von der Datenbank angenommen", name)
		}
	}
	mat, wh := seedStock(t, env, p.company, "Stk", 20)

	order, item := uuid.New(), uuid.New()
	ok("Auftrag", `INSERT INTO production_orders (id, company_id, nummer, project_id, elevation_id, single_elevation_id, stueckzahl, status, freigegeben_at, inhalt_hash, position_nummer)
		VALUES ($1,$2,'FA-EN-1',$3,$4,$5,2,'freigegeben',now(),'h','3')`, order, p.company, p.project, p.elevation, p.variant)
	ok("Zeile", `INSERT INTO production_order_items (id, order_id, pos, art, menge_je_stueck, unit, material_id)
		VALUES ($1,$2,1,'artikel',5,'Stk',$3)`, item, order, mat)

	insRes := `INSERT INTO stock_reservations (id, material_id, warehouse_id, project_id, qty, status) VALUES ($1,$2,$3,$4,$5,$6)`
	resv := "resv-" + uuid.NewString()[:12]
	ok("Reservierung verbraucht", insRes, resv, mat, wh, p.project, 10, "verbraucht")
	bad("Reservierung unbekannter Status", insRes, "resv-"+uuid.NewString()[:12], mat, wh, p.project, 1, "entnommen")

	mv := func() string {
		t.Helper()
		id := uuid.NewString()
		ok("Lagerbewegung", `INSERT INTO stock_movements (id, material_id, warehouse_id, quantity, uom, movement_type, reason, reference)
			VALUES ($1,$2,$3,-1,'Stk','out','Entnahme Fertigungsauftrag','FA-EN-1')`, id, mat, wh)
		return id
	}
	ins := `INSERT INTO production_order_withdrawals (id, company_id, order_id, item_id, material_id, warehouse_id, menge, einheit,
		stock_movement_id, stock_reservation_id, reservierung_menge) VALUES ($1,$2,$3,$4,$5,$6,$7,'Stk',$8,$9,$10)`
	w1, m1 := uuid.New(), mv()
	ok("Entnahme mit Zeile und Reservierung", ins, w1, p.company, order, item, mat, wh, 4, m1, resv, 4)
	ok("Entnahme ohne Zeile", ins, uuid.New(), p.company, order, nil, mat, wh, 1, mv(), nil, 0)
	bad("Menge 0", ins, uuid.New(), p.company, order, item, mat, wh, 0, mv(), nil, 0)
	bad("Menge negativ", ins, uuid.New(), p.company, order, item, mat, wh, -1, mv(), nil, 0)
	bad("mehr Reservierung verbraucht als entnommen", ins, uuid.New(), p.company, order, item, mat, wh, 2, mv(), resv, 3)
	bad("Reservierung negativ", ins, uuid.New(), p.company, order, item, mat, wh, 2, mv(), resv, -1)
	bad("Lagerbewegung doppelt", ins, uuid.New(), p.company, order, item, mat, wh, 1, m1, nil, 0)
	bad("ohne Lagerbewegung", ins, uuid.New(), p.company, order, item, mat, wh, 1, uuid.NewString(), nil, 0)
	bad("unbekannte Zeile", ins, uuid.New(), p.company, order, uuid.New(), mat, wh, 1, mv(), nil, 0)

	// Storno nur vollstaendig, mit eigener Gegenbuchung.
	storno := `UPDATE production_order_withdrawals SET storniert_at=now(), storno_reason=$2, storno_movement_id=$3 WHERE id=$1`
	bad("Storno ohne Begruendung", storno, w1, " ", mv())
	bad("Storno ohne Gegenbuchung", storno, w1, "falsches Lager", nil)
	bad("Storno mit derselben Bewegung", storno, w1, "falsches Lager", m1)
	bad("Begruendung ohne Storno", `UPDATE production_order_withdrawals SET storno_reason='x' WHERE id=$1`, w1)
	ok("Storno", storno, w1, "falsches Lager", mv())

	// Belege halten Auftrag, Zeile, Material, Lager und Bewegung fest.
	bad("Auftrag mit Entnahmen loeschen", `DELETE FROM production_orders WHERE id=$1`, order)
	bad("Zeile mit Entnahmen loeschen", `DELETE FROM production_order_items WHERE id=$1`, item)
	bad("Material mit Entnahmen loeschen", `DELETE FROM materials WHERE id=$1`, mat)
	bad("Bewegung einer Entnahme loeschen", `DELETE FROM stock_movements WHERE id=$1`, m1)
	ok("Reservierung loeschen leert den Bezug", `DELETE FROM stock_reservations WHERE id=$1`, resv)
	var ref *string
	if err := env.PG.QueryRow(ctx, `SELECT stock_reservation_id FROM production_order_withdrawals WHERE id=$1`, w1).Scan(&ref); err != nil || ref != nil {
		t.Fatalf("Bezug zur Reservierung: %v %v", ref, err)
	}
}

// Eine verbrauchte Reservierung laesst sich nicht von Hand freigeben;
// eine aktive weiterhin (Gegenprobe).
func TestReleaseReservationRejectsConsumed(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	p := seedPosition(t, env)
	mat, wh := seedStock(t, env, p.company, "Stk", 5)
	svc := materials.NewService(env.PG, nil, "")
	ins := `INSERT INTO stock_reservations (id, material_id, warehouse_id, project_id, qty, status) VALUES ($1,$2,$3,$4,1,$5)`
	used, active := "resv-"+uuid.NewString()[:12], "resv-"+uuid.NewString()[:12]
	for id, st := range map[string]string{used: "verbraucht", active: "aktiv"} {
		if _, err := env.PG.Exec(ctx, ins, id, mat, wh, p.project, st); err != nil {
			t.Fatalf("Reservierung %s: %v", st, err)
		}
	}
	if _, err := svc.ReleaseReservation(ctx, used, p.company); err == nil || !strings.Contains(err.Error(), "bereits verbraucht") {
		t.Fatalf("verbrauchte Reservierung freigegeben: %v", err)
	}
	var st string
	if err := env.PG.QueryRow(ctx, `SELECT status FROM stock_reservations WHERE id=$1`, used).Scan(&st); err != nil || st != "verbraucht" {
		t.Fatalf("Status nach Ablehnung: %q %v", st, err)
	}
	out, err := svc.ReleaseReservation(ctx, active, p.company)
	if err != nil || out.Status != "freigegeben" {
		t.Fatalf("aktive Reservierung: %+v %v", out, err)
	}
	// verbraucht zaehlt nicht zum Reservierten: verfuegbar 5 - 0
	if got, err := svc.AvailableStock(ctx, mat, wh, p.company); err != nil || got != 5 {
		t.Fatalf("verfuegbar: %v %v", got, err)
	}
}
