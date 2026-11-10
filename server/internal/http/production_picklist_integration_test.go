package apihttp

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/production"
	"nalaerp3/internal/testutil"
)

// Kommissionierliste als PDF ueber HTTP (Backlog H.3.3b).

func TestPickListPDFData(t *testing.T) {
	termin := time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC)
	got := pickListPDFData(&production.PickList{Nummer: "FA-1", ProjectNummer: "P-1", ProjectName: "Haus", PositionNummer: "3", PositionName: "Fenster",
		VarianteName: "links", Stueckzahl: 2, Liefertermin: &termin, Status: production.StatusInArbeit, Lager: "HL – Halle",
		Lines: []production.PickLine{
			{Pos: 1, ArticleCode: "P-100", Description: "Blendrahmen", MaterialNummer: "M-1", MaterialBezeichnung: "Profil", Abmessung: "L 6000 mm", Menge: 4.92162, Einheit: "m", Plaetze: []string{"HL: 8 m"}},
			{Pos: 2, Description: "Isolierglas", Menge: 2, Einheit: "", Gesamt: true},
		}}, time.Date(2026, 11, 10, 8, 5, 0, 0, time.UTC))
	if got.Project != "P-1 Haus" || got.Position != "3 Fenster (links)" || got.Status != "in Arbeit" || got.Liefertermin != "02.12.2026" ||
		got.PrintedAt != "10.11.2026 08:05" || got.Lager != "HL – Halle" || len(got.Lines) != 2 {
		t.Fatalf("Kopf: %+v", got)
	}
	if l := got.Lines[0]; l.Material != "P-100 Blendrahmen\nLager: M-1 Profil" || l.Menge != "4,922 m" || l.Abmessung != "L 6000 mm" || len(l.Plaetze) != 1 {
		t.Fatalf("Zeile 1: %+v", l)
	}
	if l := got.Lines[1]; l.Material != "Isolierglas" || l.Menge != "2 (gesamt)" {
		t.Fatalf("Zeile 2: %+v", l)
	}
	// Ohne Termin, Variante und mit unbekanntem Status.
	got = pickListPDFData(&production.PickList{Nummer: "FA-2", PositionNummer: "4", Status: "x"}, time.Now())
	if got.Liefertermin != "" || got.Position != "4" || got.Status != "x" || len(got.Lines) != 0 {
		t.Fatalf("leer: %+v", got)
	}
	if code, _ := classifyDomainError(errors.New("Kommissionierliste nur bei freigegebenen oder begonnenen Aufträgen")); code != http.StatusConflict {
		t.Fatalf("Status-Meldung: %d", code)
	}
}

func TestProductionPickListHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	prod := c.user("production")
	p := seedProductionPosition(t, env, c.company)
	mat, wh := seedWithdrawalStock(t, env, c.company, p)
	o := decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, productionOrdersPath, p.body(2, "")), http.StatusCreated)
	path := productionOrdersPath + o.ID.String() + "/kommissionierliste.pdf"

	wantAPIError(t, doAuthedRequest(t, handler, prod, http.MethodGet, path, ""), http.StatusConflict, "conflict", "Kommissionierliste nur bei freigegebenen oder begonnenen Aufträgen")
	decodeJSON[productionOrderResponse](t, doAuthedRequest(t, handler, prod, http.MethodPost, productionOrdersPath+o.ID.String()+"/freigeben", `{"lager_id":"`+wh+`"}`), http.StatusOK)

	for name, tc := range map[string]struct {
		role string
		ok   bool
	}{
		"Produktion": {"production", true},
		"Vertrieb":   {"sales", true},
		"Personal":   {"hr", false},
		"Lager":      {"inventory", false},
	} {
		t.Run(name, func(t *testing.T) {
			rec := doAuthedRequest(t, handler, c.user(tc.role), http.MethodGet, path, "")
			if !tc.ok {
				wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				return
			}
			body := rec.Body.Bytes()
			if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/pdf" ||
				!strings.Contains(rec.Header().Get("Content-Disposition"), "Kommissionierliste_"+o.Nummer+".pdf") ||
				!bytes.HasPrefix(body, []byte("%PDF")) || !bytes.Contains(body, []byte(o.Nummer)) || !bytes.Contains(body, []byte(mat)) ||
				!bytes.Contains(body, []byte(wh+": 10 m")) {
				t.Fatalf("PDF: %d %s %s", rec.Code, rec.Header().Get("Content-Type"), rec.Header().Get("Content-Disposition"))
			}
		})
	}

	wantAPIError(t, doAuthedRequest(t, handler, prod, http.MethodGet, productionOrdersPath+"kaputt/kommissionierliste.pdf", ""), http.StatusBadRequest, "validation_error", "Ungültige Auftrags-ID")
	wantAPIError(t, doAuthedRequest(t, handler, prod, http.MethodGet, productionOrdersPath+uuid.NewString()+"/kommissionierliste.pdf", ""), http.StatusNotFound, "not_found", "Fertigungsauftrag nicht gefunden")
	two := newCatalogTenant(t, env, handler).user("admin")
	wantAPIError(t, doAuthedRequest(t, handler, two, http.MethodGet, path, ""), http.StatusNotFound, "not_found", "Fertigungsauftrag nicht gefunden")
}
