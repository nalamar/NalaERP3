package apihttp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"nalaerp3/internal/mail"
	"nalaerp3/internal/testutil"
)

// Routen /settings/mail (Backlog F.14.2c). Erfundene Zugangsdaten; der
// Versand geht an einen Test-Transport bzw. an einen geschlossenen Port.

const mailTestPassword = "erfundenes-Passwort-42"

type recordingTransport struct {
	cfgs []mail.SMTPConfig
	msgs []mail.Message
	// fail: echter Sender - der gespeicherte Server ist dann ein geschlossener Port.
	fail bool
}

func (r *recordingTransport) Send(ctx context.Context, cfg mail.SMTPConfig, msg mail.Message) error {
	if r.fail {
		return (&mail.Sender{}).Send(ctx, cfg, msg)
	}
	r.cfgs = append(r.cfgs, cfg)
	r.msgs = append(r.msgs, msg)
	return nil
}

func closedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(strings.Split(ln.Addr().String(), ":")[1])
	ln.Close()
	return port
}

func mailRequest(t *testing.T, h http.Handler, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd *bytes.Reader
	if body != "" {
		rd = bytes.NewReader([]byte(body))
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, "/api/v1"+path, rd)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func mailErrorMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body.Error.Message
}

func TestMailSettingsRoutes(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	if _, err := env.PG.Exec(ctx, `DELETE FROM company_mail_settings WHERE company_id='default'`); err != nil {
		t.Fatalf("aufraeumen: %v", err)
	}
	t.Cleanup(func() {
		_, _ = env.PG.Exec(context.Background(), `DELETE FROM company_mail_settings WHERE company_id='default'`)
	})
	cfg := *env.Cfg
	cfg.SettingsEncryptionKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32))
	cfg.MailRedirectTo = ""
	transport := &recordingTransport{}
	h := NewRouterWithDepsAndOptions(env.PG, env.Mongo, env.Redis, &cfg, V1RouterOptions{MailTransport: transport})
	testutil.SeedAuthUser(t, env, "mail-admin@example.test", "Secret123!", "admin")
	testutil.SeedAuthUser(t, env, "mail-sales@example.test", "Secret123!", "sales")
	admin := loginIntegrationUser(t, h, "mail-admin@example.test", "Secret123!")
	sales := loginIntegrationUser(t, h, "mail-sales@example.test", "Secret123!")

	// Ohne Anmeldung 401, ohne settings.manage 403 - auch fuer den Testversand.
	if rec := mailRequest(t, h, "", http.MethodGet, "/settings/mail/", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: %d", rec.Code)
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/settings/mail/", ""},
		{http.MethodPut, "/settings/mail/", `{"aktiv":false}`},
		{http.MethodPost, "/settings/mail/test", `{"an":"a@example.test"}`},
	} {
		if rec := mailRequest(t, h, sales, c.method, c.path, c.body); rec.Code != http.StatusForbidden {
			t.Fatalf("ohne Recht %s %s: %d", c.method, c.path, rec.Code)
		}
	}

	rec := mailRequest(t, h, admin, http.MethodGet, "/settings/mail/", "")
	var got mail.Settings
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got.Eingerichtet || got.SMTPPort != 465 || !got.SchluesselEingerichtet {
		t.Fatalf("leer: %d %s", rec.Code, rec.Body.String())
	}
	// Testversand ohne Zugang: 409.
	if rec := mailRequest(t, h, admin, http.MethodPost, "/settings/mail/test", `{"an":"a@example.test"}`); rec.Code != http.StatusConflict ||
		mailErrorMessage(t, rec) != "E-Mail-Versand ist nicht vollständig eingerichtet" {
		t.Fatalf("Test ohne Zugang: %d %s", rec.Code, rec.Body.String())
	}

	// Ungueltige Eingaben: 400 mit Meldung, nichts gespeichert.
	for body, want := range map[string]string{
		`{"sicherheit":"none"}`:                     "Ungültige Verschlüsselung (ssl oder starttls)",
		`{"smtp_host":"smtp://mail.example.test"}`:  "Mail-Server ungültig (nur der Name, z. B. mail.example.de)",
		`{"passwort":"x","passwort_loeschen":true}`: "Passwort entweder neu setzen oder löschen",
		`{"aktiv":true}`:                            "Für den Versand fehlt: Mail-Server, Benutzer, Absenderadresse",
		`{"aktiv":true,"smtp_host":"mail.example.test","benutzer":"erp@example.test","absender_adresse":"erp@example.test"}`: "Für den Versand fehlt: Passwort",
		`{"aktiv":`: "Ungültige Eingabe",
	} {
		rec := mailRequest(t, h, admin, http.MethodPut, "/settings/mail/", body)
		if rec.Code != http.StatusBadRequest || mailErrorMessage(t, rec) != want {
			t.Fatalf("%s: %d %s", body, rec.Code, rec.Body.String())
		}
	}

	port := closedPort(t)
	full := `{"aktiv":true,"smtp_host":"127.0.0.1","smtp_port":` + strconv.Itoa(port) + `,"sicherheit":"ssl","benutzer":"erp@example.test","passwort":"` + mailTestPassword + `","absender_adresse":"erp@example.test","absender_name":"Nala ERP"}`
	rec = mailRequest(t, h, admin, http.MethodPut, "/settings/mail/", full)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), mailTestPassword) {
		t.Fatalf("speichern: %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if !got.Aktiv || !got.PasswortGesetzt || !got.PasswortLesbar || got.UpdatedByUserID == nil {
		t.Fatalf("gespeichert: %+v", got)
	}
	if rec := mailRequest(t, h, admin, http.MethodGet, "/settings/mail/", ""); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), mailTestPassword) ||
		!strings.Contains(rec.Body.String(), `"passwort_gesetzt":true`) {
		t.Fatalf("lesen: %d %s", rec.Code, rec.Body.String())
	}

	// Testversand: Empfaenger Pflicht und gueltig; dann mit gespeichertem Zugang.
	for body, want := range map[string]string{
		`{}`: "Empfänger erforderlich",
		`{"an":"a@example.test, b@example.test"}`:             "Empfängeradresse ungültig",
		`{"an":"a@example.test\r\nRCPT TO:<b@example.test>"}`: "Empfängeradresse ungültig",
	} {
		rec := mailRequest(t, h, admin, http.MethodPost, "/settings/mail/test", body)
		if rec.Code != http.StatusBadRequest || mailErrorMessage(t, rec) != want {
			t.Fatalf("Test %s: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	transport.cfgs, transport.msgs = nil, nil
	rec = mailRequest(t, h, admin, http.MethodPost, "/settings/mail/test", `{"an":" empfaenger@example.test "}`)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"gesendet\":true,\"umgeleitet\":false}\n" {
		t.Fatalf("Test: %d %q", rec.Code, rec.Body.String())
	}
	if len(transport.msgs) != 1 || strings.Join(transport.msgs[0].To, ",") != "empfaenger@example.test" || transport.msgs[0].Subject != mailTestSubject ||
		transport.cfgs[0].Passwort != mailTestPassword || transport.cfgs[0].Port != port || transport.cfgs[0].Host != "127.0.0.1" {
		t.Fatalf("Versand: %+v %v", transport.msgs, transport.cfgs)
	}

	// Mail-Server nicht erreichbar: 502 mit zusammengefasster Meldung, ohne Passwort.
	transport.fail = true
	rec = mailRequest(t, h, admin, http.MethodPost, "/settings/mail/test", `{"an":"empfaenger@example.test"}`)
	if rec.Code != http.StatusBadGateway || mailErrorMessage(t, rec) != "Mail-Server nicht erreichbar" || strings.Contains(rec.Body.String(), mailTestPassword) {
		t.Fatalf("nicht erreichbar: %d %s", rec.Code, rec.Body.String())
	}
	transport.fail = false

	// Anderer Schluessel: Passwort gesetzt, aber nicht lesbar -> Testversand 409.
	rotated := cfg
	rotated.SettingsEncryptionKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{4}, 32))
	h2 := NewRouterWithDepsAndOptions(env.PG, env.Mongo, env.Redis, &rotated, V1RouterOptions{MailTransport: transport})
	admin2 := loginIntegrationUser(t, h2, "mail-admin@example.test", "Secret123!")
	if rec := mailRequest(t, h2, admin2, http.MethodPost, "/settings/mail/test", `{"an":"a@example.test"}`); rec.Code != http.StatusConflict ||
		mailErrorMessage(t, rec) != "Gespeichertes Geheimnis lässt sich nicht entschlüsseln" {
		t.Fatalf("anderer Schluessel: %d %s", rec.Code, rec.Body.String())
	}
	// Ohne Schluessel: kein Passwort speicherbar -> 409 mit Hinweis.
	noKey := cfg
	noKey.SettingsEncryptionKey = ""
	h3 := NewRouterWithDepsAndOptions(env.PG, env.Mongo, env.Redis, &noKey, V1RouterOptions{MailTransport: transport})
	admin3 := loginIntegrationUser(t, h3, "mail-admin@example.test", "Secret123!")
	if rec := mailRequest(t, h3, admin3, http.MethodPut, "/settings/mail/", full); rec.Code != http.StatusConflict ||
		!strings.Contains(mailErrorMessage(t, rec), "SETTINGS_ENCRYPTION_KEY") {
		t.Fatalf("ohne Schluessel: %d %s", rec.Code, rec.Body.String())
	}
	if len(transport.msgs) != 1 {
		t.Fatalf("weitere Versuche verschickt: %d", len(transport.msgs))
	}
}

