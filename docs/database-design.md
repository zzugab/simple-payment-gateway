# Database design

This project uses PostgreSQL as the source of truth for orders and payment state.

## Tables

- `customers`: customer contact data. Email uniqueness is case-insensitive.
- `orders`: business order and amount in the smallest currency unit (IDR rupiah is stored as an integer).
- `payments`: provider-facing payment record, Midtrans order/transaction identifiers, Snap token and current provider status.
- `payment_attempts`: each attempt to create or retry a provider payment.
- `payment_events`: append-only audit trail for payment state transitions.
- `webhook_events`: durable inbox for provider notifications. The provider + event key unique constraint is the first duplicate-delivery guard.
- `idempotency_keys`: request key and canonical request hash to prevent duplicate order/payment creation.
- `outbox_events`: durable work queue written in the same database transaction as domain changes.

## Important invariants

1. Amounts are integer values; never use floating point for money.
2. A payment references one order. Both rows are retained; use status changes rather than deleting financial records.
3. Provider order IDs are unique per provider.
4. Provider transaction IDs are unique when present.
5. Webhook deliveries are persisted before processing and deduplicated by provider + event key.
6. Idempotency keys are scoped and bound to a request hash. Reusing a key with a different request must return a conflict.
7. State transitions must be validated in application code and recorded in `payment_events`.
8. Outbox records must be inserted in the same SQL transaction as the state change that requires asynchronous work.

## Indexing and high-traffic considerations

Indexes cover common access paths: order status/time, customer history, payment status/time, provider identifiers, webhook processing state, and pending outbox work. The partial outbox index keeps completed work out of the hot polling index.

These indexes are a starting point, not a claim of unlimited throughput. Validate with realistic data and k6 before adding replicas, partitioning, or a message broker. Use bounded PostgreSQL connection pools and avoid holding SQL transactions open while making network calls to Midtrans.

## Migration

Migrations are versioned SQL files in `db/migrations`. Docker Compose runs the migration service after PostgreSQL is healthy and before the API starts. To inspect migration state:

```sh
docker compose run --rm migrate version
```

To roll back one migration in a disposable development environment:

```sh
docker compose run --rm migrate down 1
```

A rollback drops the initial schema and its data. Do not run it against data you need to keep.
