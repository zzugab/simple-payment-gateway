# Midtrans R&D lab

Local Sandbox only. Use fictitious customer data and simulator funds. Never expose the unauthenticated API to the public internet.

## Run
Copy .env.example only if .env does not exist. Configure a Sandbox Server Key.
Run docker compose up -d postgres, docker compose run --rm migrate up, then docker compose up -d --build api.
Open http://localhost:8080/checkout.

## Scenarios
- Catering: four nasi boxes at IDR 25000 and two snack boxes at IDR 15000, total IDR 130000.
- KPI application: one simulated monthly subscription at IDR 99000. This is a one-time checkout, not recurring billing.
- Custom: enter arbitrary test items. The API checks integer prices, quantities and matching totals.
- Repeat identical creation: verify the same order and payment are returned.
- Cancel or expire in Sandbox: use Check status and verify database transitions.
- Pay with a supported Sandbox simulator: https://simulator.sandbox.midtrans.com/ then refresh status and verify payment events and outbox.
- Wrong key: inspect API logs and payment_attempts.error_message; never print credentials.
- Timeout: keep uncertain reservation; reconcile before retrying. Do not delete payment records.
- Existing Snap transactions cannot be retroactively enriched with customer/item details; create a fresh test order.

## Inspection
SELECT order_id, item_id, name, price, quantity FROM order_items;
SELECT provider_order_id, status, paid_at FROM payments;
SELECT payment_id, status, error_message FROM payment_attempts;
SELECT payment_id, previous_status, new_status, source FROM payment_events;
SELECT aggregate_id, event_type, status FROM outbox_events;

## Before production
Use authoritative product prices from the business backend, authenticated ownership checks, rate limits, verified webhook signing configuration, automatic reconciliation, retry policy, outbox delivery, integration tests and monitoring. Never treat a checkout redirect as proof of payment. Do not store real employee payroll or KPI data in payment metadata. Refunds and recurring billing require separate implementations.
