package sales

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CreateFromQuote() begint fuer alles ausser der Mandantenpruefung sofort
// eine echte DB-Transaktion (s.pg.Begin) und ist daher ohne Postgres nicht
// direkt unit-testbar (siehe docs/adr/0001-baseline.md, kein DB-Mock im
// Repo). Testbar ohne DB sind: der companyID-Pflichtcheck (steht bewusst vor
// s.pg.Begin), die von CreateFromQuote verwendete reine Steuersatz-Funktion
// sowie quoteHasOpenApprovalRework - die zentrale fachliche Sperre "Angebote
// mit offener Freigabe-Nacharbeit duerfen nicht in einen Auftrag ueberfuehrt
// werden" (server/internal/sales/service.go:166-172). Letztere nutzt bewusst
// die schmale Schnittstelle quoteApprovalReworkQuerier (nur QueryRow) statt
// pgx.Tx, wodurch sie sich mit einem einfachen Fake statt einer echten
// Transaktion pruefen laesst.

func TestCreateFromQuoteRejectsMissingCompanyID(t *testing.T) {
	s := &Service{}

	order, err := s.CreateFromQuote(context.Background(), uuid.New(), "")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if order != nil {
		t.Fatalf("expected nil order, got %#v", order)
	}
	if err.Error() != "Mandant erforderlich" {
		t.Fatalf("expected Mandant erforderlich, got %q", err.Error())
	}
}

type fakeScanRow struct {
	val bool
	err error
}

func (r fakeScanRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	ptr, ok := dest[0].(*bool)
	if !ok {
		return errors.New("unexpected scan destination type")
	}
	*ptr = r.val
	return nil
}

type fakeQuoteApprovalReworkQuerier struct {
	row pgx.Row
}

func (q fakeQuoteApprovalReworkQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return q.row
}

func TestQuoteHasOpenApprovalReworkReturnsTrueWhenQueryFindsOpenRework(t *testing.T) {
	q := fakeQuoteApprovalReworkQuerier{row: fakeScanRow{val: true}}

	got, err := quoteHasOpenApprovalRework(context.Background(), q, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Fatal("expected hasOpenApprovalRework=true")
	}
}

func TestQuoteHasOpenApprovalReworkReturnsFalseWhenNoOpenRework(t *testing.T) {
	q := fakeQuoteApprovalReworkQuerier{row: fakeScanRow{val: false}}

	got, err := quoteHasOpenApprovalRework(context.Background(), q, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Fatal("expected hasOpenApprovalRework=false")
	}
}

func TestQuoteHasOpenApprovalReworkPropagatesQueryError(t *testing.T) {
	q := fakeQuoteApprovalReworkQuerier{row: fakeScanRow{err: errors.New("db down")}}

	_, err := quoteHasOpenApprovalRework(context.Background(), q, uuid.New())
	if err == nil {
		t.Fatal("expected error to be propagated, got nil")
	}
}

func TestSalesOrderTaxRateKnownAndUnknownCodes(t *testing.T) {
	cases := map[string]float64{
		"DE19": 0.19,
		"DE7":  0.07,
		"":     0,
		"XX":   0,
	}
	for code, want := range cases {
		if got := taxRate(code); got != want {
			t.Errorf("taxRate(%q) = %v, want %v", code, got, want)
		}
	}
}

// UpdateStatus() prueft die Statusgueltigkeit (isStatus) VOR jedem
// DB-Zugriff (server/internal/sales/service.go:598-603) - damit ist dieser
// Negativfall ueber den echten exportierten Einstiegspunkt testbar, ohne
// eine Transaktion zu beruehren.
func TestUpdateStatusRejectsUnknownStatus(t *testing.T) {
	s := &Service{}

	order, err := s.UpdateStatus(context.Background(), uuid.New(), "nicht-existent", "company-1", "user-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if order != nil {
		t.Fatalf("expected nil order, got %#v", order)
	}
	if err.Error() != "ungültiger Auftragsstatus" {
		t.Fatalf("expected 'ungültiger Auftragsstatus', got %q", err.Error())
	}
}

