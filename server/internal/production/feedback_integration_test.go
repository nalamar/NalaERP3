package production

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Rueckmeldung je Arbeitsgang (Backlog H.3.5b): melden nur in Arbeit,
// zuruecknehmen mit Begruendung, neu melden, Entfernen gesperrt,
// Mandant, Protokoll.
func TestOperationFeedback(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	p := seedPosition(t, env)
	other := seedPosition(t, env)
	actor := testutil.SeedAuthUser(t, env, "rm-"+uuid.NewString()[:8]+"@example.com", "Passwort-123!", "production")

	wc, err := svc.CreateWorkCenter(ctx, WorkCenterInput{Name: "Zuschnitt", StundenJeTag: 8}, p.company)
	if err != nil {
		t.Fatal(err)
	}
	order, err := svc.Create(ctx, OrderInput{ProjectID: p.project, ElevationID: p.elevation, SingleElevationID: p.variant, Stueckzahl: 4}, "", p.company)
	if err != nil {
		t.Fatal(err)
	}
	add := func(name string) uuid.UUID {
		t.Helper()
		x, err := svc.AddOperation(ctx, order.ID, OperationInput{Name: name, WorkCenterID: wc.ID, RuestzeitMin: 30, MinutenJeStueck: 10}, "", p.company)
		if err != nil {
			t.Fatal(err)
		}
		return x.ID
	}
	op1, op2 := add("Zuschnitt"), add("Verglasung")
	ist := func(v float64) FeedbackInput { return FeedbackInput{IstMinuten: &v} }
	report := func(op uuid.UUID, in FeedbackInput, company string) (*Operation, error) {
		return svc.ReportOperationDone(ctx, order.ID, op, in, actor, company)
	}
	feedbackRows := func(op uuid.UUID) int {
		t.Helper()
		var n int
		if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM production_operation_feedback WHERE operation_id=$1`, op).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// Entwurf und freigegeben: noch nicht.
	if _, err := report(op1, FeedbackInput{}, p.company); !errors.Is(err, errRueckmeldungStatus) {
		t.Fatalf("Entwurf: %v", err)
	}
	if _, err := svc.Release(ctx, order.ID, "", "", p.company); err != nil {
		t.Fatal(err)
	}
	if _, err := report(op1, FeedbackInput{}, p.company); !errors.Is(err, errRueckmeldungStatus) {
		t.Fatalf("freigegeben: %v", err)
	}
	if _, err := svc.Start(ctx, order.ID, "", p.company); err != nil {
		t.Fatal(err)
	}

	// Fehlerfaelle ohne Wirkung.
	if _, err := report(op1, ist(0), p.company); !errors.Is(err, errRueckmeldungIst) {
		t.Fatalf("Ist 0: %v", err)
	}
	if _, err := report(uuid.New(), FeedbackInput{}, p.company); !errors.Is(err, ErrOperationNotFound) {
		t.Fatalf("unbekannter Arbeitsgang: %v", err)
	}
	if _, err := report(op1, FeedbackInput{}, other.company); !errors.Is(err, ErrNotFound) {
		t.Fatalf("anderer Mandant: %v", err)
	}
	if _, err := svc.RevertOperationDone(ctx, order.ID, op1, "x", actor, other.company); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Ruecknahme anderer Mandant: %v", err)
	}
	if n := feedbackRows(op1); n != 0 {
		t.Fatalf("nach Fehlern %d Rueckmeldungen", n)
	}

	// Melden mit und ohne Ist-Zeit.
	x, err := report(op1, FeedbackInput{IstMinuten: fPtr(95.456), Notiz: " Rest morgen "}, p.company)
	if err != nil || !x.Erledigt || x.ErledigtAt == nil || x.IstMinuten == nil || *x.IstMinuten != 95.46 || x.RueckmeldungNotiz != "Rest morgen" {
		t.Fatalf("gemeldet: %+v %v", x, err)
	}
	if _, err := report(op1, FeedbackInput{}, p.company); !errors.Is(err, errOperationAlreadyDone) {
		t.Fatalf("doppelt: %v", err)
	}
	if x, err = report(op2, FeedbackInput{}, p.company); err != nil || !x.Erledigt || x.IstMinuten != nil {
		t.Fatalf("ohne Ist: %+v %v", x, err)
	}
	list, err := svc.ListOperations(ctx, order.ID, p.company)
	if err != nil || len(list) != 2 || !list[0].Erledigt || !list[1].Erledigt || *list[0].IstMinuten != 95.46 {
		t.Fatalf("Liste: %+v %v", list, err)
	}
	var by *string
	if err := env.PG.QueryRow(ctx, `SELECT gemeldet_by_user_id FROM production_operation_feedback WHERE operation_id=$1`, op1).Scan(&by); err != nil || by == nil || *by != actor {
		t.Fatalf("gemeldet von: %v %v", by, err)
	}

	// Zuruecknehmen.
	if _, err := svc.RevertOperationDone(ctx, order.ID, op2, "  ", actor, p.company); err == nil {
		t.Fatal("Ruecknahme ohne Begruendung angenommen")
	}
	if x, err = svc.RevertOperationDone(ctx, order.ID, op2, " versehentlich ", actor, p.company); err != nil || x.Erledigt || x.ErledigtAt != nil {
		t.Fatalf("zurueckgenommen: %+v %v", x, err)
	}
	if _, err := svc.RevertOperationDone(ctx, order.ID, op2, "nochmal", actor, p.company); !errors.Is(err, errOperationNotDone) {
		t.Fatalf("doppelt zurueckgenommen: %v", err)
	}
	var reason string
	var revBy *string
	if err := env.PG.QueryRow(ctx, `SELECT ruecknahme_reason, zurueckgenommen_by_user_id FROM production_operation_feedback
		WHERE operation_id=$1 AND zurueckgenommen_at IS NOT NULL`, op2).Scan(&reason, &revBy); err != nil || reason != "versehentlich" || revBy == nil || *revBy != actor {
		t.Fatalf("Ruecknahme: %q %v %v", reason, revBy, err)
	}
	if x, err = report(op2, ist(40), p.company); err != nil || !x.Erledigt || *x.IstMinuten != 40 {
		t.Fatalf("neu gemeldet: %+v %v", x, err)
	}
	if n := feedbackRows(op2); n != 2 {
		t.Fatalf("Rueckmeldungen op2: %d", n)
	}

	// Entfernen: mit Rueckmeldung gesperrt, ohne weiterhin moeglich.
	for _, op := range []uuid.UUID{op1, op2} {
		if err := svc.RemoveOperation(ctx, order.ID, op, "", p.company); !errors.Is(err, errOperationHasFeedback) {
			t.Fatalf("entfernen mit Rueckmeldung: %v", err)
		}
	}
	if err := svc.RemoveOperation(ctx, order.ID, add("Endkontrolle"), "", p.company); err != nil {
		t.Fatalf("entfernen ohne Rueckmeldung: %v", err)
	}

	// Soll/Ist: je 30 + 10 x 4 = 70 Soll; op1 95,46 Ist, op2 40 Ist.
	si, err := svc.OrderSollIst(ctx, order.ID, p.company)
	want := SollIst{Arbeitsgaenge: 2, Erledigt: 2, SollMin: 140, SollErledigtMin: 140, MitIst: 2, SollMitIstMin: 140, IstMin: 135.46}
	if err != nil || *si != want {
		t.Fatalf("Soll/Ist: %+v %v", si, err)
	}
	if _, err := svc.OrderSollIst(ctx, order.ID, other.company); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Soll/Ist anderer Mandant: %v", err)
	}

	var done, reverted int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FILTER (WHERE action='operation_done'), count(*) FILTER (WHERE action='operation_done_reverted')
		FROM entity_change_log WHERE entity_type='production_order' AND entity_id=$1`, order.ID.String()).Scan(&done, &reverted); err != nil || done != 3 || reverted != 1 {
		t.Fatalf("Protokoll: %d gemeldet, %d zurueckgenommen, %v", done, reverted, err)
	}

	// Nach "fertig" bleibt der Stand.
	if _, err := svc.Finish(ctx, order.ID, "", p.company); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RevertOperationDone(ctx, order.ID, op1, "x", actor, p.company); !errors.Is(err, errRueckmeldungStatus) {
		t.Fatalf("Ruecknahme nach fertig: %v", err)
	}
	if list, err = svc.ListOperations(ctx, order.ID, p.company); err != nil || !list[0].Erledigt || !list[1].Erledigt {
		t.Fatalf("Liste nach fertig: %+v %v", list, err)
	}
}
