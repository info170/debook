# Slot Booking System

Project documentation is maintained in English.

- [System Design](system-design.md)


- [Database structure](database.md)
- [OpenAPI / Swagger](../openapi.yaml)


## Swagger / OpenAPI

The API specification is stored in [openapi.yaml](../openapi.yaml). With Compose running, Swagger UI is available at [http://localhost:8081](http://localhost:8081).

```bash
docker compose up -d swagger-ui
```

Swagger documents health/readiness, slot search, booking creation, booking retrieval, and cancellation. The API runs at [http://localhost:8080](http://localhost:8080).

## Slot seeder

The seeder clears the current slots and related bookings, then creates one available slot per minute. The default is 365 days, or 525,600 slots.

```bash
docker compose run --rm --entrypoint seed-slots api
```

Override the defaults when needed:

```bash
docker compose run --rm --entrypoint seed-slots api \
  -region eu-1 \
  -resource room-1 \
  -days 365 \
  -start 2026-01-01T00:00:00Z
```

The seeder runs `TRUNCATE ... CASCADE`, removing existing bookings, Saga state, idempotency records, outbox events, and slots. Use it only to prepare or fully reset local data.



## Slot search

Use `GET /api/v1/slots`. `region_id` and `status` are required. `resource_id`, `from`, and `to` are optional. Pass multiple statuses as a comma-separated value, for example `status=RESERVED,BOOKED`; they represent alternative statuses (`OR`), while the other filters are combined with `AND`.

```text
GET /api/v1/slots?region_id=eu-1&resource_id=room-1&status=AVAILABLE&from=2026-01-01T00:00:00Z&to=2026-01-01T02:00:00Z
```
- [User booking flow](user-flow.md)
- [Autheo: Testnet / Mainnet config](blockchain-config.md)
- [Blockchain deployer](../../blockchain-deployer/README.md)
- [Observability](observability.md)
- [Security](security.md)
