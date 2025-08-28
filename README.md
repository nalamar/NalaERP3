# NalaERP3 – Grundgerüst

Deutschsprachiges ERP für die Metallbau-Branche. Client-Server-Architektur mit Go (API) und Flutter (Web/Desktop). Alle Komponenten sind UTF-8-kompatibel und containerisiert.

## Schnellstart

1. `.env` anlegen (siehe `.env.example`).
2. Docker bauen und starten:
   
   ```bash
   docker compose up --build
   ```

3. Dienste:
   - API: http://localhost:8080 (`/healthz`, `/version`)
   - Client (Flutter Web): http://localhost:3000
   - PostgreSQL: `localhost:5432` (DB `nalaerp`, User `nala`, PW `secret`)
   - MongoDB: `localhost:27017`
   - Redis: `localhost:6379`

## Struktur

- `server/`: Go 1.24 API (Docker Multi-Stage)
- `client/`: Flutter Web-Client (Entwicklung auf Port 3000)
- `docker-compose.yml`: Orchestrierung (Postgres, Mongo, Redis, API, Client)

## Hinweise

- Sprachen/Encoding: Standard `de-DE`, UTF-8.
- Versionen: Aktuelle Hauptversionen (Go 1.24, Postgres 17, Mongo 7, Redis 7).
- Nächste Schritte: Materialverwaltung Domainmodell + REST-Endpunkte, DB-Schemata (Migrations), Uploads (Mongo GridFS), Auth (JWT + Redis), Tests.

