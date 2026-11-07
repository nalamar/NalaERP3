package production

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/materials"
	"nalaerp3/internal/testutil"
)

// Gleichzeitige Reservierungen (Backlog H.5.3): ReserveAvailableTx sperrt je
// Material und Lager, damit zwei offene Transaktionen zusammen nie mehr als
// den Bestand reservieren.
func TestReserveAvailableTxConcurrent(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	p := seedPosition(t, env)
	project := p.project.String()
	mat, wh := seedStock(t, env, p.company, "Stk", 10)
	wh2 := "wh-" + uuid.NewString()[:12]
	if _, err := env.PG.Exec(ctx, `INSERT INTO warehouses (id, code, name, company_id) VALUES ($1,$1,'Zweitlager',$2)`, wh2, p.company); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `INSERT INTO stock_movements (id, material_id, warehouse_id, quantity, uom, movement_type) VALUES ($1,$2,$3,10,'Stk','purchase')`,
		uuid.NewString(), mat, wh2); err != nil {
		t.Fatal(err)
	}
	type result struct {
		qty float64
		err error
	}
	// reserveAsync reserviert in einer eigenen Transaktion und schreibt sie fest.
	reserveAsync := func(w string, want float64) <-chan result {
		done := make(chan result, 1)
		go func() {
			var qty float64
			err := func() error {
				tx, err := env.PG.Begin(ctx)
				if err != nil {
					return err
				}
				defer func() { _ = tx.Rollback(ctx) }()
				if _, qty, err = materials.ReserveAvailableTx(ctx, tx, p.company, mat, w, project, want, "Fertigungsauftrag", "FA-C"); err != nil {
					return err
				}
				return tx.Commit(ctx)
			}()
			done <- result{qty, err}
		}()
		return done
	}
	wait := func(name string, ch <-chan result) result {
		t.Helper()
		select {
		case r := <-ch:
			if r.err != nil {
				t.Fatalf("%s: %v", name, r.err)
			}
			return r
		case <-time.After(15 * time.Second):
			t.Fatalf("%s: keine Antwort", name)
			return result{}
		}
	}
	reserved := func(w string) float64 {
		t.Helper()
		var sum float64
		if err := env.PG.QueryRow(ctx, `SELECT COALESCE(SUM(qty),0)::float8 FROM stock_reservations WHERE material_id=$1 AND warehouse_id=$2 AND status='aktiv'`, mat, w).Scan(&sum); err != nil {
			t.Fatal(err)
		}
		return sum
	}

	// A haelt die Sperre in einer offenen Transaktion.
	txA, err := env.PG.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = txA.Rollback(ctx) }()
	if _, qa, err := materials.ReserveAvailableTx(ctx, txA, p.company, mat, wh, project, 10, "Fertigungsauftrag", "FA-A"); err != nil || qa != 10 {
		t.Fatalf("A: %v %v", qa, err)
	}
	// B auf demselben Lager wartet, bis A fertig ist.
	b := reserveAsync(wh, 10)
	select {
	case r := <-b:
		t.Fatalf("B hat nicht auf A gewartet: %v %v", r.qty, r.err)
	case <-time.After(500 * time.Millisecond):
	}
	// Ein anderes Lager wartet nicht (Sperre je Material und Lager).
	if r := wait("anderes Lager", reserveAsync(wh2, 4)); r.qty != 4 {
		t.Fatalf("anderes Lager: %v", r.qty)
	}
	if err := txA.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	// Negativfall: nach A ist nichts mehr frei, B reserviert nichts.
	if r := wait("B", b); r.qty != 0 {
		t.Fatalf("B nach A: %v, erwartet 0", r.qty)
	}
	if got := reserved(wh); got != 10 {
		t.Fatalf("reserviert in Lager 1: %v, erwartet 10", got)
	}

	// Viele zugleich auf dem Rest von Lager 2 (6): zusammen genau 6.
	var wg sync.WaitGroup
	var mu sync.Mutex
	var total float64
	for i := 0; i < 8; i++ {
		wg.Add(1)
		ch := reserveAsync(wh2, 2)
		go func() {
			defer wg.Done()
			r := <-ch
			mu.Lock()
			defer mu.Unlock()
			if r.err != nil {
				t.Errorf("parallel: %v", r.err)
			}
			total += r.qty
		}()
	}
	wg.Wait()
	if total != 6 || reserved(wh2) != 10 {
		t.Fatalf("parallel: %v neu reserviert, %v in Lager 2, erwartet 6 und 10", total, reserved(wh2))
	}
}
