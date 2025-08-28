# Architekturübersicht

## Komponenten
- API-Server: Go 1.24, REST, UTF-8, `de-DE`
- PostgreSQL 17: Relationale Daten, ICU-Collation `de-x-icu`
- MongoDB 7: Dokumente/Dateien (GridFS)
- Redis 7: Sessions/Rate-Limiting/Cache
- Flutter Client: Web (3000) in Entwicklung, später Desktop (Win/Linux/Mac)

## API Basis
- Healthcheck: `GET /healthz`
- Version: `GET /version`
- Namespace: `/api/v1` (Materialverwaltung folgt)

## Konfiguration
- `.env` bzw. Umgebungsvariablen
- UTF‑8 via `LANG/LC_ALL=C.UTF-8`

## Build/Deploy
- Docker-only, Multi-Stage, distroless Runtime
- `docker compose up --build`

