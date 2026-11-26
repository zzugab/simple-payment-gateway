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
- Migrations: golang-migrate, executed as a one-shot Docker Compose service.
- Payment provider: Midtrans Snap, Sandbox first.
- Async processing: PostgreSQL transactional outbox before an external broker.
- Cache: Redis deferred until there is a measured use case.
- Testing: Go tests and k6.
- Observability: structured logs, Prometheus, and OpenTelemetry in later milestones.

## Current milestone: Order API foundation
- Go HTTP server with graceful shutdown and health endpoints.
- PostgreSQL connection pool.
- Versioned schema for customers, orders, payments, payment attempts, payment events, webhook inbox, idempotency keys, and transactional outbox.
- Order creation and retrieval endpoints with request validation and PostgreSQL transaction handling.
- Idempotent order creation using the `Idempotency-Key` request header.
- Docker Compose waits for PostgreSQL health, runs migrations, then starts the API.
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
2. Build and start the stack:
   ```sh
   docker compose up --build -d
   ```
   Compose starts PostgreSQL, waits until it is healthy, runs the versioned SQL migrations, and starts the API only if migration succeeds.
3. Check containers and migration output:
   ```sh
   docker compose ps -a
   docker compose logs migrate
   docker compose logs -f api
   ```
4. Check health endpoints:
   ```sh
   curl http://localhost:8080/health/live
   curl http://localhost:8080/health/ready
   ```

The API and migration service connect to PostgreSQL using the internal Compose hostname `postgres:5432`. The host port defaults to `5433` to avoid clashing with a local PostgreSQL service already using `5432`. Set `POSTGRES_PORT` in `.env` if you need another host port.

## Inspect the database
```sh
docker compose exec postgres psql -U payment -d payment_lab
```
Then:
```sql
\\dt
SELECT version, dirty FROM schema_migrations;
```
Exit psql with `\\q`.

## Migration commands
Run pending migrations:
```sh
docker compose run --rm migrate up
```
Show current version:
```sh
docker compose run --rm migrate version
```
Roll back one migration (destructive to the initial schema and its data):
```sh
docker compose run --rm migrate down 1
```

To stop the containers:
```sh
docker compose down
```

To stop the containers and delete the local database volume (destructive; removes local database data):
```sh
docker compose down -v
```

## Optional: run Go directly on the host
If Go 1.24+ is installed, you can still run the API outside Docker. Start PostgreSQL and use a host-side database URL with `localhost` and the mapped host port:
```sh
docker compose up -d postgres
go mod tidy
set -a && source .env && set +a
go run ./cmd/api
```
For this optional workflow, set `DATABASE_URL` in your shell to use `localhost:5433` (or your configured `POSTGRES_PORT`). The Compose API service uses its own internal database URL automatically.

## Order API quick test

Create an order (replace the idempotency key with a new unique value for a new order):

```sh
curl -i -X POST http://localhost:8080/api/v1/orders \\
  -H 'Content-Type: application/json' \\
  -H 'Idempotency-Key: checkout-test-0001' \\
  -d '{"customer":{"name":"Andika","email":"andika@example.com","phone":"08123456789"},"amount":125000,"currency":"IDR","description":"Sandbox test order"}'
```

Copy the returned `orderNumber` and retrieve it:

```sh
curl -i http://localhost:8080/api/v1/orders/ORD-REPLACE-WITH-ORDER-NUMBER
```

See [docs/order-api.md](docs/order-api.md) for request/response details and idempotency behavior.

## Project layout
- cmd/api: application entry point
- internal/config: environment configuration
- internal/platform: infrastructure adapters
- internal/server: HTTP server and routing
- db/migrations: versioned SQL migrations
- docs/adr: architecture decision records
- docs/database-design.md: schema purpose, invariants, and indexing notes
- tests/load: k6 load tests (later milestone)

## Security notes
- The committed .env.example contains development-only credentials. Use strong, private values outside local development.
- Keep Midtrans Server Key and other credentials on the backend only.
- Verify Midtrans notifications before changing payment state.
- Treat browser callbacks as UX signals, not proof of payment.
- Store money as integers, never float.
- Make webhook processing idempotent and persist events before asynchronous work.

## Roadmap
1. Database foundation: schema, migrations, constraints, and indexes.
2. Payment domain: order/payment APIs and Midtrans Snap Sandbox.
3. Customer interface: checkout flow and payment result pages.
4. Admin interface: transaction search, status, and event timeline.
5. Reliability: webhook inbox, transactional outbox, retries, and reconciliation.
6. Performance: k6, profiling, metrics, tracing, and scaling experiments.

## Sandbox payment preview

Set MIDTRANS_SERVER_KEY in your local .env to the Sandbox Server Key from your Midtrans account, then run docker compose up --build -d. Open http://localhost:8080/checkout. The server uses the Sandbox endpoints only. The checkout creates an order, reserves one payment per order and opens the hosted checkout. Use test data and simulated funds only.

Endpoints:
- POST /api/v1/orders/{orderNumber}/payments
- GET /api/v1/orders/{orderNumber}/payments
- POST /api/v1/webhooks/midtrans

Set the Midtrans Sandbox payment notification URL to an HTTPS endpoint forwarding only the webhook path to the local API. Do not expose the entire unauthenticated lab API. The webhook verifies the signature and fetches authoritative transaction status before updating the database. Browser redirects never mark an order paid.

If payment creation has an uncertain outcome, the order is deliberately blocked from additional payment creation to avoid duplicate charges. Automatic recovery, refund handling, authentication, rate limiting, outbox delivery and full integration testing are not implemented yet. This is a local development preview, not a production payment gateway.
