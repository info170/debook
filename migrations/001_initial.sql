-- Initial booking domain schema. Apply with a migration runner in production.
BEGIN;

CREATE TABLE IF NOT EXISTS slots (
    id UUID PRIMARY KEY,
    region_id TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('AVAILABLE', 'RESERVED', 'BOOKED', 'CANCELLED')),
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at)
);

CREATE INDEX IF NOT EXISTS idx_slots_search ON slots (region_id, resource_id, starts_at, ends_at);
CREATE INDEX IF NOT EXISTS idx_slots_available ON slots (region_id, starts_at) WHERE status = 'AVAILABLE';

CREATE TABLE IF NOT EXISTS bookings (
    id UUID PRIMARY KEY,
    slot_id UUID NOT NULL REFERENCES slots(id),
    tenant_id TEXT NOT NULL,
    customer_ref TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('PENDING_CONFIRMATION', 'CONFIRMED', 'CANCELLED', 'FAILED')),
    idempotency_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_bookings_tenant_idempotency ON bookings (tenant_id, idempotency_key);
CREATE UNIQUE INDEX IF NOT EXISTS uq_bookings_active_slot ON bookings (slot_id)
    WHERE status IN ('PENDING_CONFIRMATION', 'CONFIRMED');
CREATE INDEX IF NOT EXISTS idx_bookings_customer ON bookings (tenant_id, customer_ref, created_at DESC);

CREATE TABLE IF NOT EXISTS booking_steps (
    booking_id UUID NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    step TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'RUNNING', 'SUCCEEDED', 'FAILED', 'COMPENSATED')),
    attempt INTEGER NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    last_error TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (booking_id, step)
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    tenant_id TEXT NOT NULL,
    key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    response_status INTEGER,
    response_payload JSONB,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, key)
);

CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY,
    aggregate_id UUID NOT NULL,
    type TEXT NOT NULL,
    schema_version INTEGER NOT NULL DEFAULT 1,
    payload JSONB NOT NULL,
    correlation_id TEXT,
    causation_id TEXT,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON outbox_events (created_at) WHERE published_at IS NULL;

COMMIT;
