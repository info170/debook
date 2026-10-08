# Booking user flow

This describes the current local scenario: the user finds an available slot, books it by `slot_id`, and checks the final status.

## 0. Prepare slots

Before the user flow, the database must contain `slots` with status `AVAILABLE`. For local setup, run:

```bash
docker compose run --rm --entrypoint seed-slots api
```

The seeder creates one-minute slots and clears the existing local booking and slot dataset.

## 1. Find available slots

Request:

```http
GET /api/v1/slots?region_id=eu-1&resource_id=room-1&status=AVAILABLE&from=2026-01-01T10:00:00Z&to=2026-01-01T12:00:00Z
```

`region_id` and `status` are required. The time range is inclusive: results satisfy `starts_at >= from` and `ends_at <= to`. Multiple statuses are comma-separated alternatives, for example `status=RESERVED,BOOKED`.

The API checks Redis first. On a cache miss it queries PostgreSQL `slots`, then caches the result for 15 seconds. This step does not change PostgreSQL data.

The response contains the `slot_id` to use for booking.

## 2. Create the booking

Request:

```http
POST /api/v1/bookings
Idempotency-Key: booking-001
Content-Type: application/json
```

```json
{
  "slot_id": "...",
  "tenant_id": "company-acme",
  "customer_ref": "user-42",
  "wallet_address": "0x..."
}
```

Booking Service executes one PostgreSQL transaction:

1. Check `idempotency_keys` by `(tenant_id, Idempotency-Key)`.
2. If the same key and request hash already exist, return the stored result without creating another booking.
3. Lock the slot row with `SELECT ... FOR UPDATE`.
4. Verify `slots.status = AVAILABLE`.
5. Update the slot: `AVAILABLE → RESERVED` and increment `version`.
6. Insert `bookings` with status `PENDING_CONFIRMATION`.
7. Insert an `outbox_events` row with type `BookingReserved`.
8. Store the response in `idempotency_keys`.
9. Commit.

The API returns `202 Accepted` and a `booking_id`. An occupied slot returns `409 Conflict`; a missing slot returns `404 Not Found`.

## 3. Background processing

The outbox relay selects `outbox_events` where `published_at IS NULL`, publishes the event to Kafka, and sets `published_at` after successful publication.

The workflow worker receives `BookingReserved`, creates or updates the `booking_steps` workflow step, and in the current MVP:

- marks the step `SUCCEEDED`;
- changes `bookings.status` to `CONFIRMED`.

The full flow will execute payment, inventory, loyalty, and blockchain steps with retries and compensation. On failure, the booking should become `FAILED` or `CANCELLED` and the slot should return to `AVAILABLE`.

### Blockchain proof

After `POST /api/v1/bookings`, the outbox sends `BookingReserved` to Kafka. Workflow computes SHA-256 of `booking_id`, creates a `blockchain_proofs` row with `PENDING`, calls Autheo `createBookingProof` using the backend minter, and after the transaction is mined marks the proof `CONFIRMED`, stores `tx_hash`, and moves the booking to `CONFIRMED`. Verification uses `GET /api/v1/bookings/{booking_id}/ownership?wallet_address=0x...`, which calls the read-only `isBookingOwned` method.

## 4. Check status

Request:

```http
GET /api/v1/bookings/{booking_id}
```

The API reads the current `bookings` row. The user may receive `PENDING_CONFIRMATION`, `CONFIRMED`, `CANCELLED`, or `FAILED`.

## 5. Cancel

Request:

```http
POST /api/v1/bookings/{booking_id}/cancel
```

In one transaction the API locks the active booking and:

1. changes `bookings.status` to `CANCELLED`;
2. changes the related `slots.status` to `AVAILABLE`;
3. increments `slots.version`;
4. commits.

## Idempotency and integrity

A retry with the same `tenant_id`, `Idempotency-Key`, and request body returns the original response. Reusing the key with a different body returns a conflict. Redis accelerates reads; PostgreSQL alone decides whether a slot can be reserved.