func TestIsStatusKnownAndUnknownValues(t *testing.T) {
	for _, s := range Statuses() {
		if !isStatus(s) {
			t.Errorf("isStatus(%q) = false, want true (aus Statuses())", s)
		}
	}
	for _, s := range []string{"", "OPEN", "unknown", "geloescht"} {
		if isStatus(s) {
			t.Errorf("isStatus(%q) = true, want false", s)
		}
	}
}

// validateStatusTransition ist die eigentliche Zustandsmaschine hinter
// UpdateStatus (server/internal/sales/service.go:803-830) und vollstaendig
// rein - jede erlaubte/verbotene Kombination laesst sich ohne DB pruefen.
func TestValidateStatusTransition(t *testing.T) {
	cases := []struct {
		name       string
		current    string
		next       string
		hasInvoice bool
		wantErr    bool
	}{
		{"gleicher Status ist immer erlaubt (auch terminal)", "completed", "completed", false, false},
		{"open -> released", "open", "released", false, false},
		{"open -> canceled", "open", "canceled", false, false},
		{"open -> invoiced ist unzulaessig", "open", "invoiced", true, true},
		{"open -> completed ist unzulaessig", "open", "completed", true, true},
		{"released -> open", "released", "open", false, false},
		{"released -> canceled", "released", "canceled", false, false},
		{"released -> invoiced erfordert Rechnung", "released", "invoiced", false, true},
		{"released -> invoiced mit Rechnung erlaubt", "released", "invoiced", true, false},
		{"released -> completed erfordert Rechnung", "released", "completed", false, true},
		{"released -> completed mit Rechnung erlaubt", "released", "completed", true, false},
		{"invoiced -> completed erfordert Rechnung", "invoiced", "completed", false, true},
		{"invoiced -> completed mit Rechnung erlaubt", "invoiced", "completed", true, false},
		{"invoiced -> open ist unzulaessig", "invoiced", "open", true, true},
		{"invoiced -> canceled ist unzulaessig (auch mit Rechnung)", "invoiced", "canceled", true, true},
		{"completed ist terminal", "completed", "open", false, true},
		{"canceled ist terminal", "canceled", "released", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStatusTransition(tc.current, tc.next, tc.hasInvoice)
			if tc.wantErr && err == nil {
				t.Fatalf("%s -> %s (hasInvoice=%v): expected error, got nil", tc.current, tc.next, tc.hasInvoice)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("%s -> %s (hasInvoice=%v): expected no error, got %v", tc.current, tc.next, tc.hasInvoice, err)
			}
		})
	}
}

func TestValidateStatusTransitionTerminalStatesUseDedicatedMessage(t *testing.T) {
	for _, current := range []string{"completed", "canceled"} {
		err := validateStatusTransition(current, "open", false)
		if err == nil {
			t.Fatalf("current=%q: expected error, got nil", current)
		}
		if err.Error() != "abgeschlossene oder stornierte Aufträge können nicht erneut umgestellt werden" {
			t.Fatalf("current=%q: expected dedicated terminal-state message, got %q", current, err.Error())
		}
	}
}

// ConvertToInvoice() prueft das Vorhandensein des ARService VOR jedem
// DB-Zugriff (server/internal/sales/service.go:629-631) - einziger dort ohne
// DB testbarer Negativfall.
func TestConvertToInvoiceRequiresARService(t *testing.T) {
	s := &Service{}

	result, err := s.ConvertToInvoice(context.Background(), uuid.New(), nil, ConvertToInvoiceInput{}, "company-1", "user-1")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}
	if err.Error() != "invoice service fehlt" {
		t.Fatalf("expected 'invoice service fehlt', got %q", err.Error())
	}
}

