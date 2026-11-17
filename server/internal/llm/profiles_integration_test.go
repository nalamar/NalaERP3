package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

func TestProfilesServiceFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := llmTestCompany(t, env)
	svc := NewSettingsService(env.PG, testBox(t, 1))
	f := func(v float64) *float64 { return &v }

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, okResponse)
	}))
	defer srv.Close()

	if list, err := svc.ListProfiles(ctx, company); err != nil || len(list) != 0 {
		t.Fatalf("leer: %+v %v", list, err)
	}
	if m, err := svc.GetMandant(ctx, company); err != nil || m.Aktiv || m.Tageslimit != 500 || m.AufrufeHeute != 0 || !m.SchluesselEingerichtet {
		t.Fatalf("Mandant Standard: %+v %v", m, err)
	}
	// Hauptschalter an ohne eingeschaltetes Profil
	if _, err := svc.SaveMandant(ctx, MandantInput{Aktiv: true}, "", company); err == nil || !strings.Contains(err.Error(), "fehlt ein eingeschaltetes Profil") {
		t.Fatalf("an ohne Profil: %v", err)
	}
	// erstes Profil wird Standard, Schwelle 0,9
	lokal, err := svc.CreateProfile(ctx, ProfileInput{Name: " Lokal ", Aktiv: true, Endpunkt: srv.URL, Modell: "m1", TimeoutS: 7}, "", company)
	if err != nil || !lokal.Standard || lokal.Name != "Lokal" || lokal.Schwelle != 0.9 || lokal.APIKeyGesetzt || lokal.UpdatedAt == nil {
		t.Fatalf("erstes Profil: %+v %v", lokal, err)
	}
	if _, err := svc.CreateProfile(ctx, ProfileInput{Name: "Claude", Aktiv: true, Anbieter: "anthropic"}, "", company); err == nil ||
		!strings.Contains(err.Error(), "fehlt: API-Schlüssel") {
		t.Fatalf("ohne Schlüssel: %v", err)
	}
	claude, err := svc.CreateProfile(ctx, ProfileInput{Name: "Claude", Aktiv: true, Anbieter: "anthropic", APIKey: "sk-geheim", Schwelle: f(0.75)}, "", company)
	if err != nil || claude.Standard || claude.Schwelle != 0.75 || !claude.APIKeyGesetzt || !claude.APIKeyLesbar {
		t.Fatalf("zweites Profil: %+v %v", claude, err)
	}
	if raw, _ := json.Marshal(claude); strings.Contains(string(raw), "sk-geheim") {
		t.Fatalf("Schlüssel im JSON: %s", raw)
	}
	if _, err := svc.CreateProfile(ctx, ProfileInput{Name: "LOKAL"}, "", company); err == nil || !strings.Contains(err.Error(), "bereits vorhanden") {
		t.Fatalf("Name doppelt: %v", err)
	}
	if _, err := svc.UpdateProfile(ctx, claude.ID, ProfileInput{Name: "lokal", Aktiv: true, Anbieter: "anthropic"}, "", company); err == nil ||
		!strings.Contains(err.Error(), "bereits vorhanden") {
		t.Fatalf("Umbenennen auf vorhandenen Namen: %v", err)
	}
	// Liste: Standard zuerst, obwohl "Claude" vor "Lokal" sortiert
	list, err := svc.ListProfiles(ctx, company)
	if err != nil || len(list) != 2 || list[0].ID != lokal.ID || list[1].ID != claude.ID || !list[1].APIKeyLesbar {
		t.Fatalf("Liste: %+v %v", list, err)
	}

	// Hauptschalter und Tageslimit getrennt
	if _, err := svc.SaveMandant(ctx, MandantInput{Tageslimit: 100001}, "", company); err == nil || !strings.Contains(err.Error(), "Tageslimit ungültig") {
		t.Fatalf("Tageslimit: %v", err)
	}
	if m, err := svc.SaveMandant(ctx, MandantInput{Aktiv: true, Tageslimit: 3}, "", company); err != nil || !m.Aktiv || m.Tageslimit != 3 {
		t.Fatalf("Mandant an: %+v %v", m, err)
	}

	// Client je Profil mit Abschrift; "" = Standardprofil
	c, ab, err := svc.ProfileClient(ctx, company, "", nil)
	if err != nil || ab.ProfilID != lokal.ID || ab.Name != "Lokal" || ab.Modell != "m1" || ab.Schwelle != 0.9 ||
		c.Limits() != (Limits{Timeout: 7 * time.Second, MaxRetries: 2}) {
		t.Fatalf("Standardprofil: %+v %v", ab, err)
	}
	if _, err := c.Complete(ctx, Request{}); err != nil || atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("Aufruf: %v %d", err, calls)
	}
	if m, _ := svc.GetMandant(ctx, company); m.AufrufeHeute != 1 {
		t.Fatalf("Aufrufe: %+v", m)
	}
	_, ab, err = svc.ProfileClient(ctx, company, claude.ID, nil)
	if err != nil || *ab != (ProfilAbschrift{ProfilID: claude.ID, Name: "Claude", Anbieter: "anthropic", Schwelle: 0.75}) {
		t.Fatalf("Claude: %+v %v", ab, err)
	}

	// Standard wechseln: genau ein Standard, bisherige Einstellung folgt
	got, err := svc.SetStandardProfile(ctx, claude.ID, "", company)
	if err != nil || !got.Standard {
		t.Fatalf("Standard setzen: %+v %v", got, err)
	}
	list, _ = svc.ListProfiles(ctx, company)
	if list[0].ID != claude.ID || !list[0].Standard || list[1].Standard {
		t.Fatalf("nach Wechsel: %+v", list)
	}
	if set, _ := svc.Get(ctx, company); set.Anbieter != "anthropic" || !set.APIKeyLesbar {
		t.Fatalf("Einstellung nach Wechsel: %+v", set)
	}
	if got, err := svc.SetStandardProfile(ctx, claude.ID, "", company); err != nil || !got.Standard {
		t.Fatalf("Standard erneut: %+v %v", got, err)
	}

	// Ausschalten: erlaubt, solange ein weiteres eingeschaltet ist
	if _, err := svc.UpdateProfile(ctx, lokal.ID, ProfileInput{Name: "Lokal", Endpunkt: srv.URL, Modell: "m1", TimeoutS: 7}, "", company); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.ProfileClient(ctx, company, lokal.ID, nil); err == nil || !strings.Contains(err.Error(), "nicht eingeschaltet") {
		t.Fatalf("Profil aus: %v", err)
	}
	if _, ab, err := svc.TestProfileClient(ctx, company, lokal.ID, nil); err != nil || ab.Name != "Lokal" {
		t.Fatalf("Testclient: %+v %v", ab, err)
	}
	if _, err := svc.UpdateProfile(ctx, claude.ID, ProfileInput{Name: "Claude", Anbieter: "anthropic"}, "", company); err == nil ||
		!strings.Contains(err.Error(), "letzte eingeschaltete Profil darf nicht") {
		t.Fatalf("letztes Profil: %v", err)
	}
	// Ändern ohne Schlüssel behält ihn
	got, err = svc.UpdateProfile(ctx, claude.ID, ProfileInput{Name: "Claude Opus", Aktiv: true, Anbieter: "anthropic", Schwelle: f(0.8)}, "", company)
	if err != nil || got.Name != "Claude Opus" || !got.APIKeyLesbar || got.Schwelle != 0.8 || !got.Standard {
		t.Fatalf("Ändern: %+v %v", got, err)
	}
	// Hauptschalter aus: kein Client, letztes Profil darf aus
	if _, err := svc.SaveMandant(ctx, MandantInput{Tageslimit: 3}, "", company); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.ProfileClient(ctx, company, claude.ID, nil); err == nil || !strings.Contains(err.Error(), "nicht eingeschaltet") {
		t.Fatalf("Hauptschalter aus: %v", err)
	}
	if _, err := svc.UpdateProfile(ctx, claude.ID, ProfileInput{Name: "Claude Opus", Anbieter: "anthropic", APIKeyLoeschen: true}, "", company); err != nil {
		t.Fatalf("letztes aus bei Hauptschalter aus: %v", err)
	}
	if got, _ := svc.GetProfile(ctx, company, claude.ID); got.APIKeyGesetzt {
		t.Fatalf("Schlüssel gelöscht: %+v", got)
	}

	// fremder Mandant, unbekannte und ungueltige ID
	other := llmTestCompany(t, env)
	for _, id := range []string{claude.ID, uuid.NewString(), "kein-uuid", ""} {
		if _, err := svc.UpdateProfile(ctx, id, ProfileInput{Name: "x"}, "", other); !errors.Is(err, ErrProfileNotFound) {
			t.Fatalf("Ändern %q: %v", id, err)
		}
		if _, err := svc.SetStandardProfile(ctx, id, "", other); !errors.Is(err, ErrProfileNotFound) {
			t.Fatalf("Standard %q: %v", id, err)
		}
		if _, _, err := svc.TestProfileClient(ctx, other, id, nil); !errors.Is(err, ErrProfileNotFound) {
			t.Fatalf("Client %q: %v", id, err)
		}
	}
	if _, err := svc.GetProfile(ctx, other, claude.ID); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("Lesen fremd: %v", err)
	}
	// gleicher Name beim anderen Mandanten erlaubt, dort erstes = Standard
	if p, err := svc.CreateProfile(ctx, ProfileInput{Name: "Lokal"}, "", other); err != nil || !p.Standard {
		t.Fatalf("anderer Mandant: %+v %v", p, err)
	}
	if list, _ := svc.ListProfiles(ctx, company); len(list) != 2 {
		t.Fatalf("eigene Liste unverändert: %+v", list)
	}
	if _, err := svc.ListProfiles(ctx, " "); err == nil {
		t.Fatal("ohne Mandant")
	}
}
