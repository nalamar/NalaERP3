package purchasing

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Lieferpapiere an der Bestellung im Service (Backlog H.4.3a).
func TestDeliveryPapers(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	s := seedEvidence(t, env)
	other := seedEvidence(t, env)
	svc := NewPaperService(env.PG, env.Mongo, env.Cfg.MongoDB)
	pdf := []byte("%PDF-1.4\nLeistungserklaerung\n")

	// Zweite Bestellung im selben Mandanten: ihre Position passt nicht.
	order2, item2 := s.order+"-2", s.item+"-2"
	for _, q := range []string{
		`INSERT INTO purchase_orders (id, supplier_id, number, currency, status, company_id) VALUES ('` + order2 + `','` + s.supplier + `','` + order2 + `','EUR','ordered','` + s.company + `')`,
		`INSERT INTO purchase_order_items (id, order_id, position, material_id, qty, uom, unit_price, currency) VALUES ('` + item2 + `','` + order2 + `',1,'` + s.material + `',1,'Stk',0,'EUR')`,
	} {
		if _, err := env.PG.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}

	item := " " + s.item + " "
	p, err := svc.Upload(ctx, s.order, DeliveryPaperInput{Art: " Leistungserklaerung ", ItemID: &item, Charge: " CH-42 ", Notiz: " Lieferung 1 "}, bytes.NewReader(pdf), `C:\Scans\dop.pdf`, "", s.company)
	if err != nil {
		t.Fatal(err)
	}
	if p.Art != PapierLeistungserklaerung || p.ItemID == nil || *p.ItemID != s.item || p.Charge != "CH-42" || p.Notiz != "Lieferung 1" ||
		p.Filename != "dop.pdf" || p.ContentType != "application/pdf" || p.Length != int64(len(pdf)) || p.OrderID != s.order {
		t.Fatalf("hochgeladen: %+v", p)
	}
	// Ohne Position (leer zaehlt wie ohne) gilt das Papier fuer die Bestellung.
	empty := " "
	whole, err := svc.Upload(ctx, s.order, DeliveryPaperInput{Art: "zeugnis_3_1", ItemID: &empty}, bytes.NewReader(pdf), "", "", s.company)
	if err != nil {
		t.Fatal(err)
	}
	if whole.ItemID != nil || whole.Filename != "nachweis" {
		t.Fatalf("ohne Position: %+v", whole)
	}

	// Negativfaelle
	_, err = svc.Upload(ctx, s.order, DeliveryPaperInput{Art: "rechnung"}, bytes.NewReader(pdf), "x.pdf", "", s.company)
	wantErr(t, err, "Ungültige Art des Lieferpapiers")
	_, err = svc.Upload(ctx, s.order, DeliveryPaperInput{Art: "lieferschein", ItemID: &item2}, bytes.NewReader(pdf), "x.pdf", "", s.company)
	wantErr(t, err, "Position nicht gefunden")
	_, err = svc.Upload(ctx, s.order, DeliveryPaperInput{Art: "lieferschein"}, bytes.NewReader(pdf), "x.pdf", "", other.company)
	wantErr(t, err, "Bestellung nicht gefunden")
	_, err = svc.Upload(ctx, "keine-bestellung", DeliveryPaperInput{Art: "lieferschein"}, bytes.NewReader(pdf), "x.pdf", "", s.company)
	wantErr(t, err, "Bestellung nicht gefunden")
	_, err = svc.Upload(ctx, s.order, DeliveryPaperInput{Art: "lieferschein"}, strings.NewReader("nur Text"), "x.txt", "", s.company)
	if !errors.Is(err, ErrEvidenceDocumentType) {
		t.Fatalf("Text: %v", err)
	}
	_, err = svc.Upload(ctx, s.order, DeliveryPaperInput{Art: "lieferschein"}, strings.NewReader(""), "x.pdf", "", s.company)
	wantErr(t, err, "Datei erforderlich")
	big := append([]byte("%PDF-1.4\n"), make([]byte, MaxEvidenceDocumentBytes)...)
	_, err = svc.Upload(ctx, s.order, DeliveryPaperInput{Art: "lieferschein"}, bytes.NewReader(big), "x.pdf", "", s.company)
	if !errors.Is(err, ErrEvidenceTooLarge) {
		t.Fatalf("zu gross: %v", err)
	}
	_, err = NewPaperService(env.PG, nil, "").Upload(ctx, s.order, DeliveryPaperInput{Art: "lieferschein"}, bytes.NewReader(pdf), "x.pdf", "", s.company)
	wantErr(t, err, "MongoDB nicht konfiguriert")

	list, err := svc.List(ctx, s.order, false, s.company)
	if err != nil || len(list) != 2 || list[0].ID != p.ID || list[1].ID != whole.ID {
		t.Fatalf("Liste: %+v, %v", list, err)
	}
	if l2, err := svc.List(ctx, order2, false, s.company); err != nil || len(l2) != 0 {
		t.Fatalf("andere Bestellung: %+v, %v", l2, err)
	}
	_, err = svc.List(ctx, s.order, false, other.company)
	wantErr(t, err, "Bestellung nicht gefunden")

	rc, meta, err := svc.Open(ctx, s.order, p.ID, s.company)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, pdf) || meta.Filename != "dop.pdf" {
		t.Fatalf("Download: %q, %+v", got, meta)
	}
	_, _, err = svc.Open(ctx, order2, p.ID, s.company)
	wantErr(t, err, "Lieferpapier nicht gefunden")
	_, _, err = svc.Open(ctx, s.order, uuid.New(), s.company)
	wantErr(t, err, "Lieferpapier nicht gefunden")
	_, _, err = svc.Open(ctx, s.order, p.ID, other.company)
	wantErr(t, err, "Bestellung nicht gefunden")

	// Storno
	_, err = svc.Storno(ctx, s.order, p.ID, " ", "", s.company)
	wantErr(t, err, "Storno-Begründung erforderlich")
	_, err = svc.Storno(ctx, order2, p.ID, "falsch", "", s.company)
	wantErr(t, err, "Lieferpapier nicht gefunden")
	st, err := svc.Storno(ctx, s.order, p.ID, " falsche Charge ", "", s.company)
	if err != nil {
		t.Fatal(err)
	}
	if st.StorniertAt == nil || st.StornoReason != "falsche Charge" || st.Charge != "CH-42" {
		t.Fatalf("storniert: %+v", st)
	}
	_, err = svc.Storno(ctx, s.order, p.ID, "nochmal", "", s.company)
	wantErr(t, err, "Lieferpapier ist bereits storniert")
	if l, _ := svc.List(ctx, s.order, false, s.company); len(l) != 1 || l[0].ID != whole.ID {
		t.Fatalf("ohne stornierte: %+v", l)
	}
	if l, _ := svc.List(ctx, s.order, true, s.company); len(l) != 2 {
		t.Fatalf("mit stornierten: %+v", l)
	}
	// Die Datei eines stornierten Papiers bleibt lesbar.
	if rc, _, err := svc.Open(ctx, s.order, p.ID, s.company); err != nil {
		t.Fatalf("nach Storno: %v", err)
	} else {
		rc.Close()
	}

	// Stornierte Bestellung: nichts mehr hinzu, Bestand bleibt lesbar.
	if _, err := env.PG.Exec(ctx, `UPDATE purchase_orders SET status='canceled' WHERE id=$1`, s.order); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Upload(ctx, s.order, DeliveryPaperInput{Art: "lieferschein"}, bytes.NewReader(pdf), "x.pdf", "", s.company)
	wantErr(t, err, "Bestellung ist storniert")
	if l, err := svc.List(ctx, s.order, true, s.company); err != nil || len(l) != 2 {
		t.Fatalf("stornierte Bestellung: %+v, %v", l, err)
	}
	// Andere Status als canceled nehmen Papiere an (hier ordered).
	if _, err := svc.Upload(ctx, order2, DeliveryPaperInput{Art: "lieferschein"}, bytes.NewReader(pdf), "x.pdf", "", s.company); err != nil {
		t.Fatalf("bestellt: %v", err)
	}
}
