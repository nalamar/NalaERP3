package migrate_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"nalaerp3/internal/testutil"
)

// Migration 111 (Backlog H.5.5a): Beleg- und Materialnummern sind je
// Mandant eindeutig - dieselbe Nummer in zwei Mandanten geht, im selben
// Mandanten (auch ohne Mandant) nicht.
func TestNumbersUniquePerCompany(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	n := uuid.NewString()[:8]
	a, b := "nr-a-"+n, "nr-b-"+n
	contact := "ct-nr-" + n
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO company_profiles (id, name) VALUES ($1, 'Nummern A'), ($2, 'Nummern B')`, []any{a, b}},
		{`INSERT INTO contacts (id, typ, rolle, name, company_id) VALUES ($1, 'org', 'both', 'Kontakt', $2)`, []any{contact, a}},
	} {
		if _, err := env.PG.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}

	type insertFn func(company *string, nummer string) error
	exec := func(sql string, args ...any) error {
		_, err := env.PG.Exec(ctx, sql, args...)
		return err
	}
	tables := []struct {
		name, index string
		nullable    bool // ohne Mandant moeglich
		insert      insertFn
	}{
		{"invoices_out", "invoices_out_company_nummer_key", true, func(c *string, nr string) error {
			return exec(`INSERT INTO invoices_out (id, nummer, contact_id, invoice_date, company_id) VALUES ($1,$2,$3,CURRENT_DATE,$4)`, uuid.New(), nr, contact, c)
		}},
		{"purchase_orders", "purchase_orders_company_number_key", true, func(c *string, nr string) error {
			return exec(`INSERT INTO purchase_orders (id, number, supplier_id, company_id) VALUES ($1,$2,$3,$4)`, uuid.NewString(), nr, contact, c)
		}},
		{"rfqs", "rfqs_company_nummer_key", false, func(c *string, nr string) error {
			return exec(`INSERT INTO rfqs (id, nummer, company_id) VALUES ($1,$2,$3)`, uuid.NewString(), nr, c)
		}},
		{"sales_orders", "sales_orders_company_nummer_key", true, func(c *string, nr string) error {
			// je Auftrag ein eigenes Angebot (source_quote_id ist eindeutig)
			quote := uuid.New()
			if err := exec(`INSERT INTO quotes (id, nummer, root_quote_id, revision_no, contact_id, status, quote_date, currency, net_amount, tax_amount, gross_amount, company_id)
				VALUES ($1, $2, $1, 1, $3, 'draft', CURRENT_DATE, 'EUR', 0, 0, 0, $4)`, quote, "AN-"+quote.String()[:8], contact, a); err != nil {
				return err
			}
			return exec(`INSERT INTO sales_orders (id, nummer, source_quote_id, contact_id, order_date, company_id) VALUES ($1,$2,$3,$4,CURRENT_DATE,$5)`, uuid.New(), nr, quote, contact, c)
		}},
		{"materials", "materials_company_nummer_key", true, func(c *string, nr string) error {
			return exec(`INSERT INTO materials (id, nummer, bezeichnung, typ, einheit, company_id) VALUES ($1,$2,'Material','artikel','Stk',$3)`, uuid.NewString(), nr, c)
		}},
	}
	duplicate := func(t *testing.T, err error, index string) {
		t.Helper()
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != index {
			t.Fatalf("erwartet doppelte Nummer in %s, bekommen %v", index, err)
		}
	}
	for _, tb := range tables {
		t.Run(tb.name, func(t *testing.T) {
			nr := "NR-" + tb.name + "-" + n
			if err := tb.insert(&a, nr); err != nil {
				t.Fatalf("Mandant A: %v", err)
			}
			if err := tb.insert(&b, nr); err != nil {
				t.Fatalf("gleiche Nummer in Mandant B abgelehnt: %v", err)
			}
			duplicate(t, tb.insert(&a, nr), tb.index)
			if tb.nullable {
				if err := tb.insert(nil, nr); err != nil {
					t.Fatalf("ohne Mandant: %v", err)
				}
				duplicate(t, tb.insert(nil, nr), tb.index)
			}
		})
	}
	// Rechnungsentwuerfe ohne Nummer bleiben mehrfach moeglich.
	for i := 0; i < 2; i++ {
		if err := exec(`INSERT INTO invoices_out (id, contact_id, invoice_date, company_id) VALUES ($1,$2,CURRENT_DATE,$3)`, uuid.New(), contact, a); err != nil {
			t.Fatalf("Entwurf ohne Nummer %d: %v", i, err)
		}
	}
}
