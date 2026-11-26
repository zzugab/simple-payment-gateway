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
- Docker Compose for local PostgreSQL.
- Environment-based configuration.
- Basic GitHub Actions CI.

No Midtrans credentials are needed yet. Never commit the .env file or payment secrets.

## Requirements
- Go 1.24+
- Docker Engine / Docker Desktop with Compose
- Make (optional)

## Local setup
1. Copy .env.example to .env.
2. Start PostgreSQL: docker compose up -d postgres
3. Load environment variables: set -a && source .env && set +a
4. Download dependencies: go mod tidy
5. Run the API: go run ./cmd/api

Health endpoints:
- GET /health/live — process liveness.
- GET /health/ready — PostgreSQL connectivity.

## Project layout
- cmd/api: application entry point
- internal/config: environment configuration
- internal/platform: infrastructure adapters
- internal/server: HTTP server and routing
- db/migrations: database migrations
- docs/adr: architecture decision records
- tests/load: k6 load tests (later milestone)

## Security notes
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
