package apihttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/hr"
	"nalaerp3/internal/testutil"
)

// Zwischenbestaetigungen ueber HTTP (Backlog F.10.3.3).

type qualConfirmationResponse struct {
	ID              uuid.UUID `json:"id"`
	QualificationID uuid.UUID `json:"qualification_id"`
	BestaetigtAm    string    `json:"bestaetigt_am"`
	BestaetigtDurch string    `json:"bestaetigt_durch"`
	Note            string    `json:"note"`
	Status          string    `json:"status"`
	StornoReason    string    `json:"storno_reason"`
	CreatedByUserID *string   `json:"created_by_user_id"`
}

func confirmationBody(am, durch string) string {
	return `{"bestaetigt_am":"` + am + `","bestaetigt_durch":"` + durch + `","note":"Baustelle Nord"}`
}

func TestQualificationConfirmationsHTTP(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	tenant := newCatalogTenant(t, env, handler)
	other := newCatalogTenant(t, env, handler)
	hrToken := tenant.user("hr")
	otherHR := other.user("hr")
	worker := tenant.linkedUser("production", "Schweisser")
	colleague := tenant.linkedUser("production", "Kollege")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doAuthedRequest(t, handler, hrToken, method, path, body)
	}

	weld := decodeQualificationType(t, do(http.MethodPost, qtPath,
		`{"code":"SCHWEISSER","name":"Schweißerprüfung","kategorie":"qualifikation","gueltigkeit_monate":36,"bestaetigung_monate":6}`), http.StatusCreated)
	plain := tenant.qualType(hrToken, "STAPLER", "null")
	rec := decodeQualRecords(t, do(http.MethodPost, qualPath, qualBody(weld.ID, "2026-01-15", worker.EmployeeID)), http.StatusCreated)[0]
	plainRec := decodeQualRecords(t, do(http.MethodPost, qualPath, qualBody(plain.ID, "2026-01-15", worker.EmployeeID)), http.StatusCreated)[0]
	confPath := qualPath + rec.ID.String() + "/confirmations"

	// Erfassen.
	var first qualConfirmationResponse
	decodeJSONStatus(t, do(http.MethodPost, confPath, confirmationBody("2026-07-10", "Schweissaufsicht")), http.StatusCreated, &first)
	if first.QualificationID != rec.ID || first.Status != "active" || jsonDay(&first.BestaetigtAm) != "2026-07-10" ||
		first.BestaetigtDurch != "Schweissaufsicht" || first.Note != "Baustelle Nord" || first.CreatedByUserID == nil {
		t.Fatalf("Bestaetigung nicht wie erfasst: %+v", first)
	}

	// Fehlerfaelle beim Erfassen.
	future := hr.WorkDateOf(time.Now()).AddDate(0, 0, 2).Format("2006-01-02")
	wantAPIError(t, do(http.MethodPost, confPath, confirmationBody("2026-07-10", "Schweissaufsicht")), http.StatusConflict, "conflict", "Bestätigung für diesen Tag ist bereits erfasst")
	wantAPIError(t, do(http.MethodPost, confPath, confirmationBody("10.07.2026", "Schweissaufsicht")), http.StatusBadRequest, "validation_error", "Ungültiges Datum in bestaetigt_am (erwartet JJJJ-MM-TT)")
	wantAPIError(t, do(http.MethodPost, confPath, `{"bestaetigt_durch":"Schweissaufsicht"}`), http.StatusBadRequest, "validation_error", "Bestätigungsdatum erforderlich")
	wantAPIError(t, do(http.MethodPost, confPath, confirmationBody("2026-07-11", " ")), http.StatusBadRequest, "validation_error", "Bestätigt durch erforderlich")
	wantAPIError(t, do(http.MethodPost, confPath, confirmationBody(future, "Schweissaufsicht")), http.StatusBadRequest, "validation_error", "Bestätigungsdatum darf nicht in der Zukunft liegen")
	wantAPIError(t, do(http.MethodPost, confPath, confirmationBody("2026-01-14", "Schweissaufsicht")), http.StatusBadRequest, "validation_error", "Bestätigungsdatum darf nicht vor dem Erwerbsdatum liegen")
	wantAPIError(t, do(http.MethodPost, qualPath+plainRec.ID.String()+"/confirmations", confirmationBody("2026-07-10", "Schweissaufsicht")), http.StatusBadRequest, "validation_error", "Qualifikationsart sieht keine Zwischenbestätigung vor")
	wantAPIError(t, do(http.MethodPost, qualPath+uuid.NewString()+"/confirmations", confirmationBody("2026-07-10", "Schweissaufsicht")), http.StatusNotFound, "not_found", "Nachweis nicht gefunden")
	wantAPIError(t, do(http.MethodPost, qualPath+"kein-uuid/confirmations", confirmationBody("2026-07-10", "Schweissaufsicht")), http.StatusBadRequest, "validation_error", "Ungültige Nachweis-ID")
	wantAPIError(t, do(http.MethodPost, confPath, `{`), http.StatusBadRequest, "validation_error", "Ungültige Eingabe")

	// Lesen: eigene mit self, fremde nur mit read.
	var list []qualConfirmationResponse
	decodeJSONStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, confPath, ""), http.StatusOK, &list)
	if len(list) != 1 || list[0].ID != first.ID {
		t.Fatalf("eigene Liste: %+v", list)
	}
	wantAPIError(t, doAuthedRequest(t, handler, colleague.Token, http.MethodGet, confPath, ""), http.StatusForbidden, "forbidden", "Keine Berechtigung")

	// Der Stand zeigt die Faelligkeit der Bestaetigung.
	var standing []struct {
		TypeID                uuid.UUID `json:"type_id"`
		BestaetigungFaelligAm *string   `json:"bestaetigung_faellig_am"`
	}
	decodeJSONStatus(t, doAuthedRequest(t, handler, worker.Token, http.MethodGet, qualPath+"standing", ""), http.StatusOK, &standing)
	found := false
	for _, s := range standing {
		if s.TypeID == weld.ID {
			found = jsonDay(s.BestaetigungFaelligAm) == "2027-01-10"
		}
	}
	if !found {
		t.Fatalf("Stand ohne bestaetigung_faellig_am 2027-01-10: %+v", standing)
	}

	// Kein Selbsteintrag, kein Storno durch den Mitarbeiter.
	stornoPath := qualPath + "confirmations/" + first.ID.String() + "/storno"
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, confPath, confirmationBody("2026-07-12", "Selbst")), http.StatusForbidden, "forbidden", "Keine Berechtigung")
	wantAPIError(t, doAuthedRequest(t, handler, worker.Token, http.MethodPost, stornoPath, `{"reason":"Selbst"}`), http.StatusForbidden, "forbidden", "Keine Berechtigung")

	// Fremder Mandant findet nichts.
	wantAPIError(t, doAuthedRequest(t, handler, otherHR, http.MethodGet, confPath, ""), http.StatusNotFound, "not_found", "Nachweis nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, otherHR, http.MethodPost, confPath, confirmationBody("2026-07-12", "Fremd")), http.StatusNotFound, "not_found", "Nachweis nicht gefunden")
	wantAPIError(t, doAuthedRequest(t, handler, otherHR, http.MethodPost, stornoPath, `{"reason":"Fremd"}`), http.StatusNotFound, "not_found", "Bestätigung nicht gefunden")

	// Stornieren.
	wantAPIError(t, do(http.MethodPost, stornoPath, `{"reason":"  "}`), http.StatusBadRequest, "validation_error", "Storno-Begründung erforderlich")
	wantAPIError(t, do(http.MethodPost, qualPath+"confirmations/kein-uuid/storno", `{"reason":"x"}`), http.StatusBadRequest, "validation_error", "Ungültige Bestätigungs-ID")
	wantAPIError(t, do(http.MethodPost, qualPath+"confirmations/"+uuid.NewString()+"/storno", `{"reason":"x"}`), http.StatusNotFound, "not_found", "Bestätigung nicht gefunden")
	var storniert qualConfirmationResponse
	decodeJSONStatus(t, do(http.MethodPost, stornoPath, `{"reason":"Falsches Datum"}`), http.StatusOK, &storniert)
	if storniert.ID != first.ID || storniert.Status != "storniert" || storniert.StornoReason != "Falsches Datum" {
		t.Fatalf("Storno: %+v", storniert)
	}
	wantAPIError(t, do(http.MethodPost, stornoPath, `{"reason":"Nochmal"}`), http.StatusConflict, "conflict", "Bestätigung ist bereits storniert")

	list = nil
	decodeJSONStatus(t, do(http.MethodGet, confPath, ""), http.StatusOK, &list)
	if len(list) != 0 {
		t.Fatalf("Standardliste nach Storno: %+v", list)
	}
	decodeJSONStatus(t, do(http.MethodGet, confPath+"?include_storniert=true", ""), http.StatusOK, &list)
	if len(list) != 1 || list[0].Status != "storniert" {
		t.Fatalf("Liste mit stornierten: %+v", list)
	}
}

