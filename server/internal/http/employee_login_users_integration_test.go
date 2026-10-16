package apihttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Auswahlliste der Logins fuer die Personalabteilung (Backlog F.7.2.1).
// Helfer (newCatalogTenant, employee, doAuthedRequest, wantAPIError,
// seedRoleWithPermissions) aus den Testdateien der Qualifikationen,
// Zeiterfassung und Urlaubsantraege. Jeder Test in frischen Mandanten.

type loginUserResponse struct {
	UserID     string     `json:"user_id"`
	Email      string     `json:"email"`
	Name       string     `json:"name"`
	EmployeeID *uuid.UUID `json:"employee_id"`
}

const loginUsersPath = "/api/v1/employees/login-users"

// login legt einen Login im Mandanten an und liefert seine ID.
func (c *catalogTenant) login(prefix, role string) (string, string) {
	c.t.Helper()
	email := prefix + "-" + uuid.NewString()[:6] + "@" + c.company + ".example.com"
	id := testutil.SeedAuthUser(c.t, c.env, email, "Secret123!", role)
	if _, err := c.env.PG.Exec(context.Background(), `UPDATE users SET company_id=$2 WHERE id=$1`, id, c.company); err != nil {
		c.t.Fatalf("Login dem Mandanten zuordnen: %v", err)
	}
	return id, email
}

func TestEmployeeLoginUsersHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	other := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")

	frei, freiEmail := tenant.login("b-frei", "production")
	if _, err := env.PG.Exec(ctx, `UPDATE users SET first_name='Frieda', last_name='Frei', display_name='' WHERE id=$1`, frei); err != nil {
		t.Fatal(err)
	}
	verknuepft, verknuepftEmail := tenant.login("a-verknuepft", "production")
	if _, err := env.PG.Exec(ctx, `UPDATE users SET display_name='Vera V.' WHERE id=$1`, verknuepft); err != nil {
		t.Fatal(err)
	}
	gesperrt, gesperrtEmail := tenant.login("c-gesperrt", "production")
	if _, err := env.PG.Exec(ctx, `UPDATE users SET is_locked=true WHERE id=$1`, gesperrt); err != nil {
		t.Fatal(err)
	}
	deaktiviert, _ := tenant.login("d-deaktiviert", "production")
	if _, err := env.PG.Exec(ctx, `UPDATE users SET is_active=false WHERE id=$1`, deaktiviert); err != nil {
		t.Fatal(err)
	}
	fremd, _ := other.login("fremd", "production")

	emp := tenant.employee("Verknuepft")
	decodeEmployee(t, doAuthedRequest(t, handler, hrToken, http.MethodPut, "/api/v1/employees/"+emp.String()+"/user", `{"user_id":"`+verknuepft+`"}`), http.StatusOK)

	rec := doAuthedRequest(t, handler, hrToken, http.MethodGet, loginUsersPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("erwartet 200, bekommen %d (%s)", rec.Code, rec.Body.String())
	}
	// Nur die schmalen Felder - keine Rollen, keine Rechte, kein Status.
	var raw []map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, r := range raw {
		keys := make([]string, 0, len(r))
		for k := range r {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if j := strings.Join(keys, ","); j != "email,name,user_id" && j != "email,employee_id,name,user_id" {
			t.Fatalf("unerwartete Felder: %s", j)
		}
	}
	var list []loginUserResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	byID := map[string]loginUserResponse{}
	emails := make([]string, 0, len(list))
	for _, u := range list {
		byID[u.UserID] = u
		emails = append(emails, u.Email)
	}
	if !sort.SliceIsSorted(emails, func(i, j int) bool { return strings.ToLower(emails[i]) < strings.ToLower(emails[j]) }) {
		t.Fatalf("nicht nach E-Mail sortiert: %v", emails)
	}
	if _, ok := byID[deaktiviert]; ok {
		t.Fatal("deaktivierter Login darf nicht erscheinen")
	}
	if _, ok := byID[fremd]; ok {
		t.Fatal("Login eines anderen Mandanten darf nicht erscheinen")
	}
	if u, ok := byID[gesperrt]; !ok || u.Email != gesperrtEmail || u.EmployeeID != nil {
		t.Fatalf("gesperrter Login muss frei erscheinen: %+v", u)
	}
	if u := byID[frei]; u.Email != freiEmail || u.Name != "Frieda Frei" || u.EmployeeID != nil {
		t.Fatalf("freier Login: %+v", u)
	}
	if u := byID[verknuepft]; u.Email != verknuepftEmail || u.Name != "Vera V." || u.EmployeeID == nil || *u.EmployeeID != emp {
		t.Fatalf("verknuepfter Login: %+v", u)
	}
	// Der Login der Personalabteilung selbst erscheint auch (er ist ein
	// aktiver Login des Mandanten): 4 Eintraege.
	if len(list) != 4 {
		t.Fatalf("erwartet 4 Logins (hr, frei, verknuepft, gesperrt), bekommen %d: %+v", len(list), list)
	}

	// Verknuepfung loesen: der Login ist wieder frei.
	decodeEmployee(t, doAuthedRequest(t, handler, hrToken, http.MethodPut, "/api/v1/employees/"+emp.String()+"/user", `{"user_id":null}`), http.StatusOK)
	var after []loginUserResponse
	if err := json.Unmarshal(doAuthedRequest(t, handler, hrToken, http.MethodGet, loginUsersPath, "").Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	for _, u := range after {
		if u.UserID == verknuepft && u.EmployeeID != nil {
			t.Fatalf("nach dem Loesen muss der Login frei sein: %+v", u)
		}
	}

	// Ein Mitarbeiter eines anderen Mandanten, der (fehlerhaft) mit einem
	// Login dieses Mandanten verknuepft waere, zaehlt hier nicht.
	otherEmp := other.employee("Fremd")
	if _, err := env.PG.Exec(ctx, `UPDATE hr_employees SET user_id=$1 WHERE id=$2`, frei, otherEmp); err != nil {
		t.Fatal(err)
	}
	var cross []loginUserResponse
	if err := json.Unmarshal(doAuthedRequest(t, handler, hrToken, http.MethodGet, loginUsersPath, "").Body.Bytes(), &cross); err != nil {
		t.Fatal(err)
	}
	for _, u := range cross {
		if u.UserID == frei && u.EmployeeID != nil {
			t.Fatalf("Mitarbeiter eines anderen Mandanten darf nicht als Verknuepfung erscheinen: %+v", u)
		}
	}
}

// Nur mit employees.write - wie das Verknuepfen selbst. employees.read
// allein genuegt nicht.
func TestEmployeeLoginUsersHTTPPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-emp-readonly", "employees.read")
	seedRoleWithPermissions(t, env, "itest-emp-writeonly", "employees.write")
	tenant := newCatalogTenant(t, env, handler)

	for _, tc := range []struct {
		role string
		ok   bool
	}{
		{"hr", true},
		{"admin", true},
		{"itest-emp-writeonly", true},
		{"itest-emp-readonly", false},
		{"production", false},
		{"inventory", false},
	} {
		t.Run(tc.role, func(t *testing.T) {
			rec := doAuthedRequest(t, handler, tenant.user(tc.role), http.MethodGet, loginUsersPath, "")
			if tc.ok {
				if rec.Code != http.StatusOK {
					t.Fatalf("erwartet 200, bekommen %d (%s)", rec.Code, rec.Body.String())
				}
				return
			}
			wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
		})
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, loginUsersPath, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: erwartet 401, bekommen %d", rec.Code)
	}
}
