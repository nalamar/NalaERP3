package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "nalaerp3/internal/app"
    "nalaerp3/internal/config"
    "nalaerp3/internal/version"
)

func main() {
    cfg := config.Load()
    log.Printf("[Start] NalaERP3 API %s (%s)", version.Version, version.Commit)

    ctx := context.Background()
    srv, err := app.New(ctx, cfg)
    if err != nil {
        log.Printf("[Fehler] Start fehlgeschlagen: %v", err)
        os.Exit(1)
    }

    // Graceful shutdown
    go func() {
        if err := srv.Start(); err != nil {
            log.Printf("[Fehler] Server: %v", err)
        }
    }()

    sigc := make(chan os.Signal, 1)
    signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
    <-sigc
    log.Printf("Beende...")
    _ = srv.HTTP.Close()
}