// Jeder Fehlertext der Bestaetigungen wird als 4xx klassifiziert. DB-los.
func TestClassifyDomainErrorCoversConfirmationMessages(t *testing.T) {
	cases := map[string]int{
		"Bestätigungsdatum erforderlich":                           http.StatusBadRequest,
		"Bestätigt durch erforderlich":                             http.StatusBadRequest,
		"Bestätigungs-ID erforderlich":                             http.StatusBadRequest,
		"Bestätigungsdatum darf nicht in der Zukunft liegen":       http.StatusBadRequest,
		"Bestätigungsdatum darf nicht vor dem Erwerbsdatum liegen": http.StatusBadRequest,
		"Bestätigungsdatum darf nicht nach dem Gültig-bis liegen":  http.StatusBadRequest,
		"Qualifikationsart sieht keine Zwischenbestätigung vor":    http.StatusBadRequest,
		"Bestätigung nicht gefunden":                               http.StatusNotFound,
		"Nachweis ist storniert":                                   http.StatusConflict,
		"Bestätigung für diesen Tag ist bereits erfasst":           http.StatusConflict,
		"Bestätigung ist bereits storniert":                        http.StatusConflict,
	}
	for msg, want := range cases {
		if got, _ := classifyDomainError(errors.New(msg)); got != want {
			t.Errorf("%q: erwartet %d, bekommen %d", msg, want, got)
		}
	}
}
