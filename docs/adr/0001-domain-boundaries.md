# ADR 0001: Domain boundaries and first release

- Status: accepted
- Date: 2026-10-05

## Context

The service must search and reserve time slots without overbooking, while external integrations (payment, inventory, loyalty, notifications, and blockchain proof) are asynchronous.

## Decisions

1. `Booking Service` owns slot and booking state transitions.
2. PostgreSQL is the source of truth for slots, bookings, idempotency, Saga state, and outbox events.
3. A booking request is idempotent per `(tenant_id, idempotency_key)` and locks the slot inside one database transaction.
4. The first release is a single-region modular Go service. Kafka, Redis, and external integrations are introduced behind explicit interfaces.
5. A booking is final only after mandatory Saga steps and blockchain proof succeed; until then its status is `PENDING_CONFIRMATION`.
6. Timestamps are UTC. Personal data is represented by `customer_ref` and is excluded from events and blockchain payloads.

## Consequences

The database prevents overbooking; caches and message brokers cannot authorize a reservation. Multi-region writes, tenant provisioning, cancellation windows, and the payment provider remain explicit follow-up decisions.
