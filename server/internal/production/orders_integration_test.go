package production

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Service der Fertigungsauftraege gegen die Datenbank (Backlog H.1.2).

// seedMaterials haengt an die Variante ein Profil, einen Artikel und ein
// Glas, wie sie der Logikal-Import anlegt (REAL, NULL-Texte).
func seedMaterials(t *testing.T, env *testutil.IntegrationEnv, variant uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO single_elevation_profiles (id, single_elevation_id, supplier_code, article_code, description, length_mm, qty, unit, price_net)
			VALUES ($1, $2, 'S1', 'P-100', 'Blendrahmen', 1230.4, 2, 'Stk', 99)`, []any{uuid.New(), variant}},
		{`INSERT INTO single_elevation_articles (id, single_elevation_id, article_code, description, qty, unit)
			VALUES ($1, $2, 'A-7', NULL, 1.5, NULL)`, []any{uuid.New(), variant}},
		{`INSERT INTO single_elevation_glass (id, single_elevation_id, configuration, description, width_mm, height_mm, area_m2, qty)
			VALUES ($1, $2, '4/16/4', 'Isolierglas', 1100, 1350, 1.485, 1)`, []any{uuid.New(), variant}},
		{`UPDATE project_elevations SET width_mm=1230, height_mm=1480, external_guid='g-pos'
			WHERE id=(SELECT elevation_id FROM project_single_elevations WHERE id=$1)`, []any{variant}},
	} {
		if _, err := env.PG.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("Material: %v", err)
		}
	}
}

func seedSalesOrder(t *testing.T, env *testutil.IntegrationEnv, p seededPosition, status string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	contact, quote, order := "ct-"+uuid.NewString()[:12], uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO contacts (id, typ, rolle, status, name, email, phone, waehrung)
			VALUES ($1, 'org', 'customer', 'active', 'Testkunde', 'kunde@example.com', '', 'EUR')`, []any{contact}},
		{`INSERT INTO quotes (id, nummer, root_quote_id, revision_no, contact_id, status, quote_date, currency, net_amount, tax_amount, gross_amount, company_id)
			VALUES ($1, $2, $1, 1, $3, 'draft', CURRENT_DATE, 'EUR', 0, 0, 0, $4)`, []any{quote, "AN-" + quote.String()[:8], contact, p.company}},
		{`INSERT INTO sales_orders (id, nummer, source_quote_id, project_id, contact_id, status, order_date, company_id)
			VALUES ($1, $2, $3, $4, $5, $6, CURRENT_DATE, $7)`, []any{order, "AB-" + order.String()[:8], quote, p.project, contact, status, p.company}},
	} {
		if _, err := env.PG.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("Kundenauftrag: %v", err)
		}
	}
	return order
}

func hasHint(o *Order, h string) bool {
	for _, x := range o.Hinweise {
		if x == h {
			return true
		}
	}
	return false
}

func wantErr(t *testing.T, name string, err error, want string) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Fatalf("%s: erwartet %q, bekommen %v", name, want, err)
	}
}

