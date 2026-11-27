# Log management

The API writes structured JSON through Go slog to stdout. The API container retains up to three 10 MB json-file logs (rotation is approximate). Set LOG_LEVEL=DEBUG, INFO, WARN or ERROR; default INFO.

- http_request: server-generated request_id (also X-Request-ID response header), method, route template, status, duration_ms, response_bytes and API error_code when available.
- provider_request: same request_id, provider, operation, provider HTTP status (0 when no response), outcome and duration_ms.
- snap_creation_unconfirmed: payment ID for reconciliation.
- HTTP 4xx is WARN, 5xx and panics ERROR, other statuses INFO.
- Unmatched URLs are labeled unmatched instead of logging arbitrary paths.
- Request/response bodies, authorization headers, query strings, customer data and provider tokens are not logged by the new middleware.

View: docker compose logs -f api
Filter JSON (requires jq): docker compose logs --no-log-prefix api | jq -R 'fromjson? | select(.level == "ERROR")'
Find a request: docker compose logs --no-log-prefix api | jq -R 'fromjson? | select(.request_id == "REPLACE_WITH_ID")'

This is operational logging, not a financial audit ledger. Payment records and payment events remain in PostgreSQL. Centralized collection, alerting, database query-level logs and pool metrics are not included. Health checks are logged too.
