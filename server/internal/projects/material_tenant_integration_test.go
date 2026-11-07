package projects

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Projektzeilen bekommen nur Material des eigenen Mandanten, beim
// Logikal-Import und beim Verknuepfen von Hand (Backlog H.5.6).
func TestVariantMaterialStaysInTenant(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewService(env.PG)
	n := uuid.NewString()[:8]
	company, other := "pj-"+n, "pjf-"+n
	project, phase, elevation, variant, article := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	own, foreign := "mat-own-"+n, "mat-foreign-"+n
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO company_profiles (id, name) VALUES ($1, 'Projekttest'), ($2, 'Fremdmandant')`, []any{company, other}},
		{`INSERT INTO materials (id, nummer, bezeichnung, typ, einheit, company_id) VALUES ($1,$1,'Eigen','artikel','Stk',$2), ($3,$3,'Fremd','artikel','Stk',$4)`, []any{own, company, foreign, other}},
		{`INSERT INTO projects (id, nummer, name, company_id) VALUES ($1, 'P-1', 'Testprojekt', $2)`, []any{project, company}},
		{`INSERT INTO project_phases (id, project_id, nummer, name) VALUES ($1, $2, '1', 'Haus A')`, []any{phase, project}},
		{`INSERT INTO project_elevations (id, phase_id, nummer, name, menge) VALUES ($1, $2, '3', 'Fenster', 1)`, []any{elevation, phase}},
		{`INSERT INTO project_single_elevations (id, elevation_id, name, menge) VALUES ($1, $2, 'links', 1)`, []any{variant, elevation}},
		{`INSERT INTO single_elevation_articles (id, single_elevation_id, article_code, qty) VALUES ($1, $2, 'A-1', 1)`, []any{article, variant}},
	} {
		if _, err := env.PG.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	linked := func() string {
		t.Helper()
		var id *string
		if err := env.PG.QueryRow(ctx, `SELECT material_id FROM single_elevation_articles WHERE id=$1`, article).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if id == nil {
			return ""
		}
		return *id
	}

	// Import: Artikelcode findet nur Material des Mandanten.
	for _, c := range []struct{ code, company, want string }{
		{foreign, company, ""},
		{"gibt-es-nicht", company, ""},
		{own, company, own},
		{foreign, other, foreign},
	} {
		if got := svc.materialIDByCode(ctx, c.code, c.company); got != c.want {
			t.Fatalf("Code %s im Mandanten %s: %q, erwartet %q", c.code, c.company, got, c.want)
		}
	}

	// Von Hand: fremdes und unbekanntes Material abgelehnt, nichts verknuepft.
	for _, bad := range []string{foreign, "gibt-es-nicht"} {
		err := svc.LinkVariantMaterial(ctx, project.String(), variant.String(), "articles", article.String(), bad, company)
		if err == nil || err.Error() != "Material nicht gefunden" {
			t.Fatalf("Material %s: %v", bad, err)
		}
		if got := linked(); got != "" {
			t.Fatalf("nach Ablehnung verknuepft: %s", got)
		}
	}
	if err := svc.LinkVariantMaterial(ctx, project.String(), variant.String(), "articles", article.String(), own, company); err != nil {
		t.Fatalf("eigenes Material: %v", err)
	}
	if got := linked(); got != own {
		t.Fatalf("verknuepft: %q, erwartet %q", got, own)
	}
}
