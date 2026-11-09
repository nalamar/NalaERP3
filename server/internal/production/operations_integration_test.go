package production

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Arbeitsgaenge am Auftrag (Backlog H.2.3a): Vorschlag aus Vorlagen,
// hinzufuegen, aendern, entfernen, Protokoll, Sperre nach fertig/storniert.
func TestOrderOperations(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	p := seedPosition(t, env)
	other := seedPosition(t, env)

	wc := func(company, name string) uuid.UUID {
		t.Helper()
		x, err := svc.CreateWorkCenter(ctx, WorkCenterInput{Name: name, StundenJeTag: 8}, company)
		if err != nil {
			t.Fatal(err)
		}
		return x.ID
	}
	tpl := func(company, name string, wcID uuid.UUID, reihenfolge int, ruest, stueck float64) uuid.UUID {
		t.Helper()
		x, err := svc.CreateTemplate(ctx, TemplateInput{Name: name, WorkCenterID: wcID, Reihenfolge: reihenfolge, RuestzeitMin: ruest, MinutenJeStueck: stueck}, company)
		if err != nil {
			t.Fatal(err)
		}
		return x.ID
	}
	deactivate := func(id uuid.UUID, name string) {
		t.Helper()
		off := false
		if _, err := svc.UpdateWorkCenter(ctx, id, WorkCenterInput{Name: name, StundenJeTag: 8}, &off, p.company); err != nil {
			t.Fatal(err)
		}
	}
	zuschnitt, montage, alt := wc(p.company, "Zuschnitt"), wc(p.company, "Montage"), wc(p.company, "Alt")
	fremd := wc(other.company, "Fremd")
	tZu := tpl(p.company, "Zuschnitt", zuschnitt, 1, 30, 10)
	tpl(p.company, "Verglasung", montage, 2, 0, 15)
	tOff := tpl(p.company, "Ausgemustert", zuschnitt, 3, 0, 0)
	tpl(p.company, "Auf altem Platz", alt, 4, 0, 0)
	tpl(other.company, "Fremde Vorlage", fremd, 1, 0, 0)
	off := false
	if _, err := svc.UpdateTemplate(ctx, tOff, TemplateInput{Name: "Ausgemustert", WorkCenterID: zuschnitt, Reihenfolge: 3}, &off, p.company); err != nil {
		t.Fatal(err)
	}
	deactivate(alt, "Alt")

	order, err := svc.Create(ctx, OrderInput{ProjectID: p.project, ElevationID: p.elevation, SingleElevationID: p.variant,
		Stueckzahl: 4, Liefertermin: dayPtr("2026-12-02")}, "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	ops := func() []Operation {
		t.Helper()
		out, err := svc.ListOperations(ctx, order.ID, p.company)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	summary := func(list []Operation) string {
		parts := []string{}
		for _, x := range list {
			w := "-"
			if x.GeplanteWoche != nil {
				w = x.GeplanteWoche.Format("2006-01-02")
			}
			parts = append(parts, x.Name+"@"+w+"/"+strings.Join(x.Hinweise, "+"))
		}
		return strings.Join(parts, ",")
	}

	// Vorschlag: nur aktive Vorlagen mit aktivem Arbeitsplatz des
	// Mandanten, Wochen rueckwaerts vom Liefertermin.
	got, err := svc.SuggestOperations(ctx, order.ID, "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	if s := summary(got); s != "Zuschnitt@2026-11-16/,Verglasung@2026-11-23/" {
		t.Fatalf("Vorschlag: %s", s)
	}
	if got[0].PlanMinuten != 70 || got[1].PlanMinuten != 60 || got[0].TemplateID == nil || *got[0].TemplateID != tZu || got[0].Pos != 1 || got[1].Pos != 2 {
		t.Fatalf("Vorschlag Werte: %+v", got)
	}
	_, err = svc.SuggestOperations(ctx, order.ID, "", p.company)
	wantErr(t, "zweiter Vorschlag", err, "Auftrag hat bereits Arbeitsgänge")
	if n := len(ops()); n != 2 {
		t.Fatalf("zweiter Vorschlag hat angelegt: %d", n)
	}

	// Hinzufuegen
	add, err := svc.AddOperation(ctx, order.ID, OperationInput{Name: " Endkontrolle ", WorkCenterID: montage, MinutenJeStueck: 2,
		GeplanteWoche: dayPtr("2026-12-09")}, "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	if add.Pos != 3 || add.Name != "Endkontrolle" || add.GeplanteWoche.Format("2006-01-02") != "2026-12-07" ||
		add.PlanMinuten != 8 || add.TemplateID != nil || strings.Join(add.Hinweise, ",") != HintNachLiefertermin {
		t.Fatalf("hinzugefügt: %+v", add)
	}
	for name, tc := range map[string]struct {
		in   OperationInput
		want string
	}{
		"fremder Arbeitsplatz":     {OperationInput{Name: "x", WorkCenterID: fremd}, "Arbeitsplatz nicht gefunden"},
		"unbekannter Arbeitsplatz": {OperationInput{Name: "x", WorkCenterID: uuid.New()}, "Arbeitsplatz nicht gefunden"},
		"deaktivierter Platz":      {OperationInput{Name: "x", WorkCenterID: alt}, "Arbeitsplatz ist deaktiviert"},
		"ohne Name":                {OperationInput{Name: " ", WorkCenterID: montage}, "Name des Arbeitsgangs erforderlich"},
	} {
		_, err := svc.AddOperation(ctx, order.ID, tc.in, "", p.company)
		wantErr(t, name, err, tc.want)
	}
	_, err = svc.AddOperation(ctx, order.ID, OperationInput{Name: "x", WorkCenterID: fremd}, "", other.company)
	wantErr(t, "Auftrag im fremden Mandanten", err, "Fertigungsauftrag nicht gefunden")
	if n := len(ops()); n != 3 {
		t.Fatalf("abgelehnte Arbeitsgänge angelegt: %d", n)
	}

	// Aendern: Arbeitsplatz Montage deaktivieren - wer schon dort ist,
	// bleibt aenderbar; wechseln dorthin nicht.
	deactivate(montage, "Montage")
	list := ops()
	glas, zu := list[1], list[0]
	ch, err := svc.UpdateOperation(ctx, order.ID, glas.ID, OperationInput{Name: "Verglasung", WorkCenterID: montage, RuestzeitMin: 5, MinutenJeStueck: 15}, "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	if ch.GeplanteWoche != nil || ch.PlanMinuten != 65 || ch.TemplateID == nil {
		t.Fatalf("geändert: %+v", ch)
	}
	_, err = svc.UpdateOperation(ctx, order.ID, zu.ID, OperationInput{Name: "Zuschnitt", WorkCenterID: montage}, "", p.company)
	wantErr(t, "Wechsel auf deaktivierten Platz", err, "Arbeitsplatz ist deaktiviert")
	_, err = svc.UpdateOperation(ctx, order.ID, uuid.New(), OperationInput{Name: "x", WorkCenterID: zuschnitt}, "", p.company)
	wantErr(t, "unbekannter Arbeitsgang", err, "Arbeitsgang nicht gefunden")
	_, err = svc.UpdateOperation(ctx, order.ID, zu.ID, OperationInput{Name: "x", WorkCenterID: zuschnitt}, "", other.company)
	wantErr(t, "fremder Mandant", err, "Fertigungsauftrag nicht gefunden")
	if s := summary(ops()); s != "Zuschnitt@2026-11-16/,Verglasung@-/,Endkontrolle@2026-12-07/"+HintNachLiefertermin {
		t.Fatalf("nach Änderungen: %s", s)
	}

	// Arbeitsgang eines anderen Auftrags ist hier unbekannt.
	second, err := svc.Create(ctx, OrderInput{ProjectID: p.project, ElevationID: p.elevation, SingleElevationID: p.variant, Stueckzahl: 1}, "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	err = svc.RemoveOperation(ctx, second.ID, zu.ID, "", p.company)
	wantErr(t, "Arbeitsgang am anderen Auftrag", err, "Arbeitsgang nicht gefunden")

	// Entfernen
	if err := svc.RemoveOperation(ctx, order.ID, add.ID, "", p.company); err != nil {
		t.Fatal(err)
	}
	wantErr(t, "zweimal entfernen", svc.RemoveOperation(ctx, order.ID, add.ID, "", p.company), "Arbeitsgang nicht gefunden")
	if s := summary(ops()); s != "Zuschnitt@2026-11-16/,Verglasung@-/" {
		t.Fatalf("nach Entfernen: %s", s)
	}

	// Einzelansicht zeigt die Arbeitsgaenge.
	full, err := svc.Get(ctx, order.ID, p.company)
	if err != nil {
		t.Fatal(err)
	}
	if s := summary(full.Arbeitsgaenge); s != "Zuschnitt@2026-11-16/,Verglasung@-/" {
		t.Fatalf("Get: %s", s)
	}

	// Protokoll
	rows, err := env.PG.Query(ctx, `SELECT action FROM entity_change_log WHERE entity_type='production_order' AND entity_id=$1 ORDER BY created_at, id`, order.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	actions := map[string]int{}
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatal(err)
		}
		actions[a]++
	}
	rows.Close()
	if actions["operations_suggested"] != 1 || actions["operation_added"] != 1 || actions["operation_changed"] != 1 || actions["operation_removed"] != 1 {
		t.Fatalf("Protokoll: %v", actions)
	}

	// Ohne Liefertermin bleibt der Vorschlag ungeplant; ohne Vorlagen
	// kein Vorschlag.
	ws, err := svc.SuggestOperations(ctx, second.ID, "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	if s := summary(ws); s != "Zuschnitt@-/" {
		t.Fatalf("ohne Liefertermin: %s", s)
	}
	// Luecke in der Mitte: der naechste bekommt die hoechste Pos + 1.
	mid, err := svc.AddOperation(ctx, second.ID, OperationInput{Name: "Mitte", WorkCenterID: zuschnitt}, "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RemoveOperation(ctx, second.ID, ws[0].ID, "", p.company); err != nil {
		t.Fatal(err)
	}
	if last, err := svc.AddOperation(ctx, second.ID, OperationInput{Name: "Ende", WorkCenterID: zuschnitt}, "", p.company); err != nil || mid.Pos != 2 || last.Pos != 3 {
		t.Fatalf("Pos nach Lücke: Mitte %d, Ende %+v, %v", mid.Pos, last, err)
	}
	third, err := svc.Create(ctx, OrderInput{ProjectID: other.project, ElevationID: other.elevation, SingleElevationID: other.variant, Stueckzahl: 1}, "", other.company)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateWorkCenter(ctx, fremd, WorkCenterInput{Name: "Fremd", StundenJeTag: 8}, &off, other.company); err != nil {
		t.Fatal(err)
	}
	_, err = svc.SuggestOperations(ctx, third.ID, "", other.company)
	wantErr(t, "ohne aktive Vorlagen", err, "Keine aktiven Arbeitsgangvorlagen vorhanden")

	// Nach Storno und nach "fertig" nichts mehr aenderbar.
	if _, err := svc.Cancel(ctx, order.ID, "Maß falsch", "", p.company); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE production_orders SET status='fertig', position_nummer='3', inhalt_hash='h',
		freigegeben_at=now(), begonnen_at=now(), fertig_at=now() WHERE id=$1`, second.ID); err != nil {
		t.Fatal(err)
	}
	const locked = "Auftrag ist abgeschlossen oder storniert, Arbeitsgänge nicht änderbar"
	for _, id := range []uuid.UUID{order.ID, second.ID} {
		opID := zu.ID
		if id == second.ID {
			opID = ws[0].ID
		}
		_, err = svc.AddOperation(ctx, id, OperationInput{Name: "x", WorkCenterID: zuschnitt}, "", p.company)
		wantErr(t, "hinzufügen gesperrt", err, locked)
		_, err = svc.UpdateOperation(ctx, id, opID, OperationInput{Name: "x", WorkCenterID: zuschnitt}, "", p.company)
		wantErr(t, "ändern gesperrt", err, locked)
		wantErr(t, "entfernen gesperrt", svc.RemoveOperation(ctx, id, opID, "", p.company), locked)
		_, err = svc.SuggestOperations(ctx, id, "", p.company)
		wantErr(t, "Vorschlag gesperrt", err, locked)
	}
	if s := summary(ops()); s != "Zuschnitt@2026-11-16/,Verglasung@-/" {
		t.Fatalf("gesperrter Auftrag verändert: %s", s)
	}
}
