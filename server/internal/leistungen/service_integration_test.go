package leistungen

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

func testMandant(t *testing.T, env *testutil.IntegrationEnv) string {
	t.Helper()
	id := "lst-" + uuid.NewString()[:8]
	if _, err := env.PG.Exec(context.Background(), `INSERT INTO company_profiles (id, name) VALUES ($1, $2)`, id, "Leistungen-Test "+id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestLeistungenAnlegenAendernListe(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	a, b := testMandant(t, env), testMandant(t, env)
	u := 0.9

	fe, err := svc.Anlegen(ctx, Eingabe{Code: " FE-1 ", Kurztext: "Aluminiumfenster einflügelig", Einheit: "Stk", Gewerk: "fenster",
		Profilserie: "Serie A75", Verglasung: "3-fach", RCKlasse: "rc2", UWert: &u, Brandschutzklasse: "T30"}, "", a)
	if err != nil || fe.Code != "FE-1" || !fe.Aktiv || fe.RCKlasse != "RC2" || fe.UWert == nil || *fe.UWert != 0.9 || fe.UpdatedByUserID != nil ||
		fe.ID == "" || fe.Profilserie != "Serie A75" || fe.Verglasung != "3-fach" || fe.Brandschutzklasse != "T30" || fe.Gewerk != "fenster" {
		t.Fatalf("Anlegen: %+v %v", fe, err)
	}
	aus := false
	if l, err := svc.Anlegen(ctx, Eingabe{Code: "DEM", Kurztext: "Altfenster ausbauen", Einheit: "Stk", Aktiv: &aus}, "", a); err != nil || l.Aktiv || l.UWert != nil {
		t.Fatalf("abgeschaltet anlegen: %+v %v", l, err)
	}
	// Code je Mandant eindeutig; im anderen Mandanten frei
	if _, err := svc.Anlegen(ctx, Eingabe{Code: "FE-1", Kurztext: "x", Einheit: "Stk"}, "", a); err == nil || err.Error() != `Code "FE-1" bereits vorhanden` {
		t.Fatalf("doppelt: %v", err)
	}
	if _, err := svc.Anlegen(ctx, Eingabe{Code: "FE-1", Kurztext: "anderer Mandant", Einheit: "m"}, "", b); err != nil {
		t.Fatalf("anderer Mandant: %v", err)
	}
	if _, err := svc.Anlegen(ctx, Eingabe{Code: "X", Kurztext: "", Einheit: "Stk"}, "", a); err == nil || err.Error() != "Kurztext erforderlich" {
		t.Fatalf("ungültig: %v", err)
	}
	if _, err := svc.Anlegen(ctx, Eingabe{Code: "X", Kurztext: "x", Einheit: "Stk"}, "", " "); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("ohne Mandant: %v", err)
	}

	// Ändern: ganzer Stand, Aktiv nil bleibt; fremder Mandant: nicht gefunden
	l, err := svc.Aendern(ctx, fe.ID, Eingabe{Code: "FE-1", Kurztext: "Fenster neu", Einheit: "St"}, "", a)
	if err != nil || l.Kurztext != "Fenster neu" || !l.Aktiv || l.RCKlasse != "" || l.UWert != nil || l.Einheit != "St" || l.UpdatedAt.Before(fe.UpdatedAt) {
		t.Fatalf("Ändern: %+v %v", l, err)
	}
	if l, err = svc.Aendern(ctx, fe.ID, Eingabe{Code: "FE-1", Kurztext: "Fenster neu", Einheit: "St", Aktiv: &aus}, "", a); err != nil || l.Aktiv {
		t.Fatalf("abschalten: %+v %v", l, err)
	}
	if l, err = svc.Aendern(ctx, fe.ID, Eingabe{Code: "FE-1", Kurztext: "Fenster neu", Einheit: "St"}, "", a); err != nil || l.Aktiv {
		t.Fatalf("aus bleibt aus: %+v %v", l, err)
	}
	if _, err := svc.Aendern(ctx, fe.ID, Eingabe{Code: "DEM", Kurztext: "x", Einheit: "Stk"}, "", a); err == nil || err.Error() != `Code "DEM" bereits vorhanden` {
		t.Fatalf("Code vergeben: %v", err)
	}
	if _, err := svc.Aendern(ctx, fe.ID, Eingabe{Code: "FE-1", Kurztext: "fremd", Einheit: "Stk"}, "", b); err == nil || err.Error() != "Leistung nicht gefunden" {
		t.Fatalf("fremder Mandant ändert: %v", err)
	}
	if _, err := svc.Aendern(ctx, fe.ID, Eingabe{Code: "FE-1", Kurztext: "", Einheit: "Stk"}, "", a); err == nil {
		t.Fatal("ungültig geändert")
	}
	if _, err := svc.Aendern(ctx, fe.ID, Eingabe{Code: "FE-1", Kurztext: "x", Einheit: "Stk"}, "", ""); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("ohne Mandant ändern: %v", err)
	}
	if _, err := svc.Lesen(ctx, fe.ID, b); err == nil || err.Error() != "Leistung nicht gefunden" {
		t.Fatalf("fremder Mandant liest: %v", err)
	}
	if got, err := svc.Lesen(ctx, fe.ID, a); err != nil || got.Kurztext != "Fenster neu" {
		t.Fatalf("Lesen: %+v %v", got, err)
	}
	if _, err := svc.Lesen(ctx, fe.ID, ""); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("ohne Mandant lesen: %v", err)
	}

	// Liste: sortiert, Suche, nur aktive, Limit/Offset, Mandant getrennt
	if _, err := svc.Anlegen(ctx, Eingabe{Code: "AA", Kurztext: "Attikablech", Einheit: "m"}, "", a); err != nil {
		t.Fatal(err)
	}
	codes := func(f Filter, m string) string {
		ls, err := svc.Liste(ctx, f, m)
		if err != nil {
			t.Fatal(err)
		}
		var cs []string
		for _, l := range ls {
			cs = append(cs, l.Code)
		}
		return strings.Join(cs, ",")
	}
	for _, c := range []struct {
		f    Filter
		want string
	}{
		{Filter{}, "AA,DEM,FE-1"},
		{Filter{NurAktive: true}, "AA"},
		{Filter{Q: "fenster"}, "DEM,FE-1"},
		{Filter{Q: " fe- "}, "FE-1"},
		{Filter{Limit: 2}, "AA,DEM"},
		{Filter{Limit: 2, Offset: 2}, "FE-1"},
		{Filter{Offset: -5, Limit: -1}, "AA,DEM,FE-1"},
		{Filter{Limit: 5000}, "AA,DEM,FE-1"},
	} {
		if got := codes(c.f, a); got != c.want {
			t.Errorf("Liste %+v: %q, erwartet %q", c.f, got, c.want)
		}
	}
	if got := codes(Filter{}, b); got != "FE-1" {
		t.Errorf("Mandant b: %q", got)
	}
	if _, err := svc.Liste(ctx, Filter{}, ""); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("ohne Mandant Liste: %v", err)
	}
	// höchstens 1000 je Seite, Standard 200
	viele := testMandant(t, env)
	if _, err := env.PG.Exec(ctx, `INSERT INTO leistungen (id, company_id, code, kurztext, einheit)
        SELECT gen_random_uuid()::text, $1, 'L' || lpad(i::text, 4, '0'), 'Leistung', 'Stk' FROM generate_series(1, 1001) i`, viele); err != nil {
		t.Fatal(err)
	}
	for limit, want := range map[int]int{5000: 1000, 0: 200} {
		if ls, err := svc.Liste(ctx, Filter{Limit: limit}, viele); err != nil || len(ls) != want {
			t.Fatalf("Limit %d: %d %v", limit, len(ls), err)
		}
	}
}