// Mit MAIL_REDIRECT_TO meldet der Testversand die Umleitung, und der
// Standard-Transport leitet tatsaechlich um.
func TestMailSettingsTestReportsRedirect(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = env.PG.Exec(context.Background(), `DELETE FROM company_mail_settings WHERE company_id='default'`)
	})
	cfg := *env.Cfg
	cfg.SettingsEncryptionKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32))
	cfg.MailRedirectTo = "umleitung@example.test"
	transport := &recordingTransport{}
	h := NewRouterWithDepsAndOptions(env.PG, env.Mongo, env.Redis, &cfg, V1RouterOptions{MailTransport: transport})
	testutil.SeedAuthUser(t, env, "mail-admin@example.test", "Secret123!", "admin")
	admin := loginIntegrationUser(t, h, "mail-admin@example.test", "Secret123!")
	if _, err := env.PG.Exec(ctx, `DELETE FROM company_mail_settings WHERE company_id='default'`); err != nil {
		t.Fatal(err)
	}
	full := `{"smtp_host":"127.0.0.1","smtp_port":` + strconv.Itoa(closedPort(t)) + `,"benutzer":"erp@example.test","passwort":"` + mailTestPassword + `","absender_adresse":"erp@example.test"}`
	if rec := mailRequest(t, h, admin, http.MethodPut, "/settings/mail/", full); rec.Code != http.StatusOK {
		t.Fatalf("speichern: %d %s", rec.Code, rec.Body.String())
	}
	// Inaktiv gespeichert: Testversand trotzdem moeglich.
	if rec := mailRequest(t, h, admin, http.MethodPost, "/settings/mail/test", `{"an":"a@example.test"}`); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"umgeleitet":true`) {
		t.Fatalf("Test: %d %s", rec.Code, rec.Body.String())
	}
	if len(transport.msgs) != 1 || transport.cfgs[0].Aktiv {
		t.Fatalf("Versand inaktiv: %+v", transport.cfgs)
	}
	// Ohne eingesetzten Transport verschickt der Router mit mail.Sender
	// (hier gegen den geschlossenen Port).
	h0 := NewRouterWithDepsAndOptions(env.PG, env.Mongo, env.Redis, &cfg, V1RouterOptions{})
	admin0 := loginIntegrationUser(t, h0, "mail-admin@example.test", "Secret123!")
	if rec := mailRequest(t, h0, admin0, http.MethodPost, "/settings/mail/test", `{"an":"a@example.test"}`); rec.Code != http.StatusBadGateway ||
		mailErrorMessage(t, rec) != "Mail-Server nicht erreichbar" {
		t.Fatalf("Standard-Transport: %d %s", rec.Code, rec.Body.String())
	}
}
