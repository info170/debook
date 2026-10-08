# Decentralized Booking Service

Initial implementation of a slot-booking service.

## Local API skeleton

```bash
go run ./cmd/api
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

The first three implementation steps are documented in `docs/adr/0001-domain-boundaries.md`, `cmd/api`, and `migrations/001_initial.sql`.

## Local Docker environment

Install and start Docker Desktop (or Docker Engine with Compose v2).

```bash
docker compose up --build -d --wait
docker compose ps -a
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

| Component | Host address | Address inside Compose |
|---|---|---|
| API | http://localhost:8080 | api:8080 |
| PostgreSQL | localhost:5432 | postgres:5432 |
| Redis | localhost:6379 | redis:6379 |
| Kafka | localhost:9092 | kafka:19092 |
| ClickHouse HTTP / native | localhost:8123 / 9000 | clickhouse:8123 / 9000 |
| Swagger UI | http://localhost:8081 | swagger-ui:8080 |
| Prometheus | http://localhost:9090 | prometheus:9090 |
| Jaeger UI | http://localhost:16686 | jaeger:16686 |
| OTLP gRPC / HTTP | localhost:4317 / 4318 | jaeger:4317 / 4318 |

PostgreSQL and ClickHouse use the `debook` user, the `debook_local` password, and the `debook` database. These credentials are intended for the local environment; ports are bound to loopback. Kafka runs as a single-node KRaft cluster without ZooKeeper.

Migrations are run manually from the PostgreSQL container. This keeps the database lifecycle separate from the API and workers:

```bash
docker compose up -d --wait postgres redis kafka clickhouse prometheus jaeger
docker compose exec -T postgres sh -c 'for migration in /migrations/*.sql; do psql -U debook -d debook -v ON_ERROR_STOP=1 -f "$migration"; done'
docker compose up -d --wait api outbox-relay workflow
```

`001_initial.sql` can be applied repeatedly because it uses `IF NOT EXISTS`. Changes to existing tables require new versioned migrations and a migration runner.

```bash
# Check tables
docker compose exec postgres psql -U debook -d debook -c '\dt'
# Check Redis and Kafka
docker compose exec redis redis-cli ping
docker compose exec kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:19092 --list
# View logs
docker compose logs -f api outbox-relay workflow
# Stop the environment while preserving data
docker compose down
```

PostgreSQL, Redis, Kafka, ClickHouse, and Prometheus data are stored in named volumes. `docker compose down -v` removes this data. Jaeger stores traces in memory and loses them when restarted.

**Current functionality:** The API currently provides only health and readiness checks. It is not yet connected to the storage services; `/readyz` indicates that the HTTP skeleton is ready. Prometheus collects its own metrics; API metrics export and trace delivery to Jaeger are planned for the observability stage. The booking worker, outbox relay, Saga, and integrations have not yet been implemented, so their containers are not created. Kafka and ClickHouse are ready for later stages; the booking business flow is not yet available.

## React Load Lab

The `frontend/` directory contains a UI for manually load-testing the `fetch slots → book a slot` scenario. Set a rate of 1 to 1,000 sessions per second, choose a duration and API parameters in the expandable settings, then click **Run test**. Every tenth successful booking is automatically cancelled.

```bash
cd frontend
npm install
npm run dev
```

The interface opens at `http://localhost:5173` by default and uses `http://127.0.0.1:8080` for the API (Docker publishes the API on the IPv4 loopback interface). Add `http://localhost:5173` to `CORS_ALLOWED_ORIGINS` to use Vite.

The Kafka configuration is based on the [official Apache Kafka Docker image](https://kafka.apache.org/41/getting-started/docker/); ClickHouse settings are based on the [image documentation](https://github.com/ClickHouse/ClickHouse/blob/master/docker/server/README.md).

## Swagger

Open [Swagger UI](http://localhost:8081) after running `docker compose up -d`. The source specification is [`docs/openapi.yaml`](docs/openapi.yaml).

## Seeding slots

The seeder clears the local bookings and slots, then creates one slot per minute for a year: `365 × 24 × 60 = 525,600` records. It uses `TRUNCATE ... CASCADE`, which also deletes existing bookings. Run it only when resetting this data is intended.

After starting PostgreSQL and applying the migrations:

```bash
docker compose run --rm --entrypoint seed-slots api
```

Options:

```bash
docker compose run --rm --entrypoint seed-slots api \
  -region eu-1 \
  -resource room-1 \
  -days 365 \
  -start 2026-01-01T00:00:00Z
```

`DATABASE_URL` is read from Compose. When running locally outside Docker, set `DATABASE_URL` or use the default PostgreSQL address at `localhost:5432`.