func TestLeistungenImport(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	a := testMandant(t, env)
	aus := false
	if _, err := svc.Anlegen(ctx, Eingabe{Code: "ALT", Kurztext: "alt", Einheit: "Stk", Aktiv: &aus}, "", a); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Anlegen(ctx, Eingabe{Code: "BLEIBT", Kurztext: "nicht in der Datei", Einheit: "Stk"}, "", a); err != nil {
		t.Fatal(err)
	}
	erg, err := svc.Importieren(ctx, []Eingabe{
		{Code: "ALT", Kurztext: "alt geändert", Einheit: "m", RCKlasse: "RC 3"},
		{Code: " NEU ", Kurztext: "neu", Einheit: "Stk"},
		{Code: "NEU-AUS", Kurztext: "neu aus", Einheit: "Stk", Aktiv: &aus},
	}, nil, "", a)
	if err != nil || erg.Neu != 2 || erg.Geaendert != 1 {
		t.Fatalf("Import: %+v %v", erg, err)
	}
	ls, _ := svc.Liste(ctx, Filter{}, a)
	byCode := map[string]Leistung{}
	for _, l := range ls {
		byCode[l.Code] = l
	}
	if alt := byCode["ALT"]; alt.Kurztext != "alt geändert" || alt.Einheit != "m" || alt.RCKlasse != "RC3" || alt.Aktiv {
		t.Fatalf("ALT (aktiv bleibt aus): %+v", alt)
	}
	if !byCode["NEU"].Aktiv || byCode["NEU-AUS"].Aktiv || byCode["BLEIBT"].Kurztext != "nicht in der Datei" || len(ls) != 4 {
		t.Fatalf("Import-Stand: %+v", ls)
	}
	an := true
	if erg, err := svc.Importieren(ctx, []Eingabe{{Code: "ALT", Kurztext: "alt", Einheit: "m", Aktiv: &an}}, nil, "", a); err != nil || erg.Geaendert != 1 || erg.Neu != 0 {
		t.Fatalf("einschalten: %+v %v", erg, err)
	}
	if !mustCode(t, svc, a, "ALT").Aktiv {
		t.Fatal("ALT nicht eingeschaltet")
	}
	// ganz oder gar nicht: Fehler in Zeile 2 -> Zeile 1 nicht geschrieben
	for _, c := range []struct {
		zeilen []Eingabe
		want   string
	}{
		{[]Eingabe{{Code: "NEU", Kurztext: "nicht gespeichert", Einheit: "Stk"}, {Code: "X", Kurztext: "", Einheit: "Stk"}}, "Zeile 2: Kurztext erforderlich"},
		{[]Eingabe{{Code: "NEU", Kurztext: "nicht gespeichert", Einheit: "Stk"}, {Code: " NEU", Kurztext: "y", Einheit: "Stk"}}, `Zeile 2: Code "NEU" ungültig - doppelt (schon in Zeile 1)`},
		{nil, "Import ungültig: keine Leistungen"},
		{make([]Eingabe, MaxImportZeilen+1), "Import ungültig: höchstens 5000 Leistungen je Datei"},
	} {
		if _, err := svc.Importieren(ctx, c.zeilen, nil, "", a); err == nil || err.Error() != c.want {
			t.Errorf("%q: %v", c.want, err)
		}
	}
	if l := mustCode(t, svc, a, "NEU"); l.Kurztext != "neu" {
		t.Fatalf("teilweise geschrieben: %+v", l)
	}
	// Zeilennummern der Datei (z. B. aus LeseCSV) in den Meldungen
	doppelt := []Eingabe{{Code: "D", Kurztext: "d", Einheit: "Stk"}, {Code: "D", Kurztext: "d", Einheit: "Stk"}}
	if _, err := svc.Importieren(ctx, doppelt, []int{5, 9}, "", a); err == nil || err.Error() != `Zeile 9: Code "D" ungültig - doppelt (schon in Zeile 5)` {
		t.Fatalf("Dateizeilen doppelt: %v", err)
	}
	if _, err := svc.Importieren(ctx, []Eingabe{{Code: "E", Kurztext: "", Einheit: "Stk"}}, []int{7}, "", a); err == nil || err.Error() != "Zeile 7: Kurztext erforderlich" {
		t.Fatalf("Dateizeile Fehler: %v", err)
	}
	if _, err := svc.Importieren(ctx, doppelt, []int{5}, "", a); err == nil || err.Error() != "Import ungültig: Zeilennummern passen nicht zu den Leistungen" {
		t.Fatalf("Nummern passen nicht: %v", err)
	}
	if _, err := svc.Importieren(ctx, []Eingabe{{Code: "X", Kurztext: "x", Einheit: "Stk"}}, nil, "", ""); err == nil || err.Error() != "Mandant erforderlich" {
		t.Fatalf("ohne Mandant: %v", err)
	}
	// Prüfungen der Datenbank
	for name, sql := range map[string]string{
		"Code Leerzeichen": `UPDATE leistungen SET code='A B' WHERE company_id=$1 AND code='NEU'`,
		"Code leer":        `UPDATE leistungen SET code='' WHERE company_id=$1 AND code='NEU'`,
		"Code lang":        `UPDATE leistungen SET code=repeat('c', 51) WHERE company_id=$1 AND code='NEU'`,
		"Kurztext leer":    `UPDATE leistungen SET kurztext=' ' WHERE company_id=$1 AND code='NEU'`,
		"Einheit leer":     `UPDATE leistungen SET einheit='' WHERE company_id=$1 AND code='NEU'`,
		"RC":               `UPDATE leistungen SET rc_klasse='RC7' WHERE company_id=$1 AND code='NEU'`,
		"U-Wert":           `UPDATE leistungen SET u_wert=0 WHERE company_id=$1 AND code='NEU'`,
		"U-Wert groß":      `UPDATE leistungen SET u_wert=10.5 WHERE company_id=$1 AND code='NEU'`,
		"Merkmal":          `UPDATE leistungen SET profilserie=repeat('p', 201) WHERE company_id=$1 AND code='NEU'`,
		"doppelt":          `UPDATE leistungen SET code='ALT' WHERE company_id=$1 AND code='NEU'`,
	} {
		if _, err := env.PG.Exec(ctx, sql, a); err == nil {
			t.Errorf("Datenbank nimmt %s an", name)
		}
	}
}

func mustCode(t *testing.T, svc *Service, companyID, code string) Leistung {
	t.Helper()
	ls, err := svc.Liste(context.Background(), Filter{Q: code}, companyID)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range ls {
		if l.Code == code {
			return l
		}
	}
	t.Fatalf("%s fehlt", code)
	return Leistung{}
}
