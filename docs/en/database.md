# Database structure

The schema source is [`migrations/001_initial.sql`](../../migrations/001_initial.sql). PostgreSQL is the source of truth for slots, bookings, idempotency, Saga steps, and outbox events. All timestamps are stored in UTC.

## `slots` — time slots

| Field | Type | Required | Purpose |
|---|---|---:|---|
| `id` | UUID | yes | Unique slot identifier. |
| `region_id` | TEXT | yes | Region and shard-routing key. |
| `resource_id` | TEXT | yes | Resource that owns the slot. |
| `starts_at` | TIMESTAMPTZ | yes | Slot start time. |
| `ends_at` | TIMESTAMPTZ | yes | Slot end time; must be after `starts_at`. |
| `status` | TEXT | yes | `AVAILABLE`, `RESERVED`, `BOOKED`, or `CANCELLED`. |
| `version` | BIGINT | yes | Row version for concurrent-update protection. |
| `created_at` / `updated_at` | TIMESTAMPTZ | yes | Creation and last-update timestamps. |

Indexes support searches by region, resource, and time range. A partial index accelerates available-slot queries.

## `bookings` — reservations

| Field | Type | Required | Purpose |
|---|---|---:|---|
| `id` | UUID | yes | Booking ID returned by the API. |
| `slot_id` | UUID | yes | Reference to `slots.id`. |
| `tenant_id` | TEXT | yes | Organization or tenant scope where the booking is created. |
| `customer_ref` | TEXT | yes | Specific user within `tenant_id`; external pseudonym without PII. |
| `status` | TEXT | yes | `PENDING_CONFIRMATION`, `CONFIRMED`, `CANCELLED`, or `FAILED`. |
| `idempotency_key` | TEXT | yes | Key for retrying a write request. |
| `request_hash` | TEXT | yes | Request-body hash used to validate retries. |
| `expires_at` | TIMESTAMPTZ | no | Expiry of a temporary reservation. |
| `created_at` / `updated_at` | TIMESTAMPTZ | yes | Creation and last-update timestamps. |

`tenant_id` identifies the organization, while `customer_ref` identifies the user inside it. For example: `company-acme` + `user-42`.

The `(tenant_id, idempotency_key)` unique constraint prevents duplicates. A partial unique index allows at most one active booking (`PENDING_CONFIRMATION` or `CONFIRMED`) per slot.

## `booking_steps` — Saga steps

| Field | Type | Required | Purpose |
|---|---|---:|---|
| `booking_id` | UUID | yes | Booking reference; deleting a booking deletes its steps. |
| `step` | TEXT | yes | Operation name: payment, inventory, loyalty, blockchain, and so on. |
| `status` | TEXT | yes | `PENDING`, `RUNNING`, `SUCCEEDED`, `FAILED`, or `COMPENSATED`. |
| `attempt` | INTEGER | yes | Number of execution attempts. |
| `last_error` | TEXT | no | Last diagnostic error without secrets or PII. |
| `updated_at` | TIMESTAMPTZ | yes | Last step-update time. |

The primary key is `(booking_id, step)`, so a step cannot be duplicated.

## `idempotency_keys` — stored retry results

| Field | Type | Required | Purpose |
|---|---|---:|---|
| `tenant_id` | TEXT | yes | Organization and scope in which the key is unique. |
| `key` | TEXT | yes | Value of the `Idempotency-Key` header. |
| `request_hash` | TEXT | yes | Hash of the original request. |
| `response_status` | INTEGER | no | Stored HTTP response status. |
| `response_payload` | JSONB | no | Stored response body for a retry. |
| `expires_at` | TIMESTAMPTZ | yes | When retention cleanup may remove the row. |
| `created_at` | TIMESTAMPTZ | yes | Time of the first request. |

The primary key is `(tenant_id, key)`. Reusing a key with a different `request_hash` must return a conflict.

## `outbox_events` — events waiting for publication

| Field | Type | Required | Purpose |
|---|---|---:|---|
| `id` | UUID | yes | Unique `event_id`. |
| `aggregate_id` | UUID | yes | Booking or other aggregate identifier. |
| `type` | TEXT | yes | Event type, for example `BookingReserved`. |
| `schema_version` | INTEGER | yes | Payload format version. |
| `payload` | JSONB | yes | Event body without PII; not a direct API response. |
| `correlation_id` | TEXT | no | Link to the originating user request. |
| `causation_id` | TEXT | no | Event or command that caused this event. |
| `published_at` | TIMESTAMPTZ | no | Successful Kafka-publication time; `NULL` means pending. |
| `created_at` | TIMESTAMPTZ | yes | Write time in the same transaction as the booking. |

The outbox relay selects rows where `published_at IS NULL`, publishes them idempotently, and then records the publication time. `idx_outbox_unpublished` accelerates this query.

## Integrity rules

- Slot reservation and booking creation run in one PostgreSQL transaction.
- Redis, Kafka, and ClickHouse are not sources of truth for concurrent booking decisions.
- Schema changes use new versioned migrations; existing migrations are immutable.
- PII must not be placed in `customer_ref`, outbox payloads, or blockchain proofs; use an external pseudonym.
