package apihttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/hr"
	"nalaerp3/internal/testutil"
)

// Anmahnen ueber HTTP (Backlog F.2.6.2): Stand eines Mitarbeiters und
// betriebsweite Liste. Helfer aus qualification_types_integration_test.go
// und qualifications_integration_test.go.
//
// Die Routen rechnen mit "heute". Damit die Tests an JEDEM Tag dasselbe
// Ergebnis liefern, sind die Nachweise so gewaehlt, dass ihr Zustand nicht
// vom Datum abhaengt (lange abgelaufen, unbefristet, fehlend) - mit einer
// Ausnahme, die relativ zu heute berechnet wird: erworben vor elf Monaten
// bei zwoelf Monaten Frist laeuft in 28 bis 33 Tagen ab, also innerhalb
// von 60 Tagen Vorwarnzeit und ausserhalb von 10.

type standingResponse struct {
	TypeID        uuid.UUID `json:"type_id"`
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	Status        string    `json:"status"`
	Erforderlich  bool      `json:"erforderlich"`
	TageBisAblauf *int      `json:"tage_bis_ablauf"`
}

type dueResponse struct {
	EmployeeID uuid.UUID `json:"employee_id"`
	LastName   string    `json:"last_name"`
	Code       string    `json:"code"`
	Status     string    `json:"status"`
}

func decodeStanding(t *testing.T, rec *httptest.ResponseRecorder) []standingResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("erwartet 200, bekommen %d mit Body %s", rec.Code, rec.Body.String())
	}
	var out []standingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

func decodeDue(t *testing.T, rec *httptest.ResponseRecorder) []dueResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("erwartet 200, bekommen %d mit Body %s", rec.Code, rec.Body.String())
	}
	var out []dueResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

func standingSummary(st []standingResponse) []string {
	out := make([]string, 0, len(st))
	for _, s := range st {
		out = append(out, s.Code+"="+s.Status)
	}
	return out
}

type standingWorld struct {
	tenant    *catalogTenant
	hrToken   string
	worker    qualPerson
	colleague uuid.UUID
	gone      uuid.UUID
}

// newStandingWorld baut einen Betrieb:
//
//   - Arbeitsschutz: Pflicht fuer alle, Frist 12 Monate
//   - Werker (verknuepft, Nachname Adler): Stapler gefordert und fehlt;
//     Ersthelfer seit 2010 abgelaufen; Jahresunterweisung vor elf Monaten
//     (laeuft bald ab); Kran unbefristet (gueltig); Arbeitsschutz fehlt
//   - Kollege Zander: nichts - Arbeitsschutz fehlt
//   - Ausgeschiedener: Arbeitsschutz fehlt, erscheint aber nicht
func newStandingWorld(t *testing.T, env *testutil.IntegrationEnv, handler http.Handler) standingWorld {
	t.Helper()
	w := standingWorld{tenant: newCatalogTenant(t, env, handler)}
	w.hrToken = w.tenant.user("hr")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, w.hrToken, method, path, body)
	}
	mkType := func(code, name, months string, pflicht bool) uuid.UUID {
		kategorie := "qualifikation"
		if months != "null" {
			kategorie = "unterweisung"
		}
		p := "false"
		if pflicht {
			p = "true"
		}
		return decodeQualificationType(t, do(http.MethodPost, qtPath,
			`{"code":"`+code+`","name":"`+name+`","kategorie":"`+kategorie+`","gueltigkeit_monate":`+months+`,"pflicht_fuer_alle":`+p+`}`), http.StatusCreated).ID
	}
	mkType("ARBEITSSCHUTZ", "Arbeitsschutz", "12", true)
	stapler := mkType("STAPLER_SCHEIN", "Stapler", "null", false)
	ersthelfer := mkType("ERSTHELFER", "Ersthelfer", "24", false)
	jahr := mkType("JAHRESUNTERWEISUNG", "Jahresunterweisung", "12", false)
	kran := mkType("KRANFUEHRER", "Kran", "null", false)

	w.worker = w.tenant.linkedUser("production", "Adler")
	w.colleague = w.tenant.employee("Zander")
	no := false
	gone, err := hr.NewEmployeeService(env.PG).Create(context.Background(), hr.EmployeeInput{FirstName: "Quali", LastName: "Gegangen", Active: &no}, w.tenant.company)
	if err != nil {
		t.Fatalf("Mitarbeiter anlegen: %v", err)
	}
	w.gone = gone.ID

	decodeRequirements(t, do(http.MethodPut, qualPath+"requirements?employee_id="+w.worker.EmployeeID.String(), `{"type_ids":["`+stapler.String()+`"]}`))
	elevenMonthsAgo := hr.WorkDateOf(time.Now()).AddDate(0, -11, 0).Format("2006-01-02")
	for _, r := range []struct {
		typ      uuid.UUID
		erworben string
	}{{ersthelfer, "2010-01-01"}, {jahr, elevenMonthsAgo}, {kran, "2015-05-05"}} {
		decodeQualRecords(t, do(http.MethodPost, qualPath, qualBody(r.typ, r.erworben, w.worker.EmployeeID)), http.StatusCreated)
	}
	return w
}

func TestQualificationStandingHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	w := newStandingWorld(t, env, handler)

	// Der Werker sieht seinen eigenen Stand - ohne employee_id.
	own := decodeStanding(t, doAuthedRequest(t, handler, w.worker.Token, http.MethodGet, qualPath+"standing", ""))
	want := []string{
		"ARBEITSSCHUTZ=fehlt", "STAPLER_SCHEIN=fehlt", // fehlt, nach Name
		"ERSTHELFER=abgelaufen",
		"JAHRESUNTERWEISUNG=laeuft_ab",
		"KRANFUEHRER=gueltig",
	}
	if got := standingSummary(own); !reflect.DeepEqual(got, want) {
		t.Fatalf("eigener Stand:\nerwartet %v\nbekommen %v", want, got)
	}
	for _, s := range own {
		switch s.Code {
		case "ARBEITSSCHUTZ", "STAPLER_SCHEIN":
			if !s.Erforderlich || s.TageBisAblauf != nil {
				t.Fatalf("%s: erwartet erforderlich ohne Tage, bekommen %+v", s.Code, s)
			}
		case "JAHRESUNTERWEISUNG":
			if s.TageBisAblauf == nil || *s.TageBisAblauf < 27 || *s.TageBisAblauf > 34 {
				t.Fatalf("Jahresunterweisung: erwartet etwa 30 Tage bis Ablauf, bekommen %v", s.TageBisAblauf)
			}
		}
		if s.Name == "" {
			t.Fatalf("Name der Art fehlt: %+v", s)
		}
	}

	// Die Personalabteilung sieht denselben Stand ueber employee_id.
	viaHR := decodeStanding(t, doAuthedRequest(t, handler, w.hrToken, http.MethodGet, qualPath+"standing?employee_id="+w.worker.EmployeeID.String(), ""))
	if !reflect.DeepEqual(standingSummary(viaHR), want) {
		t.Fatalf("Stand ueber die Personalabteilung weicht ab: %v", standingSummary(viaHR))
	}

	// Vorwarnzeit 10 Tage: die Jahresunterweisung ist dann noch gueltig.
	short := decodeStanding(t, doAuthedRequest(t, handler, w.worker.Token, http.MethodGet, qualPath+"standing?warn_tage=10", ""))
	for _, s := range short {
		if s.Code == "JAHRESUNTERWEISUNG" && s.Status != "gueltig" {
			t.Fatalf("bei 10 Tagen Vorwarnzeit erwartet gueltig, bekommen %s", s.Status)
		}
	}

	// Betriebsweite Liste: nur Offenes, nur aktive Mitarbeiter.
	due := decodeDue(t, doAuthedRequest(t, handler, w.hrToken, http.MethodGet, qualPath+"due", ""))
	gotDue := make([]string, 0, len(due))
	for _, d := range due {
		gotDue = append(gotDue, d.LastName+"/"+d.Code+"="+d.Status)
	}
	wantDue := []string{
		"Adler/ARBEITSSCHUTZ=fehlt", "Adler/STAPLER_SCHEIN=fehlt", "Zander/ARBEITSSCHUTZ=fehlt",
		"Adler/ERSTHELFER=abgelaufen",
		"Adler/JAHRESUNTERWEISUNG=laeuft_ab",
	}
	if !reflect.DeepEqual(gotDue, wantDue) {
		t.Fatalf("betriebsweite Liste:\nerwartet %v\nbekommen %v", wantDue, gotDue)
	}
	for _, d := range due {
		if d.EmployeeID == w.gone {
			t.Fatal("ausgeschiedener Mitarbeiter in der Liste")
		}
	}
	if dueShort := decodeDue(t, doAuthedRequest(t, handler, w.hrToken, http.MethodGet, qualPath+"due?warn_tage=10", "")); len(dueShort) != 4 {
		t.Fatalf("bei 10 Tagen Vorwarnzeit erwartet 4 offene Punkte, bekommen %d", len(dueShort))
	}

	// "standing" und "due" sind eigene Routen, keine Nachweis-IDs; Fehlerfaelle.
	wantAPIError(t, doAuthedRequest(t, handler, w.hrToken, http.MethodGet, qualPath+"standing?warn_tage=-1", ""), http.StatusBadRequest, "validation_error", "Ungültige Vorwarnzeit (warn_tage: ganze Zahl ab 0)")
	wantAPIError(t, doAuthedRequest(t, handler, w.hrToken, http.MethodGet, qualPath+"due?warn_tage=zwei", ""), http.StatusBadRequest, "validation_error", "Ungültige Vorwarnzeit (warn_tage: ganze Zahl ab 0)")
	wantAPIError(t, doAuthedRequest(t, handler, w.hrToken, http.MethodGet, qualPath+"standing?employee_id=xyz", ""), http.StatusBadRequest, "validation_error", "Ungültige employee_id")
	wantAPIError(t, doAuthedRequest(t, handler, w.hrToken, http.MethodGet, qualPath+"standing?employee_id="+uuid.NewString(), ""), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
}

func TestQualificationStandingHTTPPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-qual-readonly", "qualifications.read")
	seedRoleWithPermissions(t, env, "itest-qual-writeonly", "qualifications.write")
	w := newStandingWorld(t, env, handler)
	reader, writer := w.tenant.user("itest-qual-readonly"), w.tenant.user("itest-qual-writeonly")
	ofWorker := qualPath + "standing?employee_id=" + w.worker.EmployeeID.String()
	ofColleague := qualPath + "standing?employee_id=" + w.colleague.String()
	forbidden := func(t *testing.T, token, path string) {
		t.Helper()
		wantAPIError(t, doAuthedRequest(t, handler, token, http.MethodGet, path, ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	}

	t.Run("self: eigener Stand ja, fremder und Liste nein", func(t *testing.T) {
		decodeStanding(t, doAuthedRequest(t, handler, w.worker.Token, http.MethodGet, ofWorker, ""))
		forbidden(t, w.worker.Token, ofColleague)
		// Die Liste enthaelt die Namen aller - self genuegt nicht.
		forbidden(t, w.worker.Token, qualPath+"due")
	})
	t.Run("nur read: fremder Stand und Liste ja", func(t *testing.T) {
		decodeStanding(t, doAuthedRequest(t, handler, reader, http.MethodGet, ofColleague, ""))
		decodeDue(t, doAuthedRequest(t, handler, reader, http.MethodGet, qualPath+"due", ""))
	})
	t.Run("nur write: weder fremder Stand noch Liste", func(t *testing.T) {
		forbidden(t, writer, ofWorker)
		forbidden(t, writer, qualPath+"due")
	})
	t.Run("ohne Anmeldung", func(t *testing.T) {
		for _, p := range []string{qualPath + "standing", qualPath + "due"} {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("%s ohne Anmeldung: erwartet 401, bekommen %d", p, rec.Code)
			}
		}
	})
}

func TestQualificationStandingHTTPAreIsolatedBetweenCompanies(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	w := newStandingWorld(t, env, handler)
	other := newCatalogTenant(t, env, handler)
	foreign := other.user("admin")

	wantAPIError(t, doAuthedRequest(t, handler, foreign, http.MethodGet, qualPath+"standing?employee_id="+w.worker.EmployeeID.String(), ""), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	if due := decodeDue(t, doAuthedRequest(t, handler, foreign, http.MethodGet, qualPath+"due", "")); len(due) != 0 {
		t.Fatalf("Liste des anderen Mandanten muss leer sein, bekommen %+v", due)
	}
}
