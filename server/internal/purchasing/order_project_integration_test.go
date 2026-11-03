package purchasing

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Projekt an der Bestellung und Pruefung des Lieferanten (Backlog H.4.3c).
func TestPurchaseOrderProjectAndSupplier(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	s := seedEvidence(t, env)
	other := seedEvidence(t, env)
	svc := NewService(env.PG)
	// Zwei Leerzeichen je Seite: uuid.Parse schneidet bei 38 Zeichen ohne
	// Pruefung das erste und letzte Zeichen ab.
	project := "  " + s.project.String() + "  "
	// Eigene Nummern: der Nummernkreis jedes neuen Mandanten beginnt gleich,
	// die Nummer ist aber global eindeutig.
	po, _, err := svc.Create(ctx, PurchaseOrderCreate{Number: "PO-PRJ-" + uuid.NewString()[:13], SupplierID: " " + s.supplier + " ", ProjectID: &project}, s.company)
	if err != nil {
		t.Fatal(err)
	}
	if po.SupplierID != s.supplier || po.ProjectID == nil || *po.ProjectID != s.project.String() {
		t.Fatalf("angelegt: %+v", po)
	}
	got, _, err := svc.Get(ctx, po.ID, s.company)
	if err != nil || got.ProjectID == nil || *got.ProjectID != s.project.String() {
		t.Fatalf("Einzelabruf: %+v, %v", got, err)
	}
	list, err := svc.List(ctx, PurchaseOrderFilter{SupplierID: s.supplier}, s.company)
	found := false
	for _, o := range list {
		if o.ID == po.ID {
			found = o.ProjectID != nil && *o.ProjectID == s.project.String()
		}
		if o.ID == s.order && o.ProjectID != nil {
			t.Fatalf("ohne Projekt: %+v", o)
		}
	}
	if err != nil || !found {
		t.Fatalf("Liste: %+v, %v", list, err)
	}
	blank := " "
	if po2, _, err := svc.Create(ctx, PurchaseOrderCreate{Number: "PO-PRJ-" + uuid.NewString()[:13], SupplierID: s.supplier, ProjectID: &blank}, s.company); err != nil || po2.ProjectID != nil {
		t.Fatalf("leeres Projekt: %+v, %v", po2, err)
	}

	// Lieferant: Rolle both zaehlt, Kunde und fremder Mandant nicht.
	both, customer := s.supplier+"-both", s.supplier+"-kunde"
	for id, rolle := range map[string]string{both: "both", customer: "customer"} {
		if _, err := env.PG.Exec(ctx, `INSERT INTO contacts (id, typ, rolle, name, company_id) VALUES ($1,'org',$2,'Kontakt',$3)`, id, rolle, s.company); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := svc.Create(ctx, PurchaseOrderCreate{Number: "PO-PRJ-" + uuid.NewString()[:13], SupplierID: both}, s.company); err != nil {
		t.Fatalf("Rolle both: %v", err)
	}
	for _, sup := range []string{customer, other.supplier, "kein-kontakt"} {
		_, _, err := svc.Create(ctx, PurchaseOrderCreate{Number: "PO-PRJ-" + uuid.NewString()[:13], SupplierID: sup}, s.company)
		wantErr(t, err, "Lieferant nicht gefunden")
	}
	foreign, bad, unknown := other.project.String(), "kein-uuid", uuid.NewString()
	_, _, err = svc.Create(ctx, PurchaseOrderCreate{Number: "PO-PRJ-" + uuid.NewString()[:13], SupplierID: s.supplier, ProjectID: &foreign}, s.company)
	wantErr(t, err, "Projekt nicht gefunden")
	_, _, err = svc.Create(ctx, PurchaseOrderCreate{Number: "PO-PRJ-" + uuid.NewString()[:13], SupplierID: s.supplier, ProjectID: &unknown}, s.company)
	wantErr(t, err, "Projekt nicht gefunden")
	_, _, err = svc.Create(ctx, PurchaseOrderCreate{Number: "PO-PRJ-" + uuid.NewString()[:13], SupplierID: s.supplier, ProjectID: &bad}, s.company)
	wantErr(t, err, "Ungültige Projekt-ID")

	// Aendern: setzen, auch nach der Bestellung (ordered), und entfernen.
	ordered := "ordered"
	if _, _, err := svc.Update(ctx, s.order, PurchaseOrderUpdate{Status: &ordered}, s.company, ""); err != nil {
		t.Fatal(err)
	}
	up, _, err := svc.Update(ctx, s.order, PurchaseOrderUpdate{ProjectID: &project}, s.company, "")
	if err != nil || up.ProjectID == nil || *up.ProjectID != s.project.String() || up.Status != "ordered" {
		t.Fatalf("Projekt gesetzt: %+v, %v", up, err)
	}
	_, _, err = svc.Update(ctx, s.order, PurchaseOrderUpdate{ProjectID: &foreign}, s.company, "")
	wantErr(t, err, "Projekt nicht gefunden")
	if g, _, _ := svc.Get(ctx, s.order, s.company); g.ProjectID == nil || *g.ProjectID != s.project.String() {
		t.Fatalf("nach Fehler unveraendert: %+v", g)
	}
	empty := ""
	up, _, err = svc.Update(ctx, s.order, PurchaseOrderUpdate{ProjectID: &empty}, s.company, "")
	if err != nil || up.ProjectID != nil {
		t.Fatalf("Projekt entfernt: %+v, %v", up, err)
	}
	// Ohne Angabe bleibt das Projekt, wie es ist.
	if _, _, err := svc.Update(ctx, po.ID, PurchaseOrderUpdate{Note: &empty}, s.company, ""); err != nil {
		t.Fatal(err)
	}
	if g, _, _ := svc.Get(ctx, po.ID, s.company); g.ProjectID == nil {
		t.Fatalf("ohne Angabe: %+v", g)
	}
}