func TestOrderCreateAndDraft(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	p := seedPosition(t, env)
	seedMaterials(t, env, p.variant)
	other := seedPosition(t, env)
	in := OrderInput{ProjectID: p.project, ElevationID: p.elevation, SingleElevationID: p.variant, Stueckzahl: 4,
		Ausfuehrungsklasse: strPtr(" exc2 "), Notiz: " erstes Los "}

	// Negativfaelle
	_, err := svc.Create(ctx, in, "", "")
	wantErr(t, "ohne Mandant", err, "Mandant erforderlich")
	_, err = svc.Create(ctx, in, "", other.company)
	wantErr(t, "fremdes Projekt", err, "Projekt nicht gefunden")
	bad := in
	bad.ElevationID = other.elevation
	_, err = svc.Create(ctx, bad, "", p.company)
	wantErr(t, "Position aus anderem Projekt", err, "Position nicht im Projekt")
	bad = in
	bad.SingleElevationID = other.variant
	_, err = svc.Create(ctx, bad, "", p.company)
	wantErr(t, "Variante anderer Position", err, "Variante nicht an der Position")
	bad = in
	bad.Stueckzahl = 0
	_, err = svc.Create(ctx, bad, "", p.company)
	wantErr(t, "Stückzahl 0", err, "Stückzahl muss größer als 0 sein")
	var n int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM production_orders WHERE company_id=$1`, p.company).Scan(&n); err != nil || n != 0 {
		t.Fatalf("nach Fehlern angelegt: %d, %v", n, err)
	}

	o, err := svc.Create(ctx, in, "", p.company)
	if err != nil {
		t.Fatalf("Entwurf: %v", err)
	}
	// Neuer Mandant ohne Nummernkreis: trotzdem "FA-", nicht "PO-".
	if want := "FA-" + time.Now().Format("2006") + "-0001"; o.Nummer != want {
		t.Fatalf("Nummer %q, erwartet %q", o.Nummer, want)
	}
	if o.Status != StatusEntwurf || o.Stueckzahl != 4 || o.Ausfuehrungsklasse == nil || *o.Ausfuehrungsklasse != "EXC2" ||
		o.Notiz != "erstes Los" || o.ProjectNummer != "P-1" || o.Items != nil || o.FreigegebenAt != nil {
		t.Fatalf("Entwurf: %+v", o)
	}
	// Im Entwurf die aktuellen Werte der Position.
	if o.PhaseName != "Haus A" || o.PositionNummer != "3" || o.PositionName != "Fenster" || o.VarianteName != "links" ||
		o.WidthMM == nil || *o.WidthMM != 1230 {
		t.Fatalf("Position im Entwurf: %+v", o)
	}
	if !hasHint(o, HintKeinKundenauftrag) || hasHint(o, HintMengeUeberschritten) || len(o.Hinweise) != 1 {
		t.Fatalf("Hinweise: %v", o.Hinweise)
	}
	var stored string
	if err := env.PG.QueryRow(ctx, `SELECT position_nummer FROM production_orders WHERE id=$1`, o.ID).Scan(&stored); err != nil || stored != "" {
		t.Fatalf("Entwurf darf nichts kopieren: %q, %v", stored, err)
	}

	// Zweites Los: 4 + 7 > 10 -> Hinweis an beiden.
	in2 := in
	in2.Stueckzahl = 7
	o2, err := svc.Create(ctx, in2, "", p.company)
	if err != nil {
		t.Fatalf("zweites Los: %v", err)
	}
	if !strings.HasSuffix(o2.Nummer, "-0002") || !hasHint(o2, HintMengeUeberschritten) {
		t.Fatalf("zweites Los: %s %v", o2.Nummer, o2.Hinweise)
	}
	if g, _ := svc.Get(ctx, o.ID, p.company); !hasHint(g, HintMengeUeberschritten) {
		t.Fatalf("erstes Los: %v", g.Hinweise)
	}

	// UpdateDraft
	if _, err := svc.UpdateDraft(ctx, o2.ID, DraftUpdate{Stueckzahl: 0}, p.company); err == nil {
		t.Fatal("Stückzahl 0 geändert")
	}
	_, err = svc.UpdateDraft(ctx, o2.ID, DraftUpdate{Stueckzahl: 6, Ausfuehrungsklasse: strPtr("EXC9")}, p.company)
	wantErr(t, "Klasse", err, "Ungültige Ausführungsklasse")
	_, err = svc.UpdateDraft(ctx, o2.ID, DraftUpdate{Stueckzahl: 6}, other.company)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("fremder Mandant: %v", err)
	}
	day := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	u, err := svc.UpdateDraft(ctx, o2.ID, DraftUpdate{Stueckzahl: 6, Liefertermin: &day, Ausfuehrungsklasse: strPtr(""), Notiz: " n "}, p.company)
	if err != nil {
		t.Fatalf("ändern: %v", err)
	}
	if u.Stueckzahl != 6 || u.Liefertermin == nil || !u.Liefertermin.Equal(day) || u.Ausfuehrungsklasse != nil || u.Notiz != "n" ||
		hasHint(u, HintMengeUeberschritten) {
		t.Fatalf("geändert: %+v", u)
	}
	// Ein storniertes Los zaehlt nicht zur Menge: 4 + 6 = 10.
	o3, err := svc.Create(ctx, OrderInput{ProjectID: p.project, ElevationID: p.elevation, SingleElevationID: p.variant, Stueckzahl: 5}, "", p.company)
	if err != nil || !hasHint(o3, HintMengeUeberschritten) {
		t.Fatalf("drittes Los: %v %v", o3, err)
	}
	if _, err := svc.Cancel(ctx, o3.ID, "doppelt", "", p.company); err != nil {
		t.Fatal(err)
	}
	if g, _ := svc.Get(ctx, o.ID, p.company); hasHint(g, HintMengeUeberschritten) {
		t.Fatalf("storniertes Los mitgezählt: %v", g.Hinweise)
	}
	// Mit einem Kundenauftrag (nicht storniert) entfaellt der Hinweis.
	seedSalesOrder(t, env, p, "canceled")
	if g, _ := svc.Get(ctx, o.ID, p.company); !hasHint(g, HintKeinKundenauftrag) {
		t.Fatalf("stornierter Kundenauftrag zählt nicht: %v", g.Hinweise)
	}
	seedSalesOrder(t, env, p, "open")
	if g, _ := svc.Get(ctx, o.ID, p.company); hasHint(g, HintKeinKundenauftrag) {
		t.Fatalf("mit Kundenauftrag: %v", g.Hinweise)
	}
	if _, err := svc.Release(ctx, o2.ID, "", "", p.company); err != nil {
		t.Fatalf("freigeben: %v", err)
	}
	_, err = svc.UpdateDraft(ctx, o2.ID, DraftUpdate{Stueckzahl: 1}, p.company)
	wantErr(t, "nach Freigabe", err, "Nur ein Entwurf kann geändert werden")
}

func TestOrderReleaseCopyAndLifecycle(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	p := seedPosition(t, env)
	seedMaterials(t, env, p.variant)
	other := seedPosition(t, env)
	actor := testutil.SeedAuthUser(t, env, "fa-"+uuid.NewString()[:8]+"@example.com", "Passwort-123!", "production")
	o, err := svc.Create(ctx, OrderInput{ProjectID: p.project, ElevationID: p.elevation, SingleElevationID: p.variant, Stueckzahl: 3}, actor, p.company)
	if err != nil {
		t.Fatalf("Entwurf: %v", err)
	}

	// falsche Reihenfolge und fremder Mandant
	_, err = svc.Start(ctx, o.ID, actor, p.company)
	wantErr(t, "beginnen im Entwurf", err, "Aktion im aktuellen Status nicht erlaubt")
	_, err = svc.Finish(ctx, o.ID, actor, p.company)
	wantErr(t, "fertig im Entwurf", err, "Aktion im aktuellen Status nicht erlaubt")
	if _, err := svc.Release(ctx, o.ID, "", actor, other.company); !errors.Is(err, ErrNotFound) {
		t.Fatalf("fremd freigeben: %v", err)
	}
	if _, err := svc.Get(ctx, o.ID, other.company); !errors.Is(err, ErrNotFound) {
		t.Fatalf("fremd lesen: %v", err)
	}
	if _, err := svc.Release(ctx, uuid.New(), "", actor, p.company); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unbekannt: %v", err)
	}

	r, err := svc.Release(ctx, o.ID, "", actor, p.company)
	if err != nil {
		t.Fatalf("freigeben: %v", err)
	}
	if r.Status != StatusFreigegeben || r.FreigegebenAt == nil || r.FreigegebenByUserID == nil || *r.FreigegebenByUserID != actor {
		t.Fatalf("freigegeben: %+v", r)
	}
	if len(r.Items) != 3 || hasHint(r, HintPositionGeaendert) {
		t.Fatalf("Stückliste %d, Hinweise %v", len(r.Items), r.Hinweise)
	}
	byArt := map[string]Item{}
	for i, it := range r.Items {
		// Reihenfolge des Fingerabdrucks (kanonisch: nach Inhalt sortiert)
		if want := []string{ArtArtikel, ArtGlas, ArtProfil}[i]; it.Pos != i+1 || it.Art != want {
			t.Fatalf("Pos %d %s an Stelle %d, erwartet %s", it.Pos, it.Art, i, want)
		}
		byArt[it.Art] = it
	}
	prof, art, gl := byArt[ArtProfil], byArt[ArtArtikel], byArt[ArtGlas]
	if prof.ArticleCode != "P-100" || prof.SupplierCode != "S1" || prof.Description != "Blendrahmen" || prof.LengthMM == nil ||
		*prof.LengthMM != 1230.4 || prof.MengeJeStueck != 2 || prof.MengeGesamt != 6 || prof.Unit != "Stk" || prof.WidthMM != nil {
		t.Fatalf("Profil: %+v", prof)
	}
	if art.ArticleCode != "A-7" || art.Description != "" || art.Unit != "" || art.MengeJeStueck != 1.5 || art.MengeGesamt != 4.5 || art.LengthMM != nil {
		t.Fatalf("Artikel: %+v", art)
	}
	if gl.Configuration != "4/16/4" || gl.Description != "Isolierglas" || gl.WidthMM == nil || *gl.WidthMM != 1100 ||
		gl.HeightMM == nil || *gl.HeightMM != 1350 || gl.AreaM2 == nil || math.Abs(*gl.AreaM2-1.485) > 1e-9 || gl.MengeGesamt != 3 {
		t.Fatalf("Glas: %+v", gl)
	}
	var hash, guid string
	if err := env.PG.QueryRow(ctx, `SELECT inhalt_hash, position_guid FROM production_orders WHERE id=$1`, o.ID).Scan(&hash, &guid); err != nil ||
		len(hash) != 64 || guid != "g-pos" {
		t.Fatalf("Kopie: %q %q %v", hash, guid, err)
	}
	_, err = svc.Release(ctx, o.ID, "", actor, p.company)
	wantErr(t, "zweimal freigeben", err, "Aktion im aktuellen Status nicht erlaubt")
	var items int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM production_order_items WHERE order_id=$1`, o.ID).Scan(&items); err != nil || items != 3 {
		t.Fatalf("Stückliste nach zweitem Versuch: %d, %v", items, err)
	}

	// Logikal aendert die Position: Hinweis, die Kopie bleibt.
	if _, err := env.PG.Exec(ctx, `UPDATE single_elevation_profiles SET length_mm=1240 WHERE single_elevation_id=$1`, p.variant); err != nil {
		t.Fatal(err)
	}
	g, _ := svc.Get(ctx, o.ID, p.company)
	if !hasHint(g, HintPositionGeaendert) || g.Items[0].Art != r.Items[0].Art || *byArtOf(g, ArtProfil).LengthMM != 1230.4 {
		t.Fatalf("nach Änderung: %v %+v", g.Hinweise, g.Items)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE single_elevation_profiles SET length_mm=1230.4 WHERE single_elevation_id=$1`, p.variant); err != nil {
		t.Fatal(err)
	}
	if g, _ := svc.Get(ctx, o.ID, p.company); hasHint(g, HintPositionGeaendert) {
		t.Fatalf("zurückgeändert: %v", g.Hinweise)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE project_elevations SET name='Tür' WHERE id=$1`, p.elevation); err != nil {
		t.Fatal(err)
	}
	if g, _ := svc.Get(ctx, o.ID, p.company); !hasHint(g, HintPositionGeaendert) || g.PositionName != "Fenster" {
		t.Fatalf("Name geändert: %v %q", g.Hinweise, g.PositionName)
	}

	// Ablauf bis fertig
	_, err = svc.Finish(ctx, o.ID, actor, p.company)
	wantErr(t, "fertig vor Beginn", err, "Aktion im aktuellen Status nicht erlaubt")
	s, err := svc.Start(ctx, o.ID, actor, p.company)
	if err != nil || s.Status != StatusInArbeit || s.BegonnenAt == nil || s.BegonnenByUserID == nil || *s.BegonnenByUserID != actor {
		t.Fatalf("beginnen: %+v %v", s, err)
	}
	f, err := svc.Finish(ctx, o.ID, actor, p.company)
	if err != nil || f.Status != StatusFertig || f.FertigAt == nil || f.FertigByUserID == nil || f.FertigAt.Before(*f.BegonnenAt) {
		t.Fatalf("fertig: %+v %v", f, err)
	}
	_, err = svc.Cancel(ctx, o.ID, "zu spät", actor, p.company)
	wantErr(t, "fertig stornieren", err, "Ein fertiger Auftrag kann nicht storniert werden")

	// Storno
	c, err := svc.Create(ctx, OrderInput{ProjectID: p.project, ElevationID: p.elevation, SingleElevationID: p.variant, Stueckzahl: 1}, actor, p.company)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Release(ctx, c.ID, "", actor, p.company); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Start(ctx, c.ID, actor, p.company); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Cancel(ctx, c.ID, "  ", actor, p.company)
	wantErr(t, "ohne Grund", err, "Begründung für das Storno erforderlich")
	if _, err := svc.Cancel(ctx, c.ID, "x", actor, other.company); !errors.Is(err, ErrNotFound) {
		t.Fatalf("fremd stornieren: %v", err)
	}
	x, err := svc.Cancel(ctx, c.ID, " Maß falsch ", actor, p.company)
	if err != nil || x.Status != StatusStorniert || x.StornoReason != "Maß falsch" || x.StorniertAt == nil || x.StorniertByUserID == nil ||
		len(x.Hinweise) != 0 || len(x.Items) != 3 {
		t.Fatalf("storniert: %+v %v", x, err)
	}
	_, err = svc.Cancel(ctx, c.ID, "nochmal", actor, p.company)
	wantErr(t, "zweimal stornieren", err, "Auftrag ist bereits storniert")
	_, err = svc.Start(ctx, c.ID, actor, p.company)
	wantErr(t, "nach Storno", err, "Aktion im aktuellen Status nicht erlaubt")

	// Re-Import entfernt die Variante: Hinweis am freigegebenen Auftrag,
	// ein Entwurf laesst sich nicht mehr freigeben.
	d, err := svc.Create(ctx, OrderInput{ProjectID: p.project, ElevationID: p.elevation, SingleElevationID: p.variant, Stueckzahl: 1}, actor, p.company)
	if err != nil {
		t.Fatal(err)
	}
	// Haengt die Variante an einer anderen Position, gilt sie hier als
	// entfernt.
	moved := uuid.New()
	if _, err := env.PG.Exec(ctx, `INSERT INTO project_elevations (id, phase_id, nummer, name)
		SELECT $1, phase_id, '9', 'Neu' FROM project_elevations WHERE id=$2`, moved, p.elevation); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `UPDATE project_single_elevations SET elevation_id=$1 WHERE id=$2`, moved, p.variant); err != nil {
		t.Fatal(err)
	}
	if g, _ := svc.Get(ctx, o.ID, p.company); !hasHint(g, HintPositionEntfernt) || hasHint(g, HintPositionGeaendert) {
		t.Fatalf("Variante verschoben: %v", g.Hinweise)
	}
	_, err = svc.Release(ctx, d.ID, "", actor, p.company)
	wantErr(t, "Variante verschoben", err, "Position oder Variante ist nicht mehr im Projekt")
	if _, err := env.PG.Exec(ctx, `UPDATE project_single_elevations SET elevation_id=$1 WHERE id=$2`, p.elevation, p.variant); err != nil {
		t.Fatal(err)
	}
	if _, err := env.PG.Exec(ctx, `DELETE FROM project_single_elevations WHERE id=$1`, p.variant); err != nil {
		t.Fatal(err)
	}
	if g, _ := svc.Get(ctx, o.ID, p.company); !hasHint(g, HintPositionEntfernt) || len(g.Items) != 3 || g.PositionNummer != "3" {
		t.Fatalf("nach Re-Import: %v %d", g.Hinweise, len(g.Items))
	}
	_, err = svc.Release(ctx, d.ID, "", actor, p.company)
	wantErr(t, "Variante entfernt", err, "Position oder Variante ist nicht mehr im Projekt")
	if g, _ := svc.Get(ctx, d.ID, p.company); g.Status != StatusEntwurf || !hasHint(g, HintPositionEntfernt) {
		t.Fatalf("Entwurf nach Re-Import: %+v", g)
	}
}

