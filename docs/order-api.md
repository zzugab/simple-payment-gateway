# Order API

Base path: `/api/v1`

## Create an order

`POST /api/v1/orders`

Headers:

- `Content-Type: application/json`
- `Idempotency-Key: <unique-client-generated-key>` (8–255 characters)

Example:

```json
{
  "customer": {
    "name": "Andika",
    "email": "andika@example.com",
    "phone": "08123456789"
  },
  "amount": 125000,
  "currency": "IDR",
  "description": "Order from checkout"
}
```

Successful first request returns HTTP `201 Created`:

```json
{
  "orderNumber": "ORD-<random-hex>",
  "amount": 125000,
  "currency": "IDR",
  "status": "pending",
  "description": "Order from checkout",
  "createdAt": "2026-01-01T10:00:00Z"
}
```

Repeat the same request with the same idempotency key to receive the original order with HTTP `200 OK` and `Idempotency-Replayed: true`. Reusing the key with a different normalized request returns HTTP `409 Conflict` in the idempotency contract; clients must generate a new key for a new order.

The idempotency record and order are written in one PostgreSQL transaction. The key is retained for 24 hours; expired keys can be reused. Cleanup of expired keys will be added with the background maintenance worker.

Validation errors return HTTP `422 Unprocessable Entity`. Only IDR is currently supported, and amounts are integer rupiah values.

## Retrieve an order

`GET /api/v1/orders/{orderNumber}`

Returns the order's public checkout fields. It intentionally does not return customer contact information.

## Error response

```json
{
  "error": {
    "code": "validation_error",
    "message": "amount must be greater than zero"
  }
}
```

## Notes

- Order numbers contain 128 bits of cryptographically secure random data to make unauthenticated public lookup difficult to enumerate.
- This endpoint does not create a Midtrans transaction. Payment creation is a separate milestone and will use server-side Midtrans credentials.
- Administrative listing and customer PII access require authentication and are not exposed by this public API.
