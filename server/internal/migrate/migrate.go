package migrate

import (
	"context"
	"embed"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var sqlFS embed.FS

// Neue Migration hinzufuegen? Vorher `ls migrations | sort | tail -3`
// pruefen und eine Nummer STRIKT groesser als die hoechste vorhandene
// waehlen - die Ausfuehrungsreihenfolge haengt allein von sort.Strings()
// ueber den vollen Dateinamen ab. Siehe docs/adr/0003-migration-numbering.md
// fuer die Begruendung (u.a. warum die 14 historischen Nummernkollisionen
// 001-020 bewusst nicht behoben wurden).
//
// Seit Backlog 0.13/ADR 0005 gibt es Versions-Tracking (Tabelle
// schema_migrations): jede Migrationsdatei wird nur EINMAL ausgefuehrt, ein
// erneuter Run() gegen dieselbe DB ueberspringt bereits angewendete Dateien.
// Down-Migrationen bleiben bewusst manuell/dokumentiert (Kommentarblock im
// jeweiligen Migrationsfile, siehe z.B. 060_users_numbering_scope.sql) statt
// automatisiert - siehe ADR 0005 fuer die Begruendung.

// migrationAdvisoryLockKey ist ein beliebiger, aber fuer dieses Projekt
// stabiler Schluessel fuer pg_advisory_lock. Serialisiert parallele
// Run()-Aufrufe gegen dieselbe DB (z.B. mehrere Go-Testpakete, die
// gleichzeitig testutil.SetupIntegrationEnv aufrufen und dadurch alle
// gleichzeitig migrieren wollen) - OHNE diesen Lock koennten zwei Prozesse
// gleichzeitig dieselbe, noch nicht angewendete Migration ausfuehren: eine
// klassische Race Condition zwischen "geprueft: noch nicht angewendet" und
// "als angewendet markiert". Gefunden bei der Verifikation dieser Subtask
// selbst (Backlog 0.13): ein paralleler Testlauf mehrerer Pakete produzierte
// doppelte schema_migrations-Eintraege bzw. Konflikte in der Migrations-DDL
// selbst.
const migrationAdvisoryLockKey = 8272026

func Run(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("Datenbankverbindung fuer Migrationslauf fehlgeschlagen: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationAdvisoryLockKey); err != nil {
		return fmt.Errorf("Migrations-Lock fehlgeschlagen: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationAdvisoryLockKey)
	}()

	if _, err := conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS schema_migrations (
            filename text PRIMARY KEY,
            applied_at timestamptz NOT NULL DEFAULT now()
        )
    `); err != nil {
		return fmt.Errorf("schema_migrations anlegen fehlgeschlagen: %w", err)
	}

	applied, err := loadAppliedMigrations(ctx, conn)
	if err != nil {
		return err
	}

	entries, err := sqlFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, n := range names {
		if applied[n] {
			continue
		}
		b, err := sqlFS.ReadFile("migrations/" + n)
		if err != nil {
			return err
		}
		log.Printf("Migration ausführen: %s", n)
		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("Transaktion fuer Migration %s fehlgeschlagen: %w", n, err)
		}
		if _, err := tx.Exec(ctx, string(b)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("Migration %s fehlgeschlagen: %w", n, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, n); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("Migration %s als angewendet markieren fehlgeschlagen: %w", n, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("Migration %s commit fehlgeschlagen: %w", n, err)
		}
	}
	return nil
}

func loadAppliedMigrations(ctx context.Context, conn *pgxpool.Conn) (map[string]bool, error) {
	rows, err := conn.Query(ctx, `SELECT filename FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("bereits angewendete Migrationen lesen fehlgeschlagen: %w", err)
	}
	defer rows.Close()
	applied := make(map[string]bool)
	for rows.Next() {
		var fn string
		if err := rows.Scan(&fn); err != nil {
			return nil, err
		}
		applied[fn] = true
	}
	return applied, rows.Err()
}
