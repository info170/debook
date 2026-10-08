# Observability

The local Compose stack includes Prometheus and Jaeger. The API exposes `/metrics` with basic counters for HTTP requests and 5xx responses; Prometheus scrapes them from `api:8080/metrics`. API logs are structured JSON and include method, path, status, duration, and `request_id`.

Booking diagnosis uses `booking_id`, outbox/Kafka logs, and the `booking_steps` and `blockchain_proofs` tables. Container state is checked through `/healthz` and `/readyz`. Local interfaces are available at Prometheus `http://localhost:9090` and Jaeger `http://localhost:16686`.

The API also has read/write timeouts and a 64 KiB POST body limit. This is the local observability baseline; production should add OTLP trace export, dashboards, and alerts.

The API creates a `request_id`, returns it in `X-Request-ID`, and includes it in request logs. `/readyz` checks PostgreSQL and Redis, while `/healthz` only reports that the process is alive.
