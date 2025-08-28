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

func Run(ctx context.Context, pool *pgxpool.Pool) error {
    // Dateien lesen und sortiert ausführen
    entries, err := sqlFS.ReadDir("migrations")
    if err != nil { return err }
    names := make([]string, 0, len(entries))
    for _, e := range entries { if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") { names = append(names, e.Name()) } }
    sort.Strings(names)
    for _, n := range names {
        b, err := sqlFS.ReadFile("migrations/" + n)
        if err != nil { return err }
        log.Printf("Migration ausführen: %s", n)
        if _, err := pool.Exec(ctx, string(b)); err != nil {
            return fmt.Errorf("Migration %s fehlgeschlagen: %w", n, err)
        }
    }
    return nil
}
