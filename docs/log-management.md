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

This is operational logging, not a financial audit ledger. Payment records and payment events remain in PostgreSQL. Alerting, database query-level logs and pool metrics are not included. Health checks are logged too.

## Grafana, Loki and Alloy (local development)

Set GRAFANA_ADMIN_PASSWORD in .env to a unique password before the first start.
Start the optional profile:

```sh
docker compose --profile observability up -d --build
docker compose --profile observability ps
curl -i http://localhost:8080/health/live
```

Open http://localhost:3001 (or GRAFANA_PORT), log in with the configured admin credentials,
then open Dashboards > Payment Gateway > Payment Gateway | Logs & HTTP.
The Loki datasource is pre-provisioned. Allow a few seconds for discovery and ingestion.
Use the Request ID textbox to filter the application log panel. In Explore, select Loki:

```logql
{service="payment-gateway"} | json | msg="http_request"
{service="payment-gateway"} | json | level="ERROR"
{service="payment-gateway"} | json | request_id="REPLACE_WITH_ID"
```

HTTP counts include health checks and depend on LOG_LEVEL=INFO (or DEBUG).
The dashboard measures logs, not authoritative financial transactions or database metrics.
P95 is the 95th percentile of recorded durations; no latency data means no samples.
Loki retention is seven days, not a hard disk quota. Grafana, Loki and Alloy positions have named volumes.
Do not use docker compose down -v unless intentionally deleting data, including PostgreSQL.

Grafana binds only to localhost, Loki and Alloy have no published ports. For a remote server,
use SSH port forwarding, e.g. ssh -L 3001:127.0.0.1:3001 user@server.
This is a local lab setup: Alloy requires Docker socket access, which is privileged even
when the socket bind mount is read-only. Do not expose the socket or use this configuration
unchanged in production. Loki has no authentication and shares the local Compose network.
Grafana admin password environment settings initialize a new database only.

Troubleshooting: docker compose --profile observability logs --tail=100 alloy loki grafana
