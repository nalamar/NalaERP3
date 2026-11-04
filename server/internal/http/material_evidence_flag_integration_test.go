package apihttp

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Kennzeichen "Nachweis EN 1090" am Material ueber HTTP (ADR 0035 §2,
// Backlog H.4.4a): setzen mit materials.write, lesen mit materials.read.
func TestMaterialEvidenceFlagHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	lager := c.user("inventory")
	type material struct {
		ID       string `json:"id"`
		Nachweis *bool  `json:"nachweis_en1090"`
	}
	n := uuid.NewString()[:8]
	flagged := decodeJSON[material](t, doAuthedRequest(t, handler, lager, http.MethodPost, "/api/v1/materials/",
		`{"nummer":"MAT-EN-`+n+`","bezeichnung":"Träger","typ":"profil","einheit":"Stk","nachweis_en1090":true}`), http.StatusCreated)
	if flagged.Nachweis == nil || !*flagged.Nachweis {
		t.Fatalf("angelegt mit Kennzeichen: %+v", flagged)
	}
	plain := decodeJSON[material](t, doAuthedRequest(t, handler, lager, http.MethodPost, "/api/v1/materials/",
		`{"nummer":"MAT-OHNE-`+n+`","bezeichnung":"Schraube","typ":"zubehoer","einheit":"Stk"}`), http.StatusCreated)
	if plain.Nachweis == nil || *plain.Nachweis {
		t.Fatalf("Vorgabe ohne Kennzeichen: %+v", plain)
	}

	// Einkauf liest (materials.read), auch in der Liste.
	buyer := c.user("procurement")
	path := "/api/v1/materials/" + flagged.ID
	if g := decodeJSON[material](t, doAuthedRequest(t, handler, buyer, http.MethodGet, path, ""), http.StatusOK); g.Nachweis == nil || !*g.Nachweis {
		t.Fatalf("Einzelabruf: %+v", g)
	}
	list := decodeJSON[[]material](t, doAuthedRequest(t, handler, buyer, http.MethodGet, "/api/v1/materials/?q="+n, ""), http.StatusOK)
	got := map[string]bool{}
	for _, m := range list {
		if m.Nachweis == nil {
			t.Fatalf("Liste ohne Feld: %+v", m)
		}
		got[m.ID] = *m.Nachweis
	}
	if len(got) != 2 || !got[flagged.ID] || got[plain.ID] {
		t.Fatalf("Liste: %v", got)
	}
	// Einkauf darf das Kennzeichen nicht setzen (kein materials.write).
	if r := doAuthedRequest(t, handler, buyer, http.MethodPatch, path, `{"nachweis_en1090":false}`); r.Code != http.StatusForbidden {
		t.Fatalf("Einkauf setzt: %d", r.Code)
	}

	// Ohne Angabe bleibt das Kennzeichen; ausdruecklich false nimmt es weg.
	if u := decodeJSON[material](t, doAuthedRequest(t, handler, lager, http.MethodPatch, path, `{"bezeichnung":"Träger HEA"}`), http.StatusOK); u.Nachweis == nil || !*u.Nachweis {
		t.Fatalf("ohne Angabe: %+v", u)
	}
	if u := decodeJSON[material](t, doAuthedRequest(t, handler, lager, http.MethodPatch, path, `{"nachweis_en1090":false}`), http.StatusOK); u.Nachweis == nil || *u.Nachweis {
		t.Fatalf("entfernt: %+v", u)
	}
	if u := decodeJSON[material](t, doAuthedRequest(t, handler, lager, http.MethodPatch, "/api/v1/materials/"+plain.ID, `{"nachweis_en1090":true}`), http.StatusOK); u.Nachweis == nil || !*u.Nachweis {
		t.Fatalf("gesetzt: %+v", u)
	}
}
