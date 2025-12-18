# Repository Guidelines

## Project Structure & Module Organization
- `server/`: Go 1.24 API; entrypoint `cmd/api/main.go`, domain packages under `internal/` (contacts, materials, projects, pdfgen, settings) and SQL migrations in `migrations/`.
- `client/`: Flutter Web client; `lib/` holds UI, API helpers, pages, and shared widgets; `web/` contains static assets.
- `docs/`: Architecture notes; root contains `.env.example`, `docker-compose.yml`, and SQL sample `logi.sql`.

## Build, Test, and Development Commands
- Bootstrap containers: `docker compose up --build` (API on :8080, Flutter dev server on :3000, Postgres/Mongo/Redis). Use `.env` based on `.env.example`.
- API local dev: `cd server && go run ./cmd/api` (requires Postgres, Mongo, Redis reachable via env vars).
- API tests/format: `cd server && go test ./...` and `gofmt -w .` (run before commits; no tests yet—add alongside new packages).
- Client dev: `cd client && flutter run -d chrome` or `flutter build web` for static output. Lint with `flutter analyze` and format with `dart format .`.

## Coding Style & Naming Conventions
- Go: gofmt/goimports style; package names lower_snakecase; exported types/functions use PascalCase German/English mix as appropriate; keep handlers small and in domain packages under `internal/`.
- Dart/Flutter: files snake_case, classes/widgets PascalCase, prefer const constructors; keep API calls centralized in `lib/api.dart`.
- General: UTF-8, `de-DE` defaults; avoid embedding secrets—use env vars.

## Testing Guidelines
- Add `_test.go` alongside Go packages with table-driven tests; favor integration tests hitting in-memory or test DB schemas in `migrations/`.
- Flutter: place widget tests under `client/test/`; mirror `lib/` structure. Name tests after feature (e.g., `materials_repository_test.dart`).
- Keep coverage meaningful around import/parsing logic and PDF generation helpers.

## Commit & Pull Request Guidelines
- Existing history uses release-style messages (`0.0.x.y`). For features/fixes, prefer short imperative subjects (e.g., `Add materials API validation`); tag release bumps explicitly.
- PRs: describe scope, linked issue, and how to validate locally (commands + env vars). Include screenshots/gifs for UI changes and sample payloads for API endpoints. Note DB migrations and backward-compatibility impacts.

## Security & Configuration Tips
- Do not commit `.env`; rotate secrets when sharing dumps. Keep `POSTGRES_DSN`, `MONGO_URI`, and `REDIS_PASSWORD` in local env or CI secrets.
- Validate uploads and PDFs; large files rely on GridFS—keep size limits consistent across API and proxies.