// selectInvoiceQuantities steuert die Teilfakturierung (server/internal/sales/service.go:991-1025)
// und ist vollstaendig rein - keiner dieser Faelle ist bisher irgendwo im
// Repo getestet (weder Unit noch Integration, siehe docs/state.md).
func TestSelectInvoiceQuantitiesDefaultsToFullRemainingWhenNoSelectionGiven(t *testing.T) {
	items := []SalesOrderItem{{ID: "a"}, {ID: "b"}}
	remaining := map[string]float64{"a": 5, "b": 0}

	out, err := selectInvoiceQuantities(items, remaining, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["a"] != 5 || out["b"] != 0 {
		t.Fatalf("expected full remaining quantities, got %#v", out)
	}
}

func TestSelectInvoiceQuantitiesRejectsEmptyItemID(t *testing.T) {
	items := []SalesOrderItem{{ID: "a"}}
	remaining := map[string]float64{"a": 5}

	_, err := selectInvoiceQuantities(items, remaining, []ConvertToInvoiceItemInput{{SalesOrderItemID: "  ", Qty: 1}})
	if err == nil || err.Error() != "Auftragspositions-ID erforderlich" {
		t.Fatalf("expected 'Auftragspositions-ID erforderlich', got %v", err)
	}
}

func TestSelectInvoiceQuantitiesRejectsUnknownItemID(t *testing.T) {
	items := []SalesOrderItem{{ID: "a"}}
	remaining := map[string]float64{"a": 5}

	_, err := selectInvoiceQuantities(items, remaining, []ConvertToInvoiceItemInput{{SalesOrderItemID: "unbekannt", Qty: 1}})
	if err == nil || err.Error() != "Auftragsposition nicht gefunden" {
		t.Fatalf("expected 'Auftragsposition nicht gefunden', got %v", err)
	}
}

func TestSelectInvoiceQuantitiesRejectsNonPositiveQty(t *testing.T) {
	items := []SalesOrderItem{{ID: "a"}}
	remaining := map[string]float64{"a": 5}

	for _, qty := range []float64{0, -1} {
		_, err := selectInvoiceQuantities(items, remaining, []ConvertToInvoiceItemInput{{SalesOrderItemID: "a", Qty: qty}})
		if err == nil || err.Error() != "Teilfaktura-Menge muss größer als 0 sein" {
			t.Fatalf("qty=%v: expected 'Teilfaktura-Menge muss größer als 0 sein', got %v", qty, err)
		}
	}
}

func TestSelectInvoiceQuantitiesRejectsFullyInvoicedItem(t *testing.T) {
	items := []SalesOrderItem{{ID: "a"}}
	remaining := map[string]float64{"a": 0}

	_, err := selectInvoiceQuantities(items, remaining, []ConvertToInvoiceItemInput{{SalesOrderItemID: "a", Qty: 1}})
	if err == nil || err.Error() != "Auftragsposition ist bereits vollständig fakturiert" {
		t.Fatalf("expected 'Auftragsposition ist bereits vollständig fakturiert', got %v", err)
	}
}

func TestSelectInvoiceQuantitiesRejectsQtyExceedingRemaining(t *testing.T) {
	items := []SalesOrderItem{{ID: "a"}}
	remaining := map[string]float64{"a": 3}

	_, err := selectInvoiceQuantities(items, remaining, []ConvertToInvoiceItemInput{{SalesOrderItemID: "a", Qty: 3.5}})
	if err == nil || err.Error() != "Teilfaktura-Menge überschreitet die offene Restmenge" {
		t.Fatalf("expected 'Teilfaktura-Menge überschreitet die offene Restmenge', got %v", err)
	}
}

func TestSelectInvoiceQuantitiesAcceptsValidPartialSelection(t *testing.T) {
	items := []SalesOrderItem{{ID: "a"}, {ID: "b"}}
	remaining := map[string]float64{"a": 5, "b": 10}

	out, err := selectInvoiceQuantities(items, remaining, []ConvertToInvoiceItemInput{{SalesOrderItemID: "a", Qty: 2}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out["a"] != 2 {
		t.Fatalf("expected only item a with qty 2, got %#v", out)
	}
}
