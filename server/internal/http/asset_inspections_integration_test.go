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

	"nalaerp3/internal/hr"
	"nalaerp3/internal/testutil"
)

// HTTP-Anbindung der Pruefungen, des Pruefstands und der
// Faelligkeitsliste (Backlog F.3.6.3).
//
// Stand und Liste rechnen mit "heute". Die Daten sind deshalb so gewaehlt,
// dass ihr Stand an jedem Tag gleich ist (2025 durchgefallen, 2024
// ueberfaellig, Folgetermin 2099), plus genau eine Pruefung relativ zu
// heute (faellig in 10 Tagen).

type inspectionResponse struct {
	ID                 uuid.UUID  `json:"id"`
	AssetID            uuid.UUID  `json:"asset_id"`
	GeprueftAm         string     `json:"geprueft_am"`
	Ergebnis           string     `json:"ergebnis"`
	NaechstePruefungAm *string    `json:"naechste_pruefung_am"`
	PrueferName        string     `json:"pruefer_name"`
	PrueferEmployeeID  *uuid.UUID `json:"pruefer_employee_id"`
	Notiz              string     `json:"notiz"`
	Status             string     `json:"status"`
	StornoReason       string     `json:"storno_reason"`
	StorniertByUserID  *string    `json:"storniert_by_user_id"`
	CreatedByUserID    *string    `json:"created_by_user_id"`
}

type assetStandingResponse struct {
	Status         string     `json:"status"`
	InspectionID   *uuid.UUID `json:"inspection_id"`
	Ergebnis       string     `json:"ergebnis"`
	TageBisFaellig *int       `json:"tage_bis_faellig"`
	Maengel        bool       `json:"maengel"`
}

