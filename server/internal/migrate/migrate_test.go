// Externes Testpaket (migrate_test statt migrate), damit testutil importiert
// werden kann, ohne einen Importzyklus zu erzeugen: testutil.SetupIntegrationEnv
// ruft selbst migrate.Run auf.
package migrate_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"nalaerp3/internal/migrate"
	"nalaerp3/internal/testutil"
)

// TestRunTracksEveryMigrationFile belegt Backlog 0.13/ADR 0005: nach einem
// Run() enthaelt schema_migrations genau einen Eintrag pro .sql-Datei im
// migrations/-Verzeichnis (dynamisch ermittelt statt einer hartcodierten
// Zahl, damit der Test nicht bei jeder neuen Migration angepasst werden
// muss).
func TestRunTracksEveryMigrationFile(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()

	entries, err := os.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	wantCount := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			wantCount++
		}
	}

	var gotCount int
	if err := env.PG.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&gotCount); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if gotCount != wantCount {
		t.Fatalf("expected schema_migrations to contain %d rows (one per .sql file), got %d", wantCount, gotCount)
	}
}

// TestRunIsIdempotentOnRepeatedInvocation belegt den eigentlichen Zweck des
// Versions-Trackings: ein erneuter Run() gegen dieselbe, bereits migrierte
// DB fuehrt KEINE Migration erneut aus. Waere die Skip-Logik fehlerhaft und
// wuerde eine Datei doch erneut ausgefuehrt, wuerde der anschliessende
// `INSERT INTO schema_migrations` wegen des PRIMARY-KEY-Konflikts mit einem
// echten Fehler abbrechen (kein stilles Doppelzaehlen moeglich) - dieser
// Test ist also nicht nur eine Zeilenanzahl-Pruefung, sondern deckt jeden
// Skip-Logik-Fehler direkt per Fehlerrueckgabe von Run() auf.
func TestRunIsIdempotentOnRepeatedInvocation(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t) // fuehrt Run() bereits einmal aus
	ctx := context.Background()

	var countAfterFirst int
	if err := env.PG.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&countAfterFirst); err != nil {
		t.Fatalf("query schema_migrations after first run: %v", err)
	}

	if err := migrate.Run(ctx, env.PG); err != nil {
		t.Fatalf("expected second Run to succeed without re-executing migrations, got %v", err)
	}
	if err := migrate.Run(ctx, env.PG); err != nil {
		t.Fatalf("expected third Run to succeed without re-executing migrations, got %v", err)
	}

	var countAfterThird int
	if err := env.PG.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&countAfterThird); err != nil {
		t.Fatalf("query schema_migrations after third run: %v", err)
	}
	if countAfterThird != countAfterFirst {
		t.Fatalf("expected schema_migrations row count to stay stable across repeated Run calls, got %d then %d", countAfterFirst, countAfterThird)
	}
}
