package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/assets"
	"nalaerp3/internal/testutil"
)

// HTTP-Anbindung der Ausgaben (Backlog F.3.6.2). Helfer (newCatalogTenant,
// linkedUser, employee, doAuthedRequest, wantAPIError,
// seedRoleWithPermissions) aus den Testdateien der Qualifikationen,
// Zeiterfassung und Urlaubsantraege. Alle Ausgabedaten liegen fest in der
// Vergangenheit.

type assignmentResponse struct {
	ID                      uuid.UUID `json:"id"`
	AssetID                 uuid.UUID `json:"asset_id"`
	Inventarnummer          string    `json:"inventarnummer"`
	EmployeeID              uuid.UUID `json:"employee_id"`
	MitarbeiterName         string    `json:"mitarbeiter_name"`
	AusgegebenAm            string    `json:"ausgegeben_am"`
	ZurueckgegebenAm        *string   `json:"zurueckgegeben_am"`
	RueckgabeZustand        string    `json:"rueckgabe_zustand"`
	Notiz                   string    `json:"notiz"`
	Status                  string    `json:"status"`
	StornoReason            string    `json:"storno_reason"`
	StorniertByUserID       *string   `json:"storniert_by_user_id"`
	AusgegebenByUserID      *string   `json:"ausgegeben_by_user_id"`
	ZurueckgenommenByUserID *string   `json:"zurueckgenommen_by_user_id"`
	Hinweise                []string  `json:"hinweise"`
}

func decodeAssignment(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) assignmentResponse {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("erwartet Status %d, bekommen %d mit Body %s", wantStatus, rec.Code, rec.Body.String())
	}
	var out assignmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

func decodeAssignmentList(t *testing.T, rec *httptest.ResponseRecorder) []assignmentResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("erwartet 200, bekommen %d mit Body %s", rec.Code, rec.Body.String())
	}
	var out []assignmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Liste nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

func assignmentNumbers(list []assignmentResponse) string {
	out := make([]string, 0, len(list))
	for _, x := range list {
		out = append(out, x.Inventarnummer)
	}
	return strings.Join(out, ",")
}

const aaPath = "/api/v1/asset-assignments/"

func (c *catalogTenant) asset(token, nr, extra string) assetResponse {
	c.t.Helper()
	return decodeAsset(c.t, doAuthedRequest(c.t, c.handler, token, http.MethodPost, assetPath, `{"inventarnummer":"`+nr+`","bezeichnung":"Stueck `+nr+`","kategorie":"leiter"`+extra+`}`), http.StatusCreated)
}

func issueBody(asset, employee uuid.UUID, date string) string {
	return `{"asset_id":"` + asset.String() + `","employee_id":"` + employee.String() + `","ausgegeben_am":"` + date + `"}`
}

func TestAssetAssignmentsHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, hrToken, method, path, body)
	}
	anna := tenant.employee("Anna")
	bert := tenant.employee("Bert")
	leiter := tenant.asset(hrToken, "L-1", "")

	issued := decodeAssignment(t, do(http.MethodPost, aaPath, `{"asset_id":"`+leiter.ID.String()+`","employee_id":"`+anna.String()+`","ausgegeben_am":"2026-01-05","notiz":"mit Haken"}`), http.StatusCreated)
	if issued.AssetID != leiter.ID || issued.EmployeeID != anna || !strings.HasPrefix(issued.AusgegebenAm, "2026-01-05") || issued.Notiz != "mit Haken" ||
		issued.Inventarnummer != "L-1" || issued.MitarbeiterName != "Quali Anna" || issued.AusgegebenByUserID == nil || issued.Status != "active" {
		t.Fatalf("Ausgabe nicht wie erwartet: %+v", issued)
	}
	// Ohne Pruefintervall und ohne Pruefung: keine Hinweise, Feld fehlt.
	if len(issued.Hinweise) != 0 {
		t.Fatalf("keine Hinweise erwartet: %q", issued.Hinweise)
	}
	got := decodeAssignment(t, do(http.MethodGet, aaPath+issued.ID.String(), ""), http.StatusOK)
	if got.ID != issued.ID || got.EmployeeID != anna {
		t.Fatalf("Einzelabruf: %+v", got)
	}

	wantAPIError(t, do(http.MethodPost, aaPath, issueBody(leiter.ID, bert, "2026-01-06")), http.StatusConflict, "conflict", "Asset ist bereits ausgegeben")
	wantAPIError(t, do(http.MethodPost, aaPath+issued.ID.String()+"/return", `{"datum":"2026-01-04"}`), http.StatusBadRequest, "validation_error", "Rückgabedatum darf nicht vor dem Ausgabedatum liegen")
	wantAPIError(t, do(http.MethodPost, aaPath+issued.ID.String()+"/return", `{}`), http.StatusBadRequest, "validation_error", "Rückgabedatum erforderlich")
	wantAPIError(t, do(http.MethodPost, aaPath+issued.ID.String()+"/return", `{"datum":"05.01.2026"}`), http.StatusBadRequest, "validation_error", "Ungültiges Datum in datum (erwartet JJJJ-MM-TT)")
	returned := decodeAssignment(t, do(http.MethodPost, aaPath+issued.ID.String()+"/return", `{"datum":"2026-01-10","zustand":"Holm verbogen"}`), http.StatusOK)
	if returned.ZurueckgegebenAm == nil || !strings.HasPrefix(*returned.ZurueckgegebenAm, "2026-01-10") || returned.RueckgabeZustand != "Holm verbogen" || returned.ZurueckgenommenByUserID == nil {
		t.Fatalf("Rueckgabe: %+v", returned)
	}
	wantAPIError(t, do(http.MethodPost, aaPath+issued.ID.String()+"/return", `{"datum":"2026-01-11"}`), http.StatusConflict, "conflict", "Asset wurde bereits zurückgegeben")
	wantAPIError(t, do(http.MethodPost, aaPath, issueBody(leiter.ID, bert, "2026-01-08")), http.StatusConflict, "conflict", "Ausgabe überschneidet sich mit einer früheren Ausgabe dieses Assets")

	second := decodeAssignment(t, do(http.MethodPost, aaPath, issueBody(leiter.ID, bert, "2026-01-10")), http.StatusCreated)
	wantAPIError(t, do(http.MethodPost, aaPath+second.ID.String()+"/storno", `{"reason":" "}`), http.StatusBadRequest, "validation_error", "Storno-Begründung erforderlich")
	st := decodeAssignment(t, do(http.MethodPost, aaPath+second.ID.String()+"/storno", `{"reason":"falsche Person"}`), http.StatusOK)
	if st.Status != "storniert" || st.StornoReason != "falsche Person" || st.StorniertByUserID == nil {
		t.Fatalf("Storno: %+v", st)
	}
	wantAPIError(t, do(http.MethodPost, aaPath+second.ID.String()+"/storno", `{"reason":"nochmal"}`), http.StatusConflict, "conflict", "Ausgabe ist bereits storniert")
	wantAPIError(t, do(http.MethodPost, aaPath+second.ID.String()+"/return", `{"datum":"2026-01-12"}`), http.StatusConflict, "conflict", "Ausgabe ist storniert")

	// Liste mit Filtern.
	other := tenant.asset(hrToken, "L-2", "")
	decodeAssignment(t, do(http.MethodPost, aaPath, issueBody(other.ID, bert, "2026-01-03")), http.StatusCreated)
	if nums := assignmentNumbers(decodeAssignmentList(t, do(http.MethodGet, aaPath, ""))); nums != "L-1,L-2" {
		t.Fatalf("Liste: %s", nums)
	}
	if nums := assignmentNumbers(decodeAssignmentList(t, do(http.MethodGet, aaPath+"?include_storniert=true", ""))); nums != "L-1,L-1,L-2" {
		t.Fatalf("mit Stornierten: %s", nums)
	}
	if nums := assignmentNumbers(decodeAssignmentList(t, do(http.MethodGet, aaPath+"?nur_offen=true", ""))); nums != "L-2" {
		t.Fatalf("nur offene: %s", nums)
	}
	if nums := assignmentNumbers(decodeAssignmentList(t, do(http.MethodGet, aaPath+"?asset_id="+other.ID.String(), ""))); nums != "L-2" {
		t.Fatalf("je Asset: %s", nums)
	}
	if nums := assignmentNumbers(decodeAssignmentList(t, do(http.MethodGet, aaPath+"?employee_id="+anna.String(), ""))); nums != "L-1" {
		t.Fatalf("je Mitarbeiter: %s", nums)
	}

	// Pruefstand: gesperrt abgelehnt, Hinweise in der Antwort.
	svc := assets.NewService(env.PG)
	gurt := tenant.asset(hrToken, "PSA-1", `,"pruefintervall_monate":12`)
	if _, err := svc.RecordInspection(context.Background(), assets.InspectionInput{AssetID: gurt.ID, GeprueftAm: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC), Ergebnis: "nicht_bestanden", PrueferName: "Meier"}, "", tenant.company); err != nil {
		t.Fatal(err)
	}
	wantAPIError(t, do(http.MethodPost, aaPath, issueBody(gurt.ID, anna, "2026-01-05")), http.StatusConflict, "conflict", "Asset ist gesperrt: letzte Prüfung nicht bestanden")
	nie := tenant.asset(hrToken, "PSA-2", `,"pruefintervall_monate":12`)
	withHint := decodeAssignment(t, do(http.MethodPost, aaPath, issueBody(nie.ID, anna, "2026-01-05")), http.StatusCreated)
	if strings.Join(withHint.Hinweise, "|") != "Asset wurde noch nie geprüft" {
		t.Fatalf("Hinweise: %q", withHint.Hinweise)
	}

	// Ausgemustert, inaktiver Mitarbeiter, Fehlerfaelle.
	alt := tenant.asset(hrToken, "ALT-1", "")
	decodeAsset(t, do(http.MethodPost, assetPath+alt.ID.String()+"/retire", `{"datum":"2026-01-02","grund":"kaputt"}`), http.StatusOK)
	wantAPIError(t, do(http.MethodPost, aaPath, issueBody(alt.ID, anna, "2026-01-05")), http.StatusConflict, "conflict", "Asset ist ausgemustert")
	weg := tenant.employee("Weg")
	if _, err := env.PG.Exec(context.Background(), `UPDATE hr_employees SET active=false WHERE id=$1`, weg); err != nil {
		t.Fatal(err)
	}
	frei := tenant.asset(hrToken, "FREI-1", "")
	wantAPIError(t, do(http.MethodPost, aaPath, issueBody(frei.ID, weg, "2026-01-05")), http.StatusConflict, "conflict", "Mitarbeiter ist nicht aktiv")
	wantAPIError(t, do(http.MethodPost, aaPath, issueBody(frei.ID, uuid.New(), "2026-01-05")), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	wantAPIError(t, do(http.MethodPost, aaPath, issueBody(uuid.New(), anna, "2026-01-05")), http.StatusNotFound, "not_found", "Asset nicht gefunden")
	wantAPIError(t, do(http.MethodPost, aaPath, `{"asset_id":"`+frei.ID.String()+`","employee_id":"`+anna.String()+`"}`), http.StatusBadRequest, "validation_error", "Ausgabedatum erforderlich")
	wantAPIError(t, do(http.MethodPost, aaPath, issueBody(frei.ID, anna, "5.1.2026")), http.StatusBadRequest, "validation_error", "Ungültiges Datum in ausgegeben_am (erwartet JJJJ-MM-TT)")
	wantAPIError(t, do(http.MethodPost, aaPath, issueBody(frei.ID, anna, time.Now().AddDate(0, 0, 2).Format("2006-01-02"))), http.StatusBadRequest, "validation_error", "Ausgabedatum darf nicht in der Zukunft liegen")
	wantAPIError(t, do(http.MethodPost, aaPath, `{"employee_id":"`+anna.String()+`","ausgegeben_am":"2026-01-05"}`), http.StatusBadRequest, "validation_error", "asset_id erforderlich")
	wantAPIError(t, do(http.MethodPost, aaPath, `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	wantAPIError(t, do(http.MethodGet, aaPath+uuid.NewString(), ""), http.StatusNotFound, "not_found", "Ausgabe nicht gefunden")
	wantAPIError(t, do(http.MethodGet, aaPath+"kein-uuid", ""), http.StatusBadRequest, "validation_error", "Ungültige Ausgabe-ID")
	wantAPIError(t, do(http.MethodPost, aaPath+"kein-uuid/return", `{}`), http.StatusBadRequest, "validation_error", "Ungültige Ausgabe-ID")
	wantAPIError(t, do(http.MethodPost, aaPath+"kein-uuid/storno", `{}`), http.StatusBadRequest, "validation_error", "Ungültige Ausgabe-ID")
	wantAPIError(t, do(http.MethodPost, aaPath+uuid.NewString()+"/return", `{"datum":"2026-01-05"}`), http.StatusNotFound, "not_found", "Ausgabe nicht gefunden")
	wantAPIError(t, do(http.MethodGet, aaPath+"?asset_id=kaputt", ""), http.StatusBadRequest, "validation_error", "Ungültige asset_id")
	wantAPIError(t, do(http.MethodGet, aaPath+"?employee_id=kaputt", ""), http.StatusBadRequest, "validation_error", "Ungültige employee_id")
}

// Schreiben nur mit assets.write. Lesen: mit assets.read alles, mit
// assets.self nur die eigenen Ausgaben - fremde weder in der Liste noch
// einzeln.
func TestAssetAssignmentsHTTPPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-aa-readonly", "assets.read")
	seedRoleWithPermissions(t, env, "itest-aa-writeonly", "assets.write")
	seedRoleWithPermissions(t, env, "itest-aa-none")
	tenant := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")

	monteur := tenant.linkedUser("production", "Monteur")
	lager := tenant.linkedUser("inventory", "Lager")
	fremd := tenant.employee("Fremd")
	own := decodeAssignment(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, aaPath, issueBody(tenant.asset(hrToken, "M-1", "").ID, monteur.EmployeeID, "2026-01-05")), http.StatusCreated)
	foreign := decodeAssignment(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, aaPath, issueBody(tenant.asset(hrToken, "F-1", "").ID, fremd, "2026-01-05")), http.StatusCreated)

	// Selbstauskunft: nur das eigene, auch ohne employee_id.
	for _, p := range []qualPerson{monteur} {
		if nums := assignmentNumbers(decodeAssignmentList(t, doAuthedRequest(t, handler, p.Token, http.MethodGet, aaPath, ""))); nums != "M-1" {
			t.Fatalf("eigene Liste: %s", nums)
		}
		if nums := assignmentNumbers(decodeAssignmentList(t, doAuthedRequest(t, handler, p.Token, http.MethodGet, aaPath+"?employee_id="+p.EmployeeID.String(), ""))); nums != "M-1" {
			t.Fatalf("eigene Liste mit eigener employee_id: %s", nums)
		}
		// Ein Filter auf ein fremdes Asset liefert nichts Fremdes.
		if list := decodeAssignmentList(t, doAuthedRequest(t, handler, p.Token, http.MethodGet, aaPath+"?asset_id="+foreign.AssetID.String(), "")); len(list) != 0 {
			t.Fatalf("Filter auf fremdes Asset darf nichts liefern: %+v", list)
		}
		wantAPIError(t, doAuthedRequest(t, handler, p.Token, http.MethodGet, aaPath+"?employee_id="+fremd.String(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
		decodeAssignment(t, doAuthedRequest(t, handler, p.Token, http.MethodGet, aaPath+own.ID.String(), ""), http.StatusOK)
		wantAPIError(t, doAuthedRequest(t, handler, p.Token, http.MethodGet, aaPath+foreign.ID.String(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	}
	// Das Lager hat nur assets.self: eigene Liste (leer), nichts Fremdes.
	if list := decodeAssignmentList(t, doAuthedRequest(t, handler, lager.Token, http.MethodGet, aaPath, "")); len(list) != 0 {
		t.Fatalf("Lager: eigene Liste muss leer sein: %+v", list)
	}
	wantAPIError(t, doAuthedRequest(t, handler, lager.Token, http.MethodGet, aaPath+own.ID.String(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	// self ohne verknuepften Mitarbeiter.
	unlinked := tenant.user("production")
	wantAPIError(t, doAuthedRequest(t, handler, unlinked, http.MethodGet, aaPath, ""), http.StatusForbidden, "forbidden", "Kein Mitarbeiter mit diesem Benutzer verknüpft")

	// Rollen mit genau einem Recht.
	readOnly := tenant.user("itest-aa-readonly")
	if nums := assignmentNumbers(decodeAssignmentList(t, doAuthedRequest(t, handler, readOnly, http.MethodGet, aaPath, ""))); nums != "F-1,M-1" && nums != "M-1,F-1" {
		t.Fatalf("read: alle Ausgaben erwartet: %s", nums)
	}
	decodeAssignment(t, doAuthedRequest(t, handler, readOnly, http.MethodGet, aaPath+foreign.ID.String(), ""), http.StatusOK)
	writeOnly := tenant.user("itest-aa-writeonly")
	none := tenant.user("itest-aa-none")
	for _, tok := range []string{writeOnly, none} {
		wantAPIError(t, doAuthedRequest(t, handler, tok, http.MethodGet, aaPath, ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
		wantAPIError(t, doAuthedRequest(t, handler, tok, http.MethodGet, aaPath+own.ID.String(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
		// Ohne Leserecht verraet auch eine unbekannte ID nichts: 403, nicht 404.
		wantAPIError(t, doAuthedRequest(t, handler, tok, http.MethodGet, aaPath+uuid.NewString(), ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	}

	// Schreiben: nur write.
	for _, tc := range []struct {
		name     string
		token    string
		canWrite bool
	}{
		{"production (self)", monteur.Token, false},
		{"inventory (self)", lager.Token, false},
		{"nur read", readOnly, false},
		{"keines", none, false},
		{"nur write", writeOnly, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := tenant.asset(hrToken, "W-"+uuid.NewString()[:8], "")
			rec := doAuthedRequest(t, handler, tc.token, http.MethodPost, aaPath, issueBody(a.ID, fremd, "2026-01-05"))
			if !tc.canWrite {
				wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				x := decodeAssignment(t, doAuthedRequest(t, handler, hrToken, http.MethodPost, aaPath, issueBody(a.ID, fremd, "2026-01-05")), http.StatusCreated)
				wantAPIError(t, doAuthedRequest(t, handler, tc.token, http.MethodPost, aaPath+x.ID.String()+"/return", `{"datum":"2026-01-06"}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")
				wantAPIError(t, doAuthedRequest(t, handler, tc.token, http.MethodPost, aaPath+x.ID.String()+"/storno", `{"reason":"x"}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")
				return
			}
			x := decodeAssignment(t, rec, http.StatusCreated)
			decodeAssignment(t, doAuthedRequest(t, handler, tc.token, http.MethodPost, aaPath+x.ID.String()+"/return", `{"datum":"2026-01-06"}`), http.StatusOK)
			decodeAssignment(t, doAuthedRequest(t, handler, tc.token, http.MethodPost, aaPath+x.ID.String()+"/storno", `{"reason":"x"}`), http.StatusOK)
		})
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, aaPath, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: erwartet 401, bekommen %d", rec.Code)
	}
}

func TestAssetAssignmentsHTTPAreIsolatedBetweenCompanies(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	one, two := newCatalogTenant(t, env, handler), newCatalogTenant(t, env, handler)
	tokenOne, tokenTwo := one.user("hr"), two.user("admin")
	empOne := one.employee("Eins")
	empTwo := two.employee("Zwei")
	assetOne := one.asset(tokenOne, "L-1", "")
	assetTwo := two.asset(tokenTwo, "L-1", "")

	x := decodeAssignment(t, doAuthedRequest(t, handler, tokenOne, http.MethodPost, aaPath, issueBody(assetOne.ID, empOne, "2026-01-05")), http.StatusCreated)

	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, aaPath+x.ID.String(), ""), http.StatusNotFound, "not_found", "Ausgabe nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, aaPath+x.ID.String()+"/return", `{"datum":"2026-01-06"}`), http.StatusNotFound, "not_found", "Ausgabe nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, aaPath+x.ID.String()+"/storno", `{"reason":"x"}`), http.StatusNotFound, "not_found", "Ausgabe nicht gefunden")
	if list := decodeAssignmentList(t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, aaPath+"?include_storniert=true", "")); len(list) != 0 {
		t.Fatalf("Liste des anderen Mandanten muss leer sein: %+v", list)
	}
	// Ueber Kreuz: fremdes Asset bzw. fremder Mitarbeiter.
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, aaPath, issueBody(assetOne.ID, empTwo, "2026-01-05")), http.StatusNotFound, "not_found", "Asset nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, aaPath, issueBody(assetTwo.ID, empOne, "2026-01-05")), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	if still := decodeAssignment(t, doAuthedRequest(t, handler, tokenOne, http.MethodGet, aaPath+x.ID.String(), ""), http.StatusOK); still.ZurueckgegebenAm != nil || still.Status != "active" {
		t.Fatalf("anderer Mandant hat die Ausgabe veraendert: %+v", still)
	}
}

// Jeder Fehlertext der Ausgaben muss als 4xx klassifiziert werden. DB-los.
func TestClassifyDomainErrorCoversAssetAssignmentMessages(t *testing.T) {
	cases := map[string]int{
		"asset_id erforderlich":                                               http.StatusBadRequest,
		"employee_id erforderlich":                                            http.StatusBadRequest,
		"Ausgabe-ID erforderlich":                                             http.StatusBadRequest,
		"Ausgabedatum erforderlich":                                           http.StatusBadRequest,
		"Ausgabedatum darf nicht in der Zukunft liegen":                       http.StatusBadRequest,
		"Ausgabedatum darf nicht vor dem Anschaffungsdatum liegen":            http.StatusBadRequest,
		"Rückgabedatum erforderlich":                                          http.StatusBadRequest,
		"Rückgabedatum darf nicht in der Zukunft liegen":                      http.StatusBadRequest,
		"Rückgabedatum darf nicht vor dem Ausgabedatum liegen":                http.StatusBadRequest,
		"Storno-Begründung erforderlich":                                      http.StatusBadRequest,
		"Asset nicht gefunden":                                                http.StatusNotFound,
		"Mitarbeiter nicht gefunden":                                          http.StatusNotFound,
		"Ausgabe nicht gefunden":                                              http.StatusNotFound,
		"Asset ist ausgemustert":                                              http.StatusConflict,
		"Mitarbeiter ist nicht aktiv":                                         http.StatusConflict,
		"Asset ist bereits ausgegeben":                                        http.StatusConflict,
		"Asset ist gesperrt: letzte Prüfung nicht bestanden":                  http.StatusConflict,
		"Ausgabe überschneidet sich mit einer früheren Ausgabe dieses Assets": http.StatusConflict,
		"Ausgabe ist storniert":                                               http.StatusConflict,
		"Asset wurde bereits zurückgegeben":                                   http.StatusConflict,
		"Ausgabe ist bereits storniert":                                       http.StatusConflict,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