type assetDueResponse struct {
	AssetID         uuid.UUID  `json:"asset_id"`
	Inventarnummer  string     `json:"inventarnummer"`
	Standort        string     `json:"standort"`
	EmployeeID      *uuid.UUID `json:"employee_id"`
	MitarbeiterName string     `json:"mitarbeiter_name"`
	Status          string     `json:"status"`
	TageBisFaellig  *int       `json:"tage_bis_faellig"`
	Maengel         bool       `json:"maengel"`
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) T {
	t.Helper()
	var out T
	if rec.Code != wantStatus {
		t.Fatalf("erwartet Status %d, bekommen %d mit Body %s", wantStatus, rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v (%s)", err, rec.Body.String())
	}
	return out
}

const aiPath = "/api/v1/asset-inspections/"

func inspBody(asset uuid.UUID, geprueft, ergebnis, extra string) string {
	return `{"asset_id":"` + asset.String() + `","geprueft_am":"` + geprueft + `","ergebnis":"` + ergebnis + `","pruefer_name":"Elektro Meier"` + extra + `}`
}

func TestAssetInspectionsHTTPFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	other := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, hrToken, method, path, body)
	}
	bohrer := tenant.asset(hrToken, "E-1", `,"pruefintervall_monate":6`)
	pruefer := tenant.employee("Pruefer")

	// Ohne Folgetermin: aus dem Intervall, am Monatsende begrenzt.
	first := decodeJSON[inspectionResponse](t, do(http.MethodPost, aiPath, inspBody(bohrer.ID, "2025-08-31", "bestanden", `,"pruefer_employee_id":"`+pruefer.String()+`","notiz":"Isolation ok"`)), http.StatusCreated)
	if !strings.HasPrefix(first.GeprueftAm, "2025-08-31") || first.NaechstePruefungAm == nil || !strings.HasPrefix(*first.NaechstePruefungAm, "2026-02-28") ||
		first.PrueferName != "Elektro Meier" || first.PrueferEmployeeID == nil || *first.PrueferEmployeeID != pruefer || first.Notiz != "Isolation ok" ||
		first.CreatedByUserID == nil || first.Status != "active" || first.AssetID != bohrer.ID {
		t.Fatalf("Pruefung nicht wie erwartet: %+v", first)
	}
	explicit := decodeJSON[inspectionResponse](t, do(http.MethodPost, aiPath, inspBody(bohrer.ID, "2026-02-20", "maengel", `,"naechste_pruefung_am":"2026-05-20"`)), http.StatusCreated)
	if explicit.NaechstePruefungAm == nil || !strings.HasPrefix(*explicit.NaechstePruefungAm, "2026-05-20") || explicit.Ergebnis != "maengel" {
		t.Fatalf("angegebener Folgetermin: %+v", explicit)
	}
	got := decodeJSON[inspectionResponse](t, do(http.MethodGet, aiPath+first.ID.String(), ""), http.StatusOK)
	if got.ID != first.ID || got.Notiz != "Isolation ok" {
		t.Fatalf("Einzelabruf: %+v", got)
	}

	// Liste eines Assets: juengste zuerst; Stornierte nur auf Wunsch.
	list := decodeJSON[[]inspectionResponse](t, do(http.MethodGet, aiPath+"?asset_id="+bohrer.ID.String(), ""), http.StatusOK)
	if len(list) != 2 || list[0].ID != explicit.ID || list[1].ID != first.ID {
		t.Fatalf("Liste: %+v", list)
	}
	wantAPIError(t, do(http.MethodPost, aiPath+explicit.ID.String()+"/storno", `{"reason":" "}`), http.StatusBadRequest, "validation_error", "Storno-Begründung erforderlich")
	st := decodeJSON[inspectionResponse](t, do(http.MethodPost, aiPath+explicit.ID.String()+"/storno", `{"reason":"falsches Geraet"}`), http.StatusOK)
	if st.Status != "storniert" || st.StornoReason != "falsches Geraet" || st.StorniertByUserID == nil {
		t.Fatalf("Storno: %+v", st)
	}
	wantAPIError(t, do(http.MethodPost, aiPath+explicit.ID.String()+"/storno", `{"reason":"nochmal"}`), http.StatusConflict, "conflict", "Prüfung ist bereits storniert")
	if list := decodeJSON[[]inspectionResponse](t, do(http.MethodGet, aiPath+"?asset_id="+bohrer.ID.String(), ""), http.StatusOK); len(list) != 1 || list[0].ID != first.ID {
		t.Fatalf("ohne Stornierte: %+v", list)
	}
	if list := decodeJSON[[]inspectionResponse](t, do(http.MethodGet, aiPath+"?asset_id="+bohrer.ID.String()+"&include_storniert=true", ""), http.StatusOK); len(list) != 2 {
		t.Fatalf("mit Stornierten: %+v", list)
	}

	// Fehlerfaelle mit Status, Code und Text.
	wantAPIError(t, do(http.MethodPost, aiPath, inspBody(bohrer.ID, "2026-01-05", "nicht_bestanden", `,"naechste_pruefung_am":"2027-01-01"`)), http.StatusBadRequest, "validation_error", "Bei nicht bestandener Prüfung gibt es keine nächste Prüfung")
	wantAPIError(t, do(http.MethodPost, aiPath, inspBody(bohrer.ID, "2026-01-05", "bestanden", `,"naechste_pruefung_am":"2026-01-05"`)), http.StatusBadRequest, "validation_error", "Nächste Prüfung muss nach dem Prüfdatum liegen")
	wantAPIError(t, do(http.MethodPost, aiPath, inspBody(bohrer.ID, "2026-01-05", "gut", "")), http.StatusBadRequest, "validation_error", "Ungültiges Prüfergebnis")
	wantAPIError(t, do(http.MethodPost, aiPath, `{"asset_id":"`+bohrer.ID.String()+`","geprueft_am":"2026-01-05","ergebnis":"bestanden"}`), http.StatusBadRequest, "validation_error", "Prüfer erforderlich")
	wantAPIError(t, do(http.MethodPost, aiPath, inspBody(bohrer.ID, "", "bestanden", "")), http.StatusBadRequest, "validation_error", "Prüfdatum erforderlich")
	wantAPIError(t, do(http.MethodPost, aiPath, inspBody(bohrer.ID, "05.01.2026", "bestanden", "")), http.StatusBadRequest, "validation_error", "Ungültiges Datum in geprueft_am (erwartet JJJJ-MM-TT)")
	wantAPIError(t, do(http.MethodPost, aiPath, inspBody(bohrer.ID, "2026-01-05", "bestanden", `,"naechste_pruefung_am":"bald"`)), http.StatusBadRequest, "validation_error", "Ungültiges Datum in naechste_pruefung_am (erwartet JJJJ-MM-TT)")
	wantAPIError(t, do(http.MethodPost, aiPath, inspBody(bohrer.ID, time.Now().AddDate(0, 0, 2).Format("2006-01-02"), "bestanden", "")), http.StatusBadRequest, "validation_error", "Prüfdatum darf nicht in der Zukunft liegen")
	wantAPIError(t, do(http.MethodPost, aiPath, inspBody(bohrer.ID, "2026-01-05", "bestanden", `,"pruefer_employee_id":"`+other.employee("Fremd").String()+`"`)), http.StatusNotFound, "not_found", "Mitarbeiter nicht gefunden")
	wantAPIError(t, do(http.MethodPost, aiPath, inspBody(uuid.New(), "2026-01-05", "bestanden", "")), http.StatusNotFound, "not_found", "Asset nicht gefunden")
	wantAPIError(t, do(http.MethodPost, aiPath, `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")
	alt := tenant.asset(hrToken, "ALT-1", "")
	decodeAsset(t, do(http.MethodPost, assetPath+alt.ID.String()+"/retire", `{"datum":"2026-01-02","grund":"kaputt"}`), http.StatusOK)
	wantAPIError(t, do(http.MethodPost, aiPath, inspBody(alt.ID, "2026-01-05", "bestanden", "")), http.StatusConflict, "conflict", "Asset ist ausgemustert")
	wantAPIError(t, do(http.MethodGet, aiPath, ""), http.StatusBadRequest, "validation_error", "asset_id erforderlich")
	wantAPIError(t, do(http.MethodGet, aiPath+"?asset_id=kaputt", ""), http.StatusBadRequest, "validation_error", "Ungültige asset_id")
	wantAPIError(t, do(http.MethodGet, aiPath+"?asset_id="+uuid.NewString(), ""), http.StatusNotFound, "not_found", "Asset nicht gefunden")
	wantAPIError(t, do(http.MethodGet, aiPath+uuid.NewString(), ""), http.StatusNotFound, "not_found", "Prüfung nicht gefunden")
	wantAPIError(t, do(http.MethodGet, aiPath+"kein-uuid", ""), http.StatusBadRequest, "validation_error", "Ungültige Prüfungs-ID")
	wantAPIError(t, do(http.MethodPost, aiPath+"kein-uuid/storno", `{}`), http.StatusBadRequest, "validation_error", "Ungültige Prüfungs-ID")
	wantAPIError(t, do(http.MethodPost, aiPath+uuid.NewString()+"/storno", `{"reason":"x"}`), http.StatusNotFound, "not_found", "Prüfung nicht gefunden")
}

func TestAssetInspectionsHTTPStandingAndDue(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, hrToken, method, path, body)
	}
	today := hr.WorkDateOf(time.Now())

	gesperrt := tenant.asset(hrToken, "Z-GES", `,"pruefintervall_monate":12`)
	decodeJSON[inspectionResponse](t, do(http.MethodPost, aiPath, inspBody(gesperrt.ID, "2025-01-10", "nicht_bestanden", "")), http.StatusCreated)
	ueber := tenant.asset(hrToken, "B-UE", `,"pruefintervall_monate":6,"standort":"Halle 2"`)
	decodeJSON[inspectionResponse](t, do(http.MethodPost, aiPath, inspBody(ueber.ID, "2024-01-10", "bestanden", `,"naechste_pruefung_am":"2024-07-10"`)), http.StatusCreated)
	// Maengel sperren (F.3.7, Entscheidung des Nutzers vom 2026-10-19).
	maengel := tenant.asset(hrToken, "A-MAE", `,"pruefintervall_monate":12`)
	decodeJSON[inspectionResponse](t, do(http.MethodPost, aiPath, inspBody(maengel.ID, "2025-03-10", "maengel", `,"naechste_pruefung_am":"2099-01-01"`)), http.StatusCreated)
	nie := tenant.asset(hrToken, "C-NIE", `,"pruefintervall_monate":12`)
	bald := tenant.asset(hrToken, "D-BALD", `,"pruefintervall_monate":12`)
	decodeJSON[inspectionResponse](t, do(http.MethodPost, aiPath, inspBody(bald.ID, today.AddDate(0, -1, 0).Format("2006-01-02"), "bestanden",
		`,"naechste_pruefung_am":"`+today.AddDate(0, 0, 10).Format("2006-01-02")+`"`)), http.StatusCreated)
	ok := tenant.asset(hrToken, "E-OK", `,"pruefintervall_monate":12`)
	decodeJSON[inspectionResponse](t, do(http.MethodPost, aiPath, inspBody(ok.ID, "2025-01-10", "bestanden", `,"naechste_pruefung_am":"2099-01-01"`)), http.StatusCreated)
	// Faellig in 45 Tagen: mit dem Standard von 30 Tagen noch "geprueft"
	// (bei den 60 Tagen der Qualifikationen waere es "bald faellig").
	spaeter := tenant.asset(hrToken, "G-45", `,"pruefintervall_monate":12`)
	decodeJSON[inspectionResponse](t, do(http.MethodPost, aiPath, inspBody(spaeter.ID, today.AddDate(0, -1, 0).Format("2006-01-02"), "bestanden",
		`,"naechste_pruefung_am":"`+today.AddDate(0, 0, 45).Format("2006-01-02")+`"`)), http.StatusCreated)
	frei := tenant.asset(hrToken, "F-FREI", "")
	halter := tenant.employee("Halter")
	decodeAssignment(t, do(http.MethodPost, aaPath, issueBody(ueber.ID, halter, "2024-02-01")), http.StatusCreated)
	wantAPIError(t, do(http.MethodPost, aaPath, issueBody(maengel.ID, halter, "2025-03-11")), http.StatusConflict, "conflict", "Asset ist gesperrt: letzte Prüfung mit Mängeln")

	// Pruefstand je Asset.
	for _, tc := range []struct {
		asset assetResponse
		want  string
	}{
		{gesperrt, "gesperrt"}, {maengel, "gesperrt"}, {ueber, "ueberfaellig"}, {nie, "nie_geprueft"}, {bald, "faellig_bald"}, {ok, "geprueft"}, {spaeter, "geprueft"}, {frei, "nicht_pruefpflichtig"},
	} {
		st := decodeJSON[assetStandingResponse](t, do(http.MethodGet, assetPath+tc.asset.ID.String()+"/inspection-standing", ""), http.StatusOK)
		if st.Status != tc.want {
			t.Fatalf("%s: erwartet %s, bekommen %+v", tc.asset.Inventarnummer, tc.want, st)
		}
	}
	st := decodeJSON[assetStandingResponse](t, do(http.MethodGet, assetPath+bald.ID.String()+"/inspection-standing", ""), http.StatusOK)
	if st.TageBisFaellig == nil || *st.TageBisFaellig != 10 {
		t.Fatalf("Tage bis faellig: %+v", st)
	}
	if st := decodeJSON[assetStandingResponse](t, do(http.MethodGet, assetPath+spaeter.ID.String()+"/inspection-standing?warn_tage=60", ""), http.StatusOK); st.Status != "faellig_bald" {
		t.Fatalf("Vorwarnzeit 60: %+v", st)
	}
	if st := decodeJSON[assetStandingResponse](t, do(http.MethodGet, assetPath+bald.ID.String()+"/inspection-standing?warn_tage=5", ""), http.StatusOK); st.Status != "geprueft" {
		t.Fatalf("Vorwarnzeit 5: %+v", st)
	}
	if st := decodeJSON[assetStandingResponse](t, do(http.MethodGet, assetPath+maengel.ID.String()+"/inspection-standing", ""), http.StatusOK); !st.Maengel || st.Ergebnis != "maengel" {
		t.Fatalf("gesperrt wegen Maengeln erwartet: %+v", st)
	}
	if st := decodeJSON[assetStandingResponse](t, do(http.MethodGet, assetPath+gesperrt.ID.String()+"/inspection-standing", ""), http.StatusOK); st.Maengel {
		t.Fatalf("gesperrt wegen 'nicht bestanden', nicht wegen Maengeln: %+v", st)
	}
	wantAPIError(t, do(http.MethodGet, assetPath+bald.ID.String()+"/inspection-standing?warn_tage=-1", ""), http.StatusBadRequest, "validation_error", "Ungültige Vorwarnzeit (warn_tage: ganze Zahl ab 0)")
	wantAPIError(t, do(http.MethodGet, assetPath+uuid.NewString()+"/inspection-standing", ""), http.StatusNotFound, "not_found", "Asset nicht gefunden")
	wantAPIError(t, do(http.MethodGet, assetPath+"kein-uuid/inspection-standing", ""), http.StatusBadRequest, "validation_error", "Ungültige Asset-ID")

	// Faelligkeitsliste: Rangfolge, Halter, Vorwarnzeit.
	due := decodeJSON[[]assetDueResponse](t, do(http.MethodGet, aiPath+"due", ""), http.StatusOK)
	got := make([]string, 0, len(due))
	for _, e := range due {
		got = append(got, e.Inventarnummer+":"+e.Status)
	}
	if strings.Join(got, ",") != "A-MAE:gesperrt,Z-GES:gesperrt,B-UE:ueberfaellig,C-NIE:nie_geprueft,D-BALD:faellig_bald" {
		t.Fatalf("Liste: %s", strings.Join(got, ","))
	}
	if e := due[0]; !e.Maengel {
		t.Fatalf("Eintrag A-MAE: %+v", e)
	}
	if e := due[2]; e.EmployeeID == nil || *e.EmployeeID != halter || e.MitarbeiterName != "Quali Halter" || e.Standort != "Halle 2" || e.Maengel {
		t.Fatalf("Eintrag B-UE: %+v", e)
	}
	if e := due[4]; e.TageBisFaellig == nil || *e.TageBisFaellig != 10 {
		t.Fatalf("Eintrag D-BALD: %+v", e)
	}
	if long := decodeJSON[[]assetDueResponse](t, do(http.MethodGet, aiPath+"due?warn_tage=60", ""), http.StatusOK); len(long) != 6 || long[5].Inventarnummer != "G-45" {
		t.Fatalf("Vorwarnzeit 60: G-45 als sechster Eintrag erwartet, bekommen %+v", long)
	}
	if short := decodeJSON[[]assetDueResponse](t, do(http.MethodGet, aiPath+"due?warn_tage=5", ""), http.StatusOK); len(short) != 4 {
		t.Fatalf("Vorwarnzeit 5: erwartet 4 Eintraege, bekommen %+v", short)
	}
	wantAPIError(t, do(http.MethodGet, aiPath+"due?warn_tage=x", ""), http.StatusBadRequest, "validation_error", "Ungültige Vorwarnzeit (warn_tage: ganze Zahl ab 0)")

	// Anderer Mandant: weder Stand noch Liste.
	other := newCatalogTenant(t, env, handler)
	otherToken := other.user("admin")
	wantAPIError(t, doAuthedRequest(t, handler, otherToken, http.MethodGet, assetPath+gesperrt.ID.String()+"/inspection-standing", ""), http.StatusNotFound, "not_found", "Asset nicht gefunden")
	if list := decodeJSON[[]assetDueResponse](t, doAuthedRequest(t, handler, otherToken, http.MethodGet, aiPath+"due", ""), http.StatusOK); len(list) != 0 {
		t.Fatalf("Liste des anderen Mandanten muss leer sein: %+v", list)
	}
}

// Lesen nur mit assets.read, schreiben nur mit assets.write; assets.self
// gibt hier nichts.
func TestAssetInspectionsHTTPPermissions(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	seedRoleWithPermissions(t, env, "itest-ai-readonly", "assets.read")
	seedRoleWithPermissions(t, env, "itest-ai-writeonly", "assets.write")
	tenant := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")
	a := tenant.asset(hrToken, "P-1", `,"pruefintervall_monate":12`)
	existing := decodeJSON[inspectionResponse](t, doAuthedRequest(t, handler, hrToken, http.MethodPost, aiPath, inspBody(a.ID, "2026-01-05", "bestanden", "")), http.StatusCreated)

	type call struct{ method, path, body string }
	reads := []call{
		{http.MethodGet, aiPath + "?asset_id=" + a.ID.String(), ""},
		{http.MethodGet, aiPath + existing.ID.String(), ""},
		{http.MethodGet, aiPath + "due", ""},
		{http.MethodGet, assetPath + a.ID.String() + "/inspection-standing", ""},
	}
	for _, tc := range []struct {
		role              string
		canRead, canWrite bool
	}{
		{"hr", true, true},
		{"admin", true, true},
		{"inventory", false, false},
		{"production", false, false},
		{"itest-ai-readonly", true, false},
		{"itest-ai-writeonly", false, true},
	} {
		t.Run(tc.role, func(t *testing.T) {
			token := tenant.user(tc.role)
			for _, c := range reads {
				rec := doAuthedRequest(t, handler, token, c.method, c.path, c.body)
				if tc.canRead && rec.Code != http.StatusOK {
					t.Fatalf("%s %s: erwartet 200, bekommen %d (%s)", c.method, c.path, rec.Code, rec.Body.String())
				}
				if !tc.canRead {
					wantAPIError(t, rec, http.StatusForbidden, "forbidden", "Keine Berechtigung")
				}
			}
			x := decodeJSON[inspectionResponse](t, doAuthedRequest(t, handler, hrToken, http.MethodPost, aiPath, inspBody(a.ID, "2026-01-06", "bestanden", "")), http.StatusCreated)
			post := doAuthedRequest(t, handler, token, http.MethodPost, aiPath, inspBody(a.ID, "2026-01-07", "bestanden", ""))
			storno := doAuthedRequest(t, handler, token, http.MethodPost, aiPath+x.ID.String()+"/storno", `{"reason":"x"}`)
			if tc.canWrite {
				if post.Code != http.StatusCreated || storno.Code != http.StatusOK {
					t.Fatalf("Schreiben erwartet: %d (%s), %d (%s)", post.Code, post.Body.String(), storno.Code, storno.Body.String())
				}
				return
			}
			wantAPIError(t, post, http.StatusForbidden, "forbidden", "Keine Berechtigung")
			wantAPIError(t, storno, http.StatusForbidden, "forbidden", "Keine Berechtigung")
		})
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, aiPath+"due", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: erwartet 401, bekommen %d", rec.Code)
	}
}

func TestAssetInspectionsHTTPAreIsolatedBetweenCompanies(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	one, two := newCatalogTenant(t, env, handler), newCatalogTenant(t, env, handler)
	tokenOne, tokenTwo := one.user("hr"), two.user("admin")
	a := one.asset(tokenOne, "P-1", "")
	x := decodeJSON[inspectionResponse](t, doAuthedRequest(t, handler, tokenOne, http.MethodPost, aiPath, inspBody(a.ID, "2026-01-05", "nicht_bestanden", "")), http.StatusCreated)

	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, aiPath+x.ID.String(), ""), http.StatusNotFound, "not_found", "Prüfung nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, aiPath+x.ID.String()+"/storno", `{"reason":"x"}`), http.StatusNotFound, "not_found", "Prüfung nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodGet, aiPath+"?asset_id="+a.ID.String(), ""), http.StatusNotFound, "not_found", "Asset nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, tokenTwo, http.MethodPost, aiPath, inspBody(a.ID, "2026-01-06", "bestanden", "")), http.StatusNotFound, "not_found", "Asset nicht gefunden")
	if still := decodeJSON[inspectionResponse](t, doAuthedRequest(t, handler, tokenOne, http.MethodGet, aiPath+x.ID.String(), ""), http.StatusOK); still.Status != "active" {
		t.Fatalf("anderer Mandant hat die Pruefung veraendert: %+v", still)
	}
	if list := decodeJSON[[]inspectionResponse](t, doAuthedRequest(t, handler, tokenOne, http.MethodGet, aiPath+"?asset_id="+a.ID.String(), ""), http.StatusOK); len(list) != 1 {
		t.Fatalf("anderer Mandant hat eine Pruefung angelegt: %+v", list)
	}
	var n int
	if err := env.PG.QueryRow(context.Background(), `SELECT count(*) FROM asset_inspections WHERE asset_id=$1`, a.ID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("erwartet genau 1 Pruefung, bekommen %d (%v)", n, err)
	}
}

// Jeder Fehlertext der Pruefungen muss als 4xx klassifiziert werden. DB-los.
func TestClassifyDomainErrorCoversAssetInspectionMessages(t *testing.T) {
	cases := map[string]int{
		"Prüfdatum erforderlich":                                      http.StatusBadRequest,
		"Prüfdatum darf nicht in der Zukunft liegen":                  http.StatusBadRequest,
		"Prüfdatum darf nicht vor dem Anschaffungsdatum liegen":       http.StatusBadRequest,
		"Ungültiges Prüfergebnis":                                     http.StatusBadRequest,
		"Prüfer erforderlich":                                         http.StatusBadRequest,
		"Bei nicht bestandener Prüfung gibt es keine nächste Prüfung": http.StatusBadRequest,
		"Nächste Prüfung muss nach dem Prüfdatum liegen":              http.StatusBadRequest,
		"Prüfungs-ID erforderlich":                                    http.StatusBadRequest,
		"Storno-Begründung erforderlich":                              http.StatusBadRequest,
		"Prüfung nicht gefunden":                                      http.StatusNotFound,
		"Asset nicht gefunden":                                        http.StatusNotFound,
		"Mitarbeiter nicht gefunden":                                  http.StatusNotFound,
		"Asset ist ausgemustert":                                      http.StatusConflict,
		"Prüfung ist bereits storniert":                               http.StatusConflict,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
