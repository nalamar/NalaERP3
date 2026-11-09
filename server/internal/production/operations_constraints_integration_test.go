package production

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Die Datenbank schuetzt Arbeitsplaetze, Vorlagen und Arbeitsgaenge
// (Migration 112, Backlog H.2.1) auch gegen Direkt-SQL.
func TestOperationTablesConstraints(t *testing.T) {
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

	insWC := `INSERT INTO production_work_centers (id, company_id, name, stunden_je_tag) VALUES ($1,$2,$3,$4)`
	wc := uuid.New()
	ok("Arbeitsplatz", insWC, wc, p.company, "Zuschnitt", 8)
	ok("24 Stunden", insWC, uuid.New(), p.company, "Schicht", 24)
	bad("Name doppelt (Gross/Klein, Leerraum)", insWC, uuid.New(), p.company, " zuschnitt ", 8)
	bad("Name leer", insWC, uuid.New(), p.company, "  ", 8)
	bad("0 Stunden", insWC, uuid.New(), p.company, "Null", 0)
	bad("ueber 24 Stunden", insWC, uuid.New(), p.company, "Zuviel", 24.5)
	other := seedPosition(t, env)
	ok("gleicher Name im anderen Mandanten", insWC, uuid.New(), other.company, "Zuschnitt", 8)

	insTpl := `INSERT INTO production_operation_templates (id, company_id, name, work_center_id, reihenfolge, ruestzeit_min, minuten_je_stueck)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`
	tpl := uuid.New()
	ok("Vorlage", insTpl, tpl, p.company, "Zuschnitt", wc, 1, 15, 4.5)
	bad("Vorlage Name doppelt", insTpl, uuid.New(), p.company, "ZUSCHNITT", wc, 2, 0, 0)
	bad("Vorlage Reihenfolge 0", insTpl, uuid.New(), p.company, "R0", wc, 0, 0, 0)
	bad("Vorlage Ruestzeit negativ", insTpl, uuid.New(), p.company, "RN", wc, 1, -1, 0)
	bad("Vorlage je Stueck negativ", insTpl, uuid.New(), p.company, "SN", wc, 1, 0, -1)
	bad("Vorlage ohne Arbeitsplatz", insTpl, uuid.New(), p.company, "OA", uuid.New(), 1, 0, 0)

	order := uuid.New()
	ok("Auftrag", `INSERT INTO production_orders (id, company_id, nummer, project_id, elevation_id, single_elevation_id, stueckzahl)
		VALUES ($1,$2,'FA-OP-1',$3,$4,$5,4)`, order, p.company, p.project, p.elevation, p.variant)

	insOp := `INSERT INTO production_order_operations (id, order_id, pos, name, work_center_id, template_id, ruestzeit_min, minuten_je_stueck, geplante_woche)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	op := uuid.New()
	ok("Arbeitsgang aus Vorlage, Montag", insOp, op, order, 1, "Zuschnitt", wc, tpl, 15, 4.5, "2026-11-09")
	ok("Arbeitsgang ungeplant ohne Vorlage", insOp, uuid.New(), order, 2, "Endkontrolle", wc, nil, 0, 0, nil)
	bad("Woche kein Montag", insOp, uuid.New(), order, 3, "x", wc, nil, 0, 0, "2026-11-10")
	bad("Sonntag", insOp, uuid.New(), order, 3, "x", wc, nil, 0, 0, "2026-11-15")
	bad("Pos doppelt", insOp, uuid.New(), order, 1, "x", wc, nil, 0, 0, nil)
	bad("Pos 0", insOp, uuid.New(), order, 0, "x", wc, nil, 0, 0, nil)
	bad("Name leer", insOp, uuid.New(), order, 3, " ", wc, nil, 0, 0, nil)
	bad("Zeit negativ", insOp, uuid.New(), order, 3, "x", wc, nil, 0, -2, nil)
	bad("unbekannter Auftrag", insOp, uuid.New(), uuid.New(), 1, "x", wc, nil, 0, 0, nil)

	// Loeschen: Vorlage weg -> Herkunft leer, Arbeitsgang bleibt;
	// Arbeitsplatz und Auftrag mit Arbeitsgaengen nicht loeschbar.
	ok("Vorlage loeschen", `DELETE FROM production_operation_templates WHERE id=$1`, tpl)
	var tplRef *uuid.UUID
	if err := env.PG.QueryRow(ctx, `SELECT template_id FROM production_order_operations WHERE id=$1`, op).Scan(&tplRef); err != nil {
		t.Fatalf("Arbeitsgang nach Loeschen der Vorlage: %v", err)
	}
	if tplRef != nil {
		t.Fatalf("Herkunft nicht geleert: %v", tplRef)
	}
	bad("Arbeitsplatz mit Arbeitsgaengen loeschen", `DELETE FROM production_work_centers WHERE id=$1`, wc)
	bad("Auftrag mit Arbeitsgaengen loeschen", `DELETE FROM production_orders WHERE id=$1`, order)
}
