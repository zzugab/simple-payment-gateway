# Simple Payment Gateway

A Go-based payment engineering lab for learning and building a reliable Midtrans integration.

## Goals
- Learn the end-to-end payment lifecycle with Midtrans Sandbox.
- Build a production-oriented foundation without premature microservices.
- Practice transaction consistency, webhook security, idempotency, and reconciliation.
- Add asynchronous processing, load testing, and observability incrementally.

## Architecture decisions
- Architecture: modular monolith first.
- HTTP: Go standard library + Chi router.
- Database: PostgreSQL with pgx.
- SQL: explicit SQL; sqlc will be introduced with the first domain queries.
- Migrations: Goose (next database milestone).
- Payment provider: Midtrans Snap, Sandbox first.
- Async processing: PostgreSQL transactional outbox before an external broker.
- Cache: Redis deferred until there is a measured use case.
- Testing: Go tests and k6.
- Observability: structured logs, Prometheus, and OpenTelemetry in later milestones.

## Current milestone: Foundation
- Go HTTP server with graceful shutdown.
- PostgreSQL connection and readiness check.
- Docker Compose runs both the Go API and PostgreSQL.
- Multi-stage Docker build; Go toolchain is not required on the host machine.
- Environment-based configuration.
- Basic GitHub Actions CI.

No Midtrans credentials are needed yet. Never commit the .env file or payment secrets.

## Requirements
- Docker Desktop (or Docker Engine) with Docker Compose v2.
- Make is optional.
- Go 1.24+ is only needed if you want to run Go commands directly on your host.

## Local setup (Docker)
1. Copy the example environment file:
   ```sh
   cp .env.example .env
   ```
2. Build the API image and start the API and database:
   ```sh
   docker compose up --build -d
   ```
3. Follow the API logs:
   ```sh
   docker compose logs -f api
   ```
4. Check the health endpoints:
   ```sh
   curl http://localhost:8080/health/live
   curl http://localhost:8080/health/ready
   ```

The API connects to PostgreSQL through the Compose service hostname `postgres`, not `localhost`. The database is marked healthy before Compose starts the API.

To stop the containers:
```sh
docker compose down
```

To stop the containers and delete the local database volume (destructive; removes local database data):
```sh
docker compose down -v
```

## Optional: run Go directly on the host
If Go 1.24+ is installed, you can still run the API outside Docker. In that case, use a host-side database URL with `localhost` as the hostname:
```sh
docker compose up -d postgres
go mod tidy
set -a && source .env && set +a
go run ./cmd/api
```
For this optional workflow, set `DATABASE_URL` in your shell to a URL using `localhost`; the Compose API service uses its own internal database URL automatically.

## Project layout
- cmd/api: application entry point
- internal/config: environment configuration
- internal/platform: infrastructure adapters
- internal/server: HTTP server and routing
- db/migrations: database migrations
- docs/adr: architecture decision records
- tests/load: k6 load tests (later milestone)

## Security notes
- The committed .env.example contains development-only credentials. Use strong, private values outside local development.
- Keep Midtrans Server Key and other credentials on the backend only.
- Verify Midtrans notifications before changing payment state.
- Treat browser callbacks as UX signals, not proof of payment.
- Store money as integers, never float.
- Make webhook processing idempotent and persist events before asynchronous work.

## Roadmap
1. Foundation: server, PostgreSQL, migrations, CI.
2. Orders and payments: domain model and Midtrans Snap Sandbox.
3. Webhooks: signature verification, status validation, idempotency.
4. Reliability: transactional outbox, worker, retries, reconciliation.
5. Performance: k6, profiling, metrics, tracing, and scaling experiments.
