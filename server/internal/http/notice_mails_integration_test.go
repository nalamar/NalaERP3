package apihttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Hinweis-Mails ueber HTTP (Backlog F.14.4c). Helfer aus
// qualification_types_integration_test.go (newCatalogTenant),
// leave_requests_integration_test.go (seedRoleWithPermissions) und
// asset_inspections_integration_test.go (decodeJSON).

const noticeMailsPath = "/api/v1/notice-mails/"

type noticeMailLogResponse struct {
	VersandTag  string  `json:"versand_tag"`
	Benutzer    string  `json:"benutzer"`
	Ergebnis    string  `json:"ergebnis"`
	FehlerStufe string  `json:"fehler_stufe"`
	CreatedAt   string  `json:"created_at"`
	FinishedAt  *string `json:"finished_at"`
}

// seedTenantUser legt einen Benutzer im Mandanten an und liefert ID und
// Token.
func seedTenantUser(t *testing.T, c *catalogTenant, role string) (string, string) {
	t.Helper()
	email := c.company + "-nm-" + uuid.NewString()[:6] + "@example.com"
	id := testutil.SeedAuthUser(t, c.env, email, "Secret123!", role)
	if _, err := c.env.PG.Exec(context.Background(), `UPDATE users SET company_id=$2, display_name=$3 WHERE id=$1`, id, c.company, "Anzeige "+role); err != nil {
		t.Fatal(err)
	}
	return id, loginIntegrationUser(t, c.handler, email, "Secret123!")
}

func TestNoticeMailOptOutHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-nm-none")
	c := newCatalogTenant(t, env, handler)
	// Auch ohne jedes Recht: die eigene Abbestellung.
	id, token := seedTenantUser(t, c, "itest-nm-none")
	otherID, otherToken := seedTenantUser(t, c, "itest-nm-none")
	do := func(tok, method, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, tok, method, noticeMailsPath+"me", body)
	}
	optedOut := func(user string) bool {
		var n int
		if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM notice_mail_optouts WHERE user_id=$1`, user).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 1
	}

	if got := decodeJSON[noticeMailPref](t, do(token, http.MethodGet, ""), http.StatusOK); got.Abbestellt {
		t.Fatalf("Start: %+v", got)
	}
	if got := decodeJSON[noticeMailPref](t, do(token, http.MethodPut, `{"abbestellt":true}`), http.StatusOK); !got.Abbestellt || !optedOut(id) || optedOut(otherID) {
		t.Fatalf("abbestellt: %+v", got)
	}
	decodeJSON[noticeMailPref](t, do(token, http.MethodPut, `{"abbestellt":true}`), http.StatusOK) // wiederholt
	if got := decodeJSON[noticeMailPref](t, do(token, http.MethodGet, ""), http.StatusOK); !got.Abbestellt {
		t.Fatalf("gelesen: %+v", got)
	}
	if got := decodeJSON[noticeMailPref](t, do(otherToken, http.MethodGet, ""), http.StatusOK); got.Abbestellt {
		t.Fatalf("anderer Benutzer: %+v", got)
	}
	if got := decodeJSON[noticeMailPref](t, do(token, http.MethodPut, `{"abbestellt":false}`), http.StatusOK); got.Abbestellt || optedOut(id) {
		t.Fatalf("wieder angemeldet: %+v", got)
	}
	for _, body := range []string{`{}`, `{"abbestellt":"ja"}`, `{`} {
		wantAPIError(t, do(token, http.MethodPut, body), http.StatusBadRequest, "validation_error", "abbestellt (true/false) erforderlich")
	}
	if optedOut(id) {
		t.Fatal("ungueltige Eingabe hat gewirkt")
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, noticeMailsPath+"me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: %d", rec.Code)
	}
}

func TestNoticeMailLogHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	one, two := newCatalogTenant(t, env, handler), newCatalogTenant(t, env, handler)
	admin := one.user("admin")
	userA, _ := seedTenantUser(t, one, "production")
	userB, _ := seedTenantUser(t, one, "hr")
	userTwo, _ := seedTenantUser(t, two, "hr")
	ctx := context.Background()
	ins := func(company, user, tag, ergebnis, stufe string, finished bool) {
		t.Helper()
		fin := "NULL"
		if finished {
			fin = "now()"
		}
		if _, err := env.PG.Exec(ctx, `INSERT INTO notice_mail_log (id, company_id, user_id, versand_tag, ergebnis, fehler_stufe, created_at, finished_at)
			VALUES ($1,$2,$3,$4,$5,$6, $4::date + interval '7 hours', `+fin+`)`, uuid.New(), company, user, tag, ergebnis, stufe); err != nil {
			t.Fatal(err)
		}
	}
	ins(one.company, userA, "2026-10-28", "gesendet", "", true)
	ins(one.company, userB, "2026-10-29", "fehler", "anmeldung", true)
	ins(one.company, userA, "2026-10-30", "versand", "", false)
	ins(two.company, userTwo, "2026-10-30", "gesendet", "", true)
	gone, _ := seedTenantUser(t, one, "hr")
	ins(one.company, gone, "2026-10-27", "gesendet", "", true)
	if _, err := env.PG.Exec(ctx, `DELETE FROM users WHERE id=$1`, gone); err != nil {
		t.Fatal(err)
	}

	list := decodeJSON[[]noticeMailLogResponse](t, doAuthedRequest(t, handler, admin, http.MethodGet, noticeMailsPath+"log", ""), http.StatusOK)
	if len(list) != 4 {
		t.Fatalf("Protokoll: %+v", list)
	}
	want := []struct{ tag, benutzer, ergebnis, stufe string }{
		{"2026-10-30", "Anzeige production", "versand", ""},
		{"2026-10-29", "Anzeige hr", "fehler", "anmeldung"},
		{"2026-10-28", "Anzeige production", "gesendet", ""},
		{"2026-10-27", "", "gesendet", ""},
	}
	for i, w := range want {
		g := list[i]
		if g.VersandTag[:10] != w.tag || g.Benutzer != w.benutzer || g.Ergebnis != w.ergebnis || g.FehlerStufe != w.stufe || g.CreatedAt == "" {
			t.Fatalf("Eintrag %d: %+v", i, g)
		}
	}
	if list[0].FinishedAt != nil || list[1].FinishedAt == nil {
		t.Fatalf("Abschluss: %+v", list)
	}
	if l := decodeJSON[[]noticeMailLogResponse](t, doAuthedRequest(t, handler, admin, http.MethodGet, noticeMailsPath+"log?limit=2", ""), http.StatusOK); len(l) != 2 || l[0].VersandTag[:10] != "2026-10-30" {
		t.Fatalf("limit: %+v", l)
	}
	for _, bad := range []string{"0", "-1", "x"} {
		wantAPIError(t, doAuthedRequest(t, handler, admin, http.MethodGet, noticeMailsPath+"log?limit="+bad, ""), http.StatusBadRequest, "validation_error", "limit muss eine positive Zahl sein")
	}
	// Ohne settings.manage kein Protokoll; der andere Mandant sieht nur seins.
	wantAPIError(t, doAuthedRequest(t, handler, one.user("production"), http.MethodGet, noticeMailsPath+"log", ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	if l := decodeJSON[[]noticeMailLogResponse](t, doAuthedRequest(t, handler, two.user("admin"), http.MethodGet, noticeMailsPath+"log", ""), http.StatusOK); len(l) != 1 {
		t.Fatalf("anderer Mandant: %+v", l)
	}
}

// Begrenzung des Protokolls: Standard und Hoechstwert.
func TestNoticeMailLogLimits(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	c := newCatalogTenant(t, env, handler)
	admin := c.user("admin")
	user, _ := seedTenantUser(t, c, "hr")
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO notice_mail_log (id, company_id, user_id, versand_tag, ergebnis)
		SELECT gen_random_uuid(), $1, $2, date '2020-01-01' + g, 'gesendet' FROM generate_series(1, 510) g`, c.company, user); err != nil {
		t.Fatal(err)
	}
	for query, want := range map[string]int{"": 100, "?limit=600": 500, "?limit=" + strconv.Itoa(250): 250} {
		if l := decodeJSON[[]noticeMailLogResponse](t, doAuthedRequest(t, handler, admin, http.MethodGet, noticeMailsPath+"log"+query, ""), http.StatusOK); len(l) != want {
			t.Errorf("%q: %d", query, len(l))
		}
	}
}