func byArtOf(o *Order, art string) Item {
	for _, it := range o.Items {
		if it.Art == art {
			return it
		}
	}
	return Item{}
}

func TestOrderList(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	p := seedPosition(t, env)
	other := seedPosition(t, env)
	d1, d2 := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	mk := func(pos seededPosition, day *time.Time) *Order {
		t.Helper()
		o, err := svc.Create(ctx, OrderInput{ProjectID: pos.project, ElevationID: pos.elevation, SingleElevationID: pos.variant,
			Stueckzahl: 1, Liefertermin: day}, "", pos.company)
		if err != nil {
			t.Fatal(err)
		}
		return o
	}
	late, none, early := mk(p, &d1), mk(p, nil), mk(p, &d2)
	cancelled := mk(p, &d2)
	if _, err := svc.Cancel(ctx, cancelled.ID, "doppelt", "", p.company); err != nil {
		t.Fatal(err)
	}
	foreign := mk(other, &d2)
	if _, err := svc.Release(ctx, early.ID, "", "", p.company); err != nil {
		t.Fatal(err)
	}
	ids := func(f OrderFilter) []uuid.UUID {
		t.Helper()
		l, err := svc.List(ctx, p.company, f)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]uuid.UUID, len(l))
		for i, o := range l {
			out[i] = o.ID
			if o.ID == foreign.ID {
				t.Fatal("fremder Mandant in der Liste")
			}
			if o.Hinweise == nil || o.Items != nil {
				t.Fatalf("Liste: Hinweise %v, Items %v", o.Hinweise, o.Items)
			}
		}
		return out
	}
	same := func(name string, got []uuid.UUID, want ...uuid.UUID) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: %v, erwartet %v", name, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%s: %v, erwartet %v", name, got, want)
			}
		}
	}
	same("alle ohne Storno, nach Termin", ids(OrderFilter{}), early.ID, late.ID, none.ID)
	same("mit Storno", ids(OrderFilter{IncludeStorniert: true}), early.ID, cancelled.ID, late.ID, none.ID)
	same("Status storniert", ids(OrderFilter{Status: StatusStorniert}), cancelled.ID)
	same("Status freigegeben", ids(OrderFilter{Status: StatusFreigegeben}), early.ID)
	same("Projekt", ids(OrderFilter{ProjectID: &p.project}), early.ID, late.ID, none.ID)
	same("fremdes Projekt", ids(OrderFilter{ProjectID: &other.project}))
	same("Position", ids(OrderFilter{ElevationID: &p.elevation}), early.ID, late.ID, none.ID)
	same("fremde Position", ids(OrderFilter{ElevationID: &other.elevation}))
}
