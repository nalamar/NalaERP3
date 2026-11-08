package settings

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Fehlt einem Mandanten der Nummernkreis, uebernimmt Next das Muster des
// Mandanten `default` fuer dieselbe Entitaet, sonst "PO-{YYYY}-{NNNN}"
// (Backlog H.5.5b, ADR 0036).
func TestNextFallsBackToDefaultPattern(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewNumberingService(env.PG)
	n := uuid.NewString()[:8]
	company := "nf-" + n
	own, onlyDefault, nowhere := "eigen_"+n, "nur_default_"+n, "nirgends_"+n
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO company_profiles (id, name) VALUES ($1, 'Nummern Ersatz')`, []any{company}},
		{`INSERT INTO number_sequences (entity, pattern, next_value, company_id) VALUES ($1, 'XD-{NNN}', 40, 'default')`, []any{onlyDefault}},
		{`INSERT INTO number_sequences (entity, pattern, next_value, company_id) VALUES ($1, 'XD-{NNN}', 40, 'default'), ($1, 'EIGEN-{NNN}', 7, $2)`, []any{own, company}},
	} {
		if _, err := env.PG.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	year := time.Now().Year()
	for _, tc := range []struct {
		name, entity string
		want         []string
		pattern      string
	}{
		{"Rechnung wie default", "invoice_out", []string{fmt.Sprintf("RE-%d-0001", year), fmt.Sprintf("RE-%d-0002", year)}, "RE-{YYYY}-{NNNN}"},
		{"Muster ohne Jahr wie default", onlyDefault, []string{"XD-001", "XD-002"}, "XD-{NNN}"},
		{"ohne Muster in default", nowhere, []string{fmt.Sprintf("PO-%d-0001", year)}, "PO-{YYYY}-{NNNN}"},
		{"eigener Nummernkreis bleibt", own, []string{"EIGEN-007", "EIGEN-008"}, "EIGEN-{NNN}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, want := range tc.want {
				got, err := svc.Next(ctx, tc.entity, company)
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Fatalf("Nummer %q, erwartet %q", got, want)
				}
			}
			cfg, err := svc.Get(ctx, tc.entity, company)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Pattern != tc.pattern {
				t.Fatalf("gespeichertes Muster %q, erwartet %q", cfg.Pattern, tc.pattern)
			}
		})
	}
	// Der Nummernkreis von `default` zaehlt nicht mit.
	var next int
	if err := env.PG.QueryRow(ctx, `SELECT next_value FROM number_sequences WHERE entity=$1 AND company_id='default'`, onlyDefault).Scan(&next); err != nil {
		t.Fatal(err)
	}
	if next != 40 {
		t.Fatalf("default weitergezaehlt: next_value %d, erwartet 40", next)
	}
}
