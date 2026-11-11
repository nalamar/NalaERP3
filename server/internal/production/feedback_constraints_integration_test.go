package production

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Die Datenbank schuetzt Rueckmeldungen je Arbeitsgang (Migration 114,
// Backlog H.3.5a) auch gegen Direkt-SQL.
func TestFeedbackTableConstraints(t *testing.T) {
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
	user := testutil.SeedAuthUser(t, env, "fb-"+uuid.NewString()[:8]+"@example.com", "Passwort-123!", "production")

	wc, order, op, op2 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	ok("Arbeitsplatz", `INSERT INTO production_work_centers (id, company_id, name, stunden_je_tag) VALUES ($1,$2,'Zuschnitt',8)`, wc, p.company)
	ok("Auftrag", `INSERT INTO production_orders (id, company_id, nummer, project_id, elevation_id, single_elevation_id, stueckzahl)
		VALUES ($1,$2,'FA-RM-1',$3,$4,$5,4)`, order, p.company, p.project, p.elevation, p.variant)
	insOp := `INSERT INTO production_order_operations (id, order_id, pos, name, work_center_id) VALUES ($1,$2,$3,'Zuschnitt',$4)`
	ok("Arbeitsgang", insOp, op, order, 1, wc)
	ok("Arbeitsgang 2", insOp, op2, order, 2, wc)

	ins := `INSERT INTO production_operation_feedback (id, company_id, operation_id, ist_minuten, notiz, gemeldet_by_user_id) VALUES ($1,$2,$3,$4,$5,$6)`
	f1 := uuid.New()
	ok("Rueckmeldung mit Ist-Zeit", ins, f1, p.company, op, 95.5, "", user)
	bad("zweite gueltige Rueckmeldung", ins, uuid.New(), p.company, op, nil, "", user)
	ok("ohne Ist-Zeit und ohne Benutzer", ins, uuid.New(), p.company, op2, nil, "Notiz", nil)
	// Ist-Zeit an einem Arbeitsgang ohne Rueckmeldung, damit nur der
	// CHECK ablehnen kann.
	op3 := uuid.New()
	ok("Arbeitsgang 3", insOp, op3, order, 3, wc)
	for name, v := range map[string]float64{"Ist 0": 0, "Ist negativ": -1, "Ist zu gross": 100000.01} {
		bad(name, ins, uuid.New(), p.company, op3, v, "", nil)
	}
	ok("Ist knapp ueber 0", ins, uuid.New(), p.company, op3, 0.01, "", nil)
	ok("Ist an der Grenze", `UPDATE production_operation_feedback SET ist_minuten=100000 WHERE id=$1`, f1)
	bad("Ist ueber der Grenze per UPDATE", `UPDATE production_operation_feedback SET ist_minuten=100000.01 WHERE id=$1`, f1)
	bad("unbekannter Arbeitsgang", ins, uuid.New(), p.company, uuid.New(), nil, "", nil)
	bad("unbekannter Mandant", ins, uuid.New(), "kein-mandant", op, nil, "", nil)

	// Ruecknahme nur vollstaendig.
	revert := `UPDATE production_operation_feedback SET zurueckgenommen_at=now(), ruecknahme_reason=$2, zurueckgenommen_by_user_id=$3 WHERE id=$1`
	bad("Ruecknahme ohne Begruendung", revert, f1, "  ", user)
	bad("Begruendung ohne Ruecknahme", `UPDATE production_operation_feedback SET ruecknahme_reason='x' WHERE id=$1`, f1)
	bad("Benutzer ohne Ruecknahme", `UPDATE production_operation_feedback SET zurueckgenommen_by_user_id=$2 WHERE id=$1`, f1, user)
	bad("Ruecknahme vor der Meldung", `UPDATE production_operation_feedback SET zurueckgenommen_at=gemeldet_at - interval '1 minute', ruecknahme_reason='x' WHERE id=$1`, f1)
	ok("Ruecknahme", revert, f1, "versehentlich gemeldet", user)
	ok("nach Ruecknahme neu melden", ins, uuid.New(), p.company, op, 80, "", user)

	// Arbeitsgang mit Rueckmeldungen (auch zurueckgenommenen) nicht
	// loeschbar, ohne Rueckmeldung weiterhin (Gegenprobe).
	bad("Arbeitsgang mit Rueckmeldung loeschen", `DELETE FROM production_order_operations WHERE id=$1`, op)
	op4 := uuid.New()
	ok("Arbeitsgang 4", insOp, op4, order, 4, wc)
	ok("Arbeitsgang ohne Rueckmeldung loeschen", `DELETE FROM production_order_operations WHERE id=$1`, op4)
	var n int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM production_operation_feedback WHERE operation_id=$1`, op).Scan(&n); err != nil || n != 2 {
		t.Fatalf("Rueckmeldungen am Arbeitsgang: %d %v", n, err)
	}
}
