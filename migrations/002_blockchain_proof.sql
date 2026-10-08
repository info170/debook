BEGIN;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS wallet_address TEXT;
CREATE TABLE IF NOT EXISTS blockchain_proofs (
 booking_id UUID PRIMARY KEY REFERENCES bookings(id) ON DELETE CASCADE,
 chain_id TEXT NOT NULL,
 contract_address TEXT NOT NULL,
 claim_hash TEXT NOT NULL UNIQUE,
 proof_id NUMERIC,
 tx_hash TEXT,
 status TEXT NOT NULL CHECK (status IN ('PENDING','SUBMITTED','CONFIRMED','FAILED')),
 confirmations INTEGER NOT NULL DEFAULT 0,
 last_error TEXT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 confirmed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_blockchain_proofs_status ON blockchain_proofs(status);
COMMIT;
