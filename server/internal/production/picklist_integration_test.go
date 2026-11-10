package production

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Daten der Kommissionierliste (Backlog H.3.3a): offene Zeilen, Plaetze
// aus dem Reservierungslager, ohne Lager aus allen Lagern, Status und
// Mandant.
func TestPickList(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	p := seedPosition(t, env)
	other := seedPosition(t, env)
	seedMaterials(t, env, p.variant)
	artMat, wh := seedStock(t, env, p.company, "Stk", 10)
	profMat := addMaterial(t, env, p.company, "m", wh, 0)
	_, wh2 := seedStock(t, env, p.company, "Stk", 0)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := env.PG.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	// Profil: R1 5 m, R2 8 m im Lager, 100 m im zweiten Lager.
	r1, r2 := "loc-"+uuid.NewString()[:12], "loc-"+uuid.NewString()[:12]
	exec(`INSERT INTO locations (id, warehouse_id, code, name) VALUES ($1,$3,'R1','Regal 1'), ($2,$3,'R2','Regal 2')`, r1, r2, wh)
	exec(`INSERT INTO stock_movements (id, material_id, warehouse_id, location_id, quantity, uom, movement_type)
		VALUES ($1,$4,$5,$6,5,'m','purchase'), ($2,$4,$5,$7,8,'m','purchase'), ($3,$4,$8,NULL,100,'m','purchase')`,
		uuid.NewString(), uuid.NewString(), uuid.NewString(), profMat, wh, r1, r2, wh2)
	exec(`UPDATE single_elevation_articles SET material_id=$2 WHERE single_elevation_id=$1`, p.variant, artMat)
	exec(`UPDATE single_elevation_profiles SET material_id=$2 WHERE single_elevation_id=$1`, p.variant, profMat)
	create := func() *Order {
		t.Helper()
		o, err := svc.Create(ctx, OrderInput{ProjectID: p.project, ElevationID: p.elevation, SingleElevationID: p.variant, Stueckzahl: 2}, "", p.company)
		if err != nil {
			t.Fatal(err)
		}
		return o
	}
	line := func(l *PickList, art string) *PickLine {
		for i := range l.Lines {
			if l.Lines[i].Art == art {
				return &l.Lines[i]
			}
		}
		return nil
	}

	o := create()
	if _, err := svc.PickList(ctx, o.ID, p.company); !errors.Is(err, errPickListStatus) {
		t.Fatalf("Entwurf: %v", err)
	}
	if _, err := svc.Release(ctx, o.ID, wh, "", p.company); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PickList(ctx, o.ID, other.company); !errors.Is(err, ErrNotFound) {
		t.Fatalf("anderer Mandant: %v", err)
	}
	l, err := svc.PickList(ctx, o.ID, p.company)
	if err != nil {
		t.Fatal(err)
	}
	if l.Nummer != o.Nummer || l.ProjectNummer != "P-1" || l.PositionNummer != "3" || l.Stueckzahl != 2 || l.Lager != wh+" – Testlager" || len(l.Lines) != 3 {
		t.Fatalf("Kopf: %+v", l)
	}
	// Profil 2 Stueck je Element x 2 x 1,2304 m = 4,9216 m, nur Plaetze im Reservierungslager.
	prof := line(l, ArtProfil)
	if prof == nil || prof.Menge != 4.9216 || prof.Einheit != "m" || prof.Gesamt || prof.Abmessung != "L 1230,4 mm" ||
		prof.MaterialNummer != profMat || prof.MaterialBezeichnung != "Testmaterial" ||
		!slices.Equal(prof.Plaetze, []string{wh + " / R2: 8 m", wh + " / R1: 5 m"}) {
		t.Fatalf("Profil: %+v", prof)
	}
	if art := line(l, ArtArtikel); art == nil || art.Menge != 3 || art.Einheit != "Stk" || !slices.Equal(art.Plaetze, []string{wh + ": 10 Stk"}) {
		t.Fatalf("Artikel: %+v", art)
	}
	// Glas ohne Material: Gesamtbedarf, keine Plaetze.
	if glas := line(l, ArtGlas); glas == nil || !glas.Gesamt || glas.Menge != 2 || glas.Abmessung != "1100 × 1350 mm" || len(glas.Plaetze) != 0 || glas.MaterialNummer != "" {
		t.Fatalf("Glas: %+v", glas)
	}

	// Artikel ganz entnommen: Zeile faellt weg, Profil teilweise: Rest.
	if _, err := svc.Start(ctx, o.ID, "", p.company); err != nil {
		t.Fatal(err)
	}
	g, _ := svc.Get(ctx, o.ID, p.company)
	for art, menge := range map[string]float64{ArtArtikel: 3, ArtProfil: 1} {
		if _, err := svc.Withdraw(ctx, o.ID, WithdrawalInput{ItemID: itemOf(t, g, art).ID.String(), WarehouseID: wh, LocationID: r2, Menge: menge}, "", p.company); err != nil {
			t.Fatal(err)
		}
	}
	l, err = svc.PickList(ctx, o.ID, p.company)
	if err != nil {
		t.Fatal(err)
	}
	if line(l, ArtArtikel) != nil || len(l.Lines) != 2 {
		t.Fatalf("nach Entnahme: %+v", l.Lines)
	}
	if prof := line(l, ArtProfil); prof.Menge != 3.9216 || !slices.Equal(prof.Plaetze, []string{wh + " / R2: 7 m", wh + " / R1: 5 m"}) {
		t.Fatalf("Profil nach Entnahme: %+v", prof)
	}

	// Ohne Reservierungslager: Plaetze aus allen Lagern, kein Lager im Kopf.
	o2 := create()
	if _, err := svc.Release(ctx, o2.ID, "", "", p.company); err != nil {
		t.Fatal(err)
	}
	l2, err := svc.PickList(ctx, o2.ID, p.company)
	if err != nil {
		t.Fatal(err)
	}
	if prof := line(l2, ArtProfil); l2.Lager != "" || prof == nil || !slices.Equal(prof.Plaetze, []string{wh2 + ": 100 m", wh + " / R2: 7 m", wh + " / R1: 5 m"}) {
		t.Fatalf("ohne Lager: %q %+v", l2.Lager, prof)
	}

	// Fertig und storniert: keine Liste.
	if _, err := svc.Finish(ctx, o.ID, "", p.company); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Cancel(ctx, o2.ID, "doppelt", "", p.company); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uuid.UUID{o.ID, o2.ID} {
		if _, err := svc.PickList(ctx, id, p.company); !errors.Is(err, errPickListStatus) {
			t.Fatalf("abgeschlossen: %v", err)
		}
	}
}
