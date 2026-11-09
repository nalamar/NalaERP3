package production

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Entnahme im Service (Backlog H.3.2a): Lagerabgang, Verbrauch und
// Wiederherstellung der Reservierung, Entnommen/Offen/Fehlmenge an der
// Zeile, Storno, Mandantengrenze.
func TestWithdrawals(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	p := seedPosition(t, env)
	other := seedPosition(t, env)
	seedMaterials(t, env, p.variant)
	// Artikel A-7: 1,5 je Stueck ohne Einheit (= Stueck); Glas mit fremdem
	// Material wird ohne Material kopiert.
	artMat, wh := seedStock(t, env, p.company, "Stk", 10)
	otherMat, otherWh := seedStock(t, env, other.company, "Stk", 10)
	looseMat := addMaterial(t, env, p.company, "Stk", wh, 5)
	for table, m := range map[string]string{"single_elevation_articles": artMat, "single_elevation_glass": otherMat} {
		if _, err := env.PG.Exec(ctx, `UPDATE `+table+` SET material_id=$2 WHERE single_elevation_id=$1`, p.variant, m); err != nil {
			t.Fatal(err)
		}
	}
	stock := func(m string) float64 {
		t.Helper()
		var q float64
		if err := env.PG.QueryRow(ctx, `SELECT COALESCE(SUM(quantity),0)::float8 FROM stock_movements WHERE material_id=$1 AND warehouse_id=$2`, m, wh).Scan(&q); err != nil {
			t.Fatal(err)
		}
		return q
	}
	reservation := func(o *Order) (string, float64) {
		t.Helper()
		var st string
		var q float64
		if err := env.PG.QueryRow(ctx, `SELECT r.status, r.qty::float8 FROM production_order_items i JOIN stock_reservations r ON r.id = i.stock_reservation_id
			WHERE i.order_id=$1 AND i.art='artikel'`, o.ID).Scan(&st, &q); err != nil {
			t.Fatal(err)
		}
		return st, q
	}
	item := func(o *Order) Item {
		t.Helper()
		got, err := svc.Get(ctx, o.ID, p.company)
		if err != nil {
			t.Fatal(err)
		}
		for _, h := range got.Hinweise {
			if h == HintFehlmenge {
				t.Fatalf("Fehlmenge nach Entnahme: %v", got.Hinweise)
			}
		}
		return itemOf(t, got, "artikel")
	}
	figures := func(name string, it Item, reserviert, entnommen, offen float64) {
		t.Helper()
		if !near(it.Reserviert, reserviert) || !near(it.Entnommen, entnommen) || !near(it.Offen, offen) || !near(it.Fehlmenge, 0) {
			t.Fatalf("%s: reserviert %v entnommen %v offen %v fehl %v", name, deref(it.Reserviert), deref(it.Entnommen), deref(it.Offen), deref(it.Fehlmenge))
		}
	}

	o, err := svc.Create(ctx, OrderInput{ProjectID: p.project, ElevationID: p.elevation, SingleElevationID: p.variant, Stueckzahl: 2}, "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Withdraw(ctx, o.ID, WithdrawalInput{MaterialID: artMat, WarehouseID: wh, Menge: 1}, "", p.company); !errors.Is(err, errEntnahmeStatus) {
		t.Fatalf("Entwurf: %v", err)
	}
	if o, err = svc.Release(ctx, o.ID, wh, "", p.company); err != nil {
		t.Fatal(err)
	}
	art, glas := itemOf(t, o, "artikel"), itemOf(t, o, "glas")
	figures("nach Freigabe", art, 3, 0, 3)
	if glas.Entnommen != nil || glas.Offen != nil {
		t.Fatalf("Zeile ohne Material: %v %v", glas.Entnommen, glas.Offen)
	}

	// Negativfaelle: nichts gebucht.
	for name, c := range map[string]struct {
		in   WithdrawalInput
		want string
	}{
		"Menge 0":             {WithdrawalInput{ItemID: art.ID.String(), WarehouseID: wh}, errEntnahmeMenge.Error()},
		"fremde Zeile":        {WithdrawalInput{ItemID: uuid.NewString(), WarehouseID: wh, Menge: 1}, "Zeile nicht im Fertigungsauftrag"},
		"kaputte Zeilen-ID":   {WithdrawalInput{ItemID: "x", WarehouseID: wh, Menge: 1}, "Zeile nicht im Fertigungsauftrag"},
		"Zeile ohne Material": {WithdrawalInput{ItemID: glas.ID.String(), WarehouseID: wh, Menge: 1}, "Zeile hat kein Lagermaterial"},
		"anderes Material":    {WithdrawalInput{ItemID: art.ID.String(), MaterialID: looseMat, WarehouseID: wh, Menge: 1}, "Material passt nicht zur Zeile"},
		"fremdes Lager":       {WithdrawalInput{ItemID: art.ID.String(), WarehouseID: otherWh, Menge: 1}, "Lager nicht gefunden"},
		"fremdes Material":    {WithdrawalInput{MaterialID: otherMat, WarehouseID: wh, Menge: 1}, "Material nicht gefunden"},
	} {
		_, err := svc.Withdraw(ctx, o.ID, c.in, "", p.company)
		wantErr(t, name, err, c.want)
	}
	if _, err := svc.Withdraw(ctx, o.ID, WithdrawalInput{MaterialID: artMat, WarehouseID: wh, Menge: 1}, "", other.company); !errors.Is(err, ErrNotFound) {
		t.Fatalf("anderer Mandant: %v", err)
	}
	if st, q := reservation(o); stock(artMat) != 10 || st != "aktiv" || q != 3 {
		t.Fatalf("nach Fehlern: Bestand %v, Reservierung %s %v", stock(artMat), st, q)
	}

	// Teilentnahme: Reservierung sinkt, Fehlmenge bleibt 0.
	w1, err := svc.Withdraw(ctx, o.ID, WithdrawalInput{ItemID: art.ID.String(), WarehouseID: wh, Menge: 1, Notiz: " erste "}, "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	if w1.MaterialID != artMat || w1.ItemPos == nil || *w1.ItemPos != art.Pos || w1.Einheit != "Stk" || w1.ReservierungMenge != 1 || w1.Notiz != "erste" || len(w1.Hinweise) != 0 {
		t.Fatalf("Entnahme 1: %+v", w1)
	}
	var typ, grund, ref string
	var qty float64
	if err := env.PG.QueryRow(ctx, `SELECT m.movement_type, m.reason, m.reference, m.quantity::float8 FROM stock_movements m
		JOIN production_order_withdrawals w ON w.stock_movement_id = m.id WHERE w.id=$1`, w1.ID).Scan(&typ, &grund, &ref, &qty); err != nil {
		t.Fatal(err)
	}
	if typ != "out" || grund != EntnahmeGrund || ref != o.Nummer || qty != -1 {
		t.Fatalf("Lagerabgang: %s %s %s %v", typ, grund, ref, qty)
	}
	if st, q := reservation(o); st != "aktiv" || q != 2 || stock(artMat) != 9 {
		t.Fatalf("nach Entnahme 1: %s %v, Bestand %v", st, q, stock(artMat))
	}
	figures("nach Entnahme 1", item(o), 2, 1, 2)

	// Mehrentnahme: Hinweis, Reservierung verbraucht.
	w2, err := svc.Withdraw(ctx, o.ID, WithdrawalInput{ItemID: art.ID.String(), WarehouseID: wh, Menge: 3}, "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	if w2.ReservierungMenge != 2 || len(w2.Hinweise) != 1 || w2.Hinweise[0] != HintMehrentnahme {
		t.Fatalf("Entnahme 2: %v %v", w2.ReservierungMenge, w2.Hinweise)
	}
	if st, _ := reservation(o); st != "verbraucht" || stock(artMat) != 6 {
		t.Fatalf("nach Entnahme 2: %s, Bestand %v", st, stock(artMat))
	}
	it := item(o)
	if it.Reserviert == nil || *it.Reserviert != 0 || !near(it.Entnommen, 4) || !near(it.Offen, 0) || !near(it.Fehlmenge, 0) {
		t.Fatalf("nach Entnahme 2: %v %v %v %v", deref(it.Reserviert), deref(it.Entnommen), deref(it.Offen), deref(it.Fehlmenge))
	}

	// Ohne Zeile: kein Bezug zu einer Reservierung.
	w3, err := svc.Withdraw(ctx, o.ID, WithdrawalInput{MaterialID: looseMat, WarehouseID: wh, Menge: 2}, "", p.company)
	if err != nil || w3.ItemID != nil || w3.ReservierungMenge != 0 || stock(looseMat) != 3 {
		t.Fatalf("ohne Zeile: %+v %v, Bestand %v", w3, err, stock(looseMat))
	}
	list, err := svc.ListWithdrawals(ctx, o.ID, p.company)
	if err != nil || len(list) != 3 || list[0].ID != w3.ID || list[2].ID != w1.ID {
		t.Fatalf("Liste: %d %v", len(list), err)
	}
	if _, err := svc.ListWithdrawals(ctx, o.ID, other.company); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Liste anderer Mandant: %v", err)
	}

	// Storno von Entnahme 2: Gegenbuchung, Reservierung wieder aktiv mit 2.
	if _, err := svc.CancelWithdrawal(ctx, o.ID, w2.ID, "  ", "", p.company); err == nil {
		t.Fatal("Storno ohne Begruendung")
	}
	if _, err := svc.CancelWithdrawal(ctx, o.ID, w2.ID, "Zaehlfehler", "", other.company); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Storno anderer Mandant: %v", err)
	}
	if _, err := svc.CancelWithdrawal(ctx, o.ID, uuid.New(), "Zaehlfehler", "", p.company); !errors.Is(err, ErrWithdrawalNotFound) {
		t.Fatalf("Storno unbekannt: %v", err)
	}
	c2, err := svc.CancelWithdrawal(ctx, o.ID, w2.ID, " Zaehlfehler ", "", p.company)
	if err != nil || c2.StorniertAt == nil || c2.StornoReason != "Zaehlfehler" {
		t.Fatalf("Storno: %+v %v", c2, err)
	}
	if err := env.PG.QueryRow(ctx, `SELECT m.movement_type, m.reason, m.quantity::float8 FROM stock_movements m
		JOIN production_order_withdrawals w ON w.storno_movement_id = m.id WHERE w.id=$1`, w2.ID).Scan(&typ, &grund, &qty); err != nil {
		t.Fatal(err)
	}
	if typ != "in" || grund != EntnahmeStornoGrund || qty != 3 {
		t.Fatalf("Gegenbuchung: %s %s %v", typ, grund, qty)
	}
	if st, q := reservation(o); st != "aktiv" || q != 2 || stock(artMat) != 9 {
		t.Fatalf("nach Storno: %s %v, Bestand %v", st, q, stock(artMat))
	}
	figures("nach Storno", item(o), 2, 1, 2)
	if _, err := svc.CancelWithdrawal(ctx, o.ID, w2.ID, "nochmal", "", p.company); err == nil || err.Error() != "Entnahme ist bereits storniert" {
		t.Fatalf("doppeltes Storno: %v", err)
	}
	// Stornierte zaehlen nicht: 1 + 2 = Bedarf 3, keine Mehrentnahme.
	w4, err := svc.Withdraw(ctx, o.ID, WithdrawalInput{ItemID: art.ID.String(), WarehouseID: wh, Menge: 2}, "", p.company)
	if err != nil || len(w4.Hinweise) != 0 {
		t.Fatalf("nach Storno: %v %v", w4, err)
	}
	if _, err := svc.CancelWithdrawal(ctx, o.ID, w4.ID, "Probe", "", p.company); err != nil {
		t.Fatal(err)
	}

	// Auftrag storniert: Reservierung frei; Storno einer Entnahme bucht
	// zurueck, stellt die Reservierung aber nicht wieder her.
	if _, err := svc.Cancel(ctx, o.ID, "Kunde abgesprungen", "", p.company); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CancelWithdrawal(ctx, o.ID, w1.ID, "Rueckgabe", "", p.company); err != nil {
		t.Fatal(err)
	}
	if st, q := reservation(o); st != "freigegeben" || q != 2 || stock(artMat) != 10 {
		t.Fatalf("nach Auftragsstorno: %s %v, Bestand %v", st, q, stock(artMat))
	}
	if it := item(o); !near(it.Entnommen, 0) || it.Offen != nil {
		t.Fatalf("storniert: entnommen %v offen %v", deref(it.Entnommen), deref(it.Offen))
	}
	if _, err := svc.Withdraw(ctx, o.ID, WithdrawalInput{ItemID: art.ID.String(), WarehouseID: wh, Menge: 1}, "", p.company); !errors.Is(err, errEntnahmeStatus) {
		t.Fatalf("storniert: %v", err)
	}
	var booked, cancelled int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FILTER (WHERE action='withdrawal_booked'), count(*) FILTER (WHERE action='withdrawal_cancelled')
		FROM entity_change_log WHERE entity_type='production_order' AND entity_id=$1`, o.ID.String()).Scan(&booked, &cancelled); err != nil {
		t.Fatal(err)
	}
	if booked != 4 || cancelled != 3 {
		t.Fatalf("Protokoll: %d gebucht, %d storniert", booked, cancelled)
	}
}

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}
