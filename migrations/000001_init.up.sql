-- Wallets: one per user. Balance is stored in poisha (integer, never float).
CREATE TABLE wallets (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     TEXT        NOT NULL,
    phone       TEXT        NOT NULL,
    currency    TEXT        NOT NULL DEFAULT 'BDT' CHECK (currency = 'BDT'),
    balance     BIGINT      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT wallets_user_id_key UNIQUE (user_id),
    CONSTRAINT wallets_phone_key   UNIQUE (phone),
    -- Last line of defence: even if application code had a bug,
    -- the database refuses to store a negative balance.
    CONSTRAINT wallets_balance_non_negative CHECK (balance >= 0)
);

-- Wallet list (GET /api/v1/wallets): newest first, id breaks ties.
CREATE INDEX wallets_created_idx ON wallets (created_at DESC, id DESC);

-- Transfers: one row per P2P transfer.
CREATE TABLE transfers (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    from_wallet_id  UUID        NOT NULL REFERENCES wallets(id),
    to_wallet_id    UUID        NOT NULL REFERENCES wallets(id),
    amount          BIGINT      NOT NULL CHECK (amount > 0),
    note            TEXT,
    status          TEXT        NOT NULL CHECK (status IN ('COMPLETED')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT transfers_different_wallets CHECK (from_wallet_id <> to_wallet_id)
);

-- Transactions: the per-wallet ledger (what the history endpoint shows).
--   ADD_MONEY    -> reference_id = gateway_ref, no counterparty
--   TRANSFER_OUT -> reference_id = transfer id, counterparty = receiver
--   TRANSFER_IN  -> reference_id = transfer id, counterparty = sender
-- id is a BIGSERIAL: it is assigned while the wallet row is locked, so for a
-- given wallet a higher id is always a later balance change. We sort on it.
CREATE TABLE transactions (
    id                      BIGSERIAL   PRIMARY KEY,
    wallet_id               UUID        NOT NULL REFERENCES wallets(id),
    type                    TEXT        NOT NULL CHECK (type IN ('ADD_MONEY', 'TRANSFER_OUT', 'TRANSFER_IN')),
    amount                  BIGINT      NOT NULL CHECK (amount > 0),
    balance_after           BIGINT      NOT NULL CHECK (balance_after >= 0),
    counterparty_wallet_id  UUID        REFERENCES wallets(id),
    reference_id            TEXT        NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX transactions_wallet_history_idx ON transactions (wallet_id, id DESC);
CREATE INDEX transactions_wallet_created_idx ON transactions (wallet_id, created_at);

-- A gateway payment can credit a wallet only once, ever.
CREATE UNIQUE INDEX transactions_gateway_ref_key
    ON transactions (reference_id) WHERE type = 'ADD_MONEY';

-- Idempotency keys. A row is inserted in the SAME database transaction as the
-- money movement, so either both are committed or neither is.
CREATE TABLE idempotency_keys (
    scope         TEXT        NOT NULL,          -- 'add_money' or 'transfer'
    key           TEXT        NOT NULL,          -- Idempotency-Key header
    request_hash  TEXT        NOT NULL,          -- SHA-256 of the normalized request
    response_body BYTEA,                         -- the original success response
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (scope, key)
);
