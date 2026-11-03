package apihttp

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Lieferpapiere an der Bestellung ueber HTTP (Backlog H.4.3b). Helfer aus
// qualification_types_integration_test.go (newCatalogTenant),
// supplier_certificates_integration_test.go (uploadMultipart) und
// time_entries_integration_test.go (wantAPIError).

type deliveryPaperResponse struct {
	ID           uuid.UUID `json:"id"`
	OrderID      string    `json:"bestellung_id"`
	ItemID       *string   `json:"position_id"`
	Art          string    `json:"art"`
	Charge       string    `json:"charge"`
	Notiz        string    `json:"notiz"`
	Filename     string    `json:"filename"`
	StornoReason string    `json:"storno_grund"`
	StorniertAt  *string   `json:"storniert_am"`
}

// uploadPaper: Multipart mit Datei im Feld "file" und Formularfeldern.
func uploadPaper(t *testing.T, handler http.Handler, token, url string, fields map[string]string, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(content)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, url, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestPurchaseOrderPapersHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	buyer := c.user("procurement")
	n := uuid.NewString()[:8]
	supplier, material, order, item, order2, item2 := "sup-pap-"+n, "mat-pap-"+n, "po-pap-"+n, "poi-pap-"+n, "po-pap2-"+n, "poi-pap2-"+n
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO contacts (id, typ, rolle, name, company_id) VALUES ($1,'org','supplier','Zulieferer',$2)`, []any{supplier, c.company}},
		{`INSERT INTO materials (id, nummer, bezeichnung, typ, einheit, company_id) VALUES ($1,$2,'Profil','profil','Stk',$3)`, []any{material, "MAT-PAP-" + n, c.company}},
		{`INSERT INTO purchase_orders (id, supplier_id, number, currency, status, company_id) VALUES ($1,$2,$3,'EUR','ordered',$4)`, []any{order, supplier, "PO-PAP-" + n, c.company}},
		{`INSERT INTO purchase_order_items (id, order_id, position, material_id, qty, uom, unit_price, currency) VALUES ($1,$2,1,$3,5,'Stk',0,'EUR')`, []any{item, order, material}},
		{`INSERT INTO purchase_orders (id, supplier_id, number, currency, status, company_id) VALUES ($1,$2,$3,'EUR','ordered',$4)`, []any{order2, supplier, "PO-PAP2-" + n, c.company}},
		{`INSERT INTO purchase_order_items (id, order_id, position, material_id, qty, uom, unit_price, currency) VALUES ($1,$2,1,$3,1,'Stk',0,'EUR')`, []any{item2, order2, material}},
	} {
		if _, err := env.PG.Exec(context.Background(), q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	do := func(token, method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, token, method, path, body)
	}
	base := "/api/v1/purchase-orders/" + order + "/papers/"
	pdf := []byte("%PDF-1.4\nLeistungserklaerung\n")

	created := decodeJSON[deliveryPaperResponse](t, uploadPaper(t, handler, buyer, base,
		map[string]string{"art": "leistungserklaerung", "position_id": item, "charge": "CH-7", "notiz": "Lieferung 1"}, "dop.pdf", pdf), http.StatusCreated)
	if created.Art != "leistungserklaerung" || created.ItemID == nil || *created.ItemID != item || created.Charge != "CH-7" ||
		created.Notiz != "Lieferung 1" || created.OrderID != order || created.Filename != "dop.pdf" {
		t.Fatalf("hochgeladen: %+v", created)
	}
	whole := decodeJSON[deliveryPaperResponse](t, uploadPaper(t, handler, buyer, base, map[string]string{"art": "zeugnis_3_1"}, "z.pdf", pdf), http.StatusCreated)
	if whole.ItemID != nil {
		t.Fatalf("ohne Position: %+v", whole)
	}

	// Eingaben
	wantAPIError(t, uploadPaper(t, handler, buyer, base, map[string]string{"art": "rechnung"}, "x.pdf", pdf), http.StatusBadRequest, "validation_error", "Ungültige Art des Lieferpapiers")
	wantAPIError(t, uploadPaper(t, handler, buyer, base, map[string]string{"art": "lieferschein", "position_id": item2}, "x.pdf", pdf), http.StatusNotFound, "not_found", "Position nicht gefunden")
	wantAPIError(t, uploadPaper(t, handler, buyer, base, map[string]string{"art": "lieferschein"}, "x.txt", []byte("Text")), http.StatusBadRequest, "validation_error", "Ungültige Datei: erlaubt sind PDF, JPEG und PNG")
	wantAPIError(t, uploadPaper(t, handler, buyer, "/api/v1/purchase-orders/keine/papers/", map[string]string{"art": "lieferschein"}, "x.pdf", pdf), http.StatusNotFound, "not_found", "Bestellung nicht gefunden")
	wantAPIError(t, do(buyer, http.MethodPost, base, `{}`), http.StatusBadRequest, "validation_error", "Ungültiges Upload-Formular oder Datei zu groß")

	list := decodeJSON[[]deliveryPaperResponse](t, do(buyer, http.MethodGet, base, ""), http.StatusOK)
	if len(list) != 2 || list[0].ID != created.ID || list[1].ID != whole.ID {
		t.Fatalf("Liste: %+v", list)
	}
	if l := decodeJSON[[]deliveryPaperResponse](t, do(buyer, http.MethodGet, "/api/v1/purchase-orders/"+order2+"/papers/", ""), http.StatusOK); len(l) != 0 {
		t.Fatalf("andere Bestellung: %+v", l)
	}
	dl := do(buyer, http.MethodGet, base+created.ID.String(), "")
	if dl.Code != http.StatusOK || !bytes.Equal(dl.Body.Bytes(), pdf) || dl.Header().Get("Content-Type") != "application/pdf" ||
		!strings.Contains(dl.Header().Get("Content-Disposition"), "dop.pdf") {
		t.Fatalf("Download: %d %q %v", dl.Code, dl.Body.String(), dl.Header())
	}
	wantAPIError(t, do(buyer, http.MethodGet, base+"kein-uuid", ""), http.StatusBadRequest, "validation_error", "Ungültige Lieferpapier-ID")
	wantAPIError(t, do(buyer, http.MethodGet, "/api/v1/purchase-orders/"+order2+"/papers/"+created.ID.String(), ""), http.StatusNotFound, "not_found", "Lieferpapier nicht gefunden")

	// Die bestehenden Bestellrouten bleiben erreichbar.
	if r := do(buyer, http.MethodGet, "/api/v1/purchase-orders/"+order, ""); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), "PO-PAP-"+n) {
		t.Fatalf("Bestellung: %d %s", r.Code, r.Body.String())
	}

	// Rechte: Buchhaltung liest, schreibt nicht; Vertrieb sieht nichts.
	finance := c.user("finance")
	if l := decodeJSON[[]deliveryPaperResponse](t, do(finance, http.MethodGet, base, ""), http.StatusOK); len(l) != 2 {
		t.Fatalf("Buchhaltung liest: %+v", l)
	}
	if r := do(finance, http.MethodGet, base+created.ID.String(), ""); r.Code != http.StatusOK {
		t.Fatalf("Buchhaltung Download: %d", r.Code)
	}
	for _, r := range []*httptest.ResponseRecorder{
		uploadPaper(t, handler, finance, base, map[string]string{"art": "lieferschein"}, "x.pdf", pdf),
		do(finance, http.MethodPost, base+created.ID.String()+"/storno", `{"reason":"x"}`),
	} {
		if r.Code != http.StatusForbidden {
			t.Fatalf("Buchhaltung schreibt: %d %s", r.Code, r.Body.String())
		}
	}
	sales := c.user("sales")
	for _, p := range []string{base, base + created.ID.String()} {
		if r := do(sales, http.MethodGet, p, ""); r.Code != http.StatusForbidden {
			t.Fatalf("Vertrieb liest %s: %d", p, r.Code)
		}
	}
	stranger := newCatalogTenant(t, env, handler).user("procurement")
	wantAPIError(t, do(stranger, http.MethodGet, base, ""), http.StatusNotFound, "not_found", "Bestellung nicht gefunden")
	wantAPIError(t, do(stranger, http.MethodGet, base+created.ID.String(), ""), http.StatusNotFound, "not_found", "Bestellung nicht gefunden")

	// Storno
	sp := base + created.ID.String() + "/storno"
	wantAPIError(t, do(buyer, http.MethodPost, sp, `{"reason":" "}`), http.StatusBadRequest, "validation_error", "Storno-Begründung erforderlich")
	wantAPIError(t, do(buyer, http.MethodPost, sp, `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, do(buyer, http.MethodPost, base+"kein-uuid/storno", `{"reason":"x"}`), http.StatusBadRequest, "validation_error", "Ungültige Lieferpapier-ID")
	st := decodeJSON[deliveryPaperResponse](t, do(buyer, http.MethodPost, sp, `{"reason":"falsche Charge"}`), http.StatusOK)
	if st.StorniertAt == nil || st.StornoReason != "falsche Charge" {
		t.Fatalf("storniert: %+v", st)
	}
	wantAPIError(t, do(buyer, http.MethodPost, sp, `{"reason":"nochmal"}`), http.StatusConflict, "conflict", "Lieferpapier ist bereits storniert")
	if l := decodeJSON[[]deliveryPaperResponse](t, do(buyer, http.MethodGet, base, ""), http.StatusOK); len(l) != 1 || l[0].ID != whole.ID {
		t.Fatalf("ohne stornierte: %+v", l)
	}
	if l := decodeJSON[[]deliveryPaperResponse](t, do(buyer, http.MethodGet, base+"?include_storniert=true", ""), http.StatusOK); len(l) != 2 {
		t.Fatalf("mit stornierten: %+v", l)
	}

	// Stornierte Bestellung: 409.
	if _, err := env.PG.Exec(context.Background(), `UPDATE purchase_orders SET status='canceled' WHERE id=$1`, order); err != nil {
		t.Fatal(err)
	}
	wantAPIError(t, uploadPaper(t, handler, buyer, base, map[string]string{"art": "lieferschein"}, "x.pdf", pdf), http.StatusConflict, "conflict", "Bestellung ist storniert")
}
