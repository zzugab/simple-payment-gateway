-- Initial payment gateway schema.
CREATE TABLE customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name TEXT NOT NULL,
    email TEXT NOT NULL,
    phone TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX customers_email_lower_uidx ON customers (LOWER(email));
CREATE INDEX customers_created_at_idx ON customers (created_at DESC);

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_number VARCHAR(64) NOT NULL UNIQUE,
    customer_id UUID REFERENCES customers(id) ON DELETE RESTRICT,
    amount BIGINT NOT NULL CHECK (amount > 0),
    currency CHAR(3) NOT NULL DEFAULT 'IDR',
    status VARCHAR(24) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'paid', 'cancelled', 'expired', 'refunded', 'partially_refunded')),
    description TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX orders_customer_created_idx ON orders (customer_id, created_at DESC);
CREATE INDEX orders_status_created_idx ON orders (status, created_at DESC);
CREATE INDEX orders_created_at_idx ON orders (created_at DESC);

CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    provider VARCHAR(32) NOT NULL DEFAULT 'midtrans',
    provider_order_id VARCHAR(64) NOT NULL,
    provider_transaction_id VARCHAR(128),
    payment_type VARCHAR(64),
    status VARCHAR(32) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'authorize', 'capture', 'settlement', 'deny', 'cancel', 'expire', 'failure', 'refund', 'partial_refund', 'chargeback')),
    amount BIGINT NOT NULL CHECK (amount > 0),
    currency CHAR(3) NOT NULL DEFAULT 'IDR',
    snap_token TEXT,
    redirect_url TEXT,
    provider_response JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_order_id)
);

CREATE UNIQUE INDEX payments_provider_transaction_uidx
    ON payments (provider, provider_transaction_id)
    WHERE provider_transaction_id IS NOT NULL;
CREATE INDEX payments_order_created_idx ON payments (order_id, created_at DESC);
CREATE INDEX payments_status_created_idx ON payments (status, created_at DESC);
CREATE INDEX payments_created_at_idx ON payments (created_at DESC);

CREATE TABLE payment_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE RESTRICT,
    attempt_number SMALLINT NOT NULL CHECK (attempt_number > 0),
    status VARCHAR(32) NOT NULL DEFAULT 'initiated',
    request_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (payment_id, attempt_number)
);

CREATE INDEX payment_attempts_payment_created_idx
    ON payment_attempts (payment_id, created_at DESC);

CREATE TABLE payment_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE RESTRICT,
    event_type VARCHAR(64) NOT NULL,
    previous_status VARCHAR(32),
    new_status VARCHAR(32),
    source VARCHAR(32) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX payment_events_payment_created_idx
    ON payment_events (payment_id, created_at DESC);
CREATE INDEX payment_events_created_at_idx ON payment_events (created_at DESC);

CREATE TABLE webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(32) NOT NULL DEFAULT 'midtrans',
    event_key VARCHAR(128) NOT NULL,
    provider_order_id VARCHAR(64),
    signature_valid BOOLEAN NOT NULL DEFAULT FALSE,
    processing_status VARCHAR(16) NOT NULL DEFAULT 'received'
        CHECK (processing_status IN ('received', 'processing', 'processed', 'failed')),
    payload JSONB NOT NULL,
    error_message TEXT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    UNIQUE (provider, event_key)
);

CREATE INDEX webhook_events_status_received_idx
    ON webhook_events (processing_status, received_at);
CREATE INDEX webhook_events_order_idx ON webhook_events (provider_order_id);

CREATE TABLE idempotency_keys (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    scope VARCHAR(128) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    request_hash CHAR(64) NOT NULL,
    response_status SMALLINT,
    response_body JSONB,
    resource_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    UNIQUE (scope, idempotency_key)
);

CREATE INDEX idempotency_keys_expires_at_idx ON idempotency_keys (expires_at);

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'processed', 'failed')),
    attempts SMALLINT NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_at TIMESTAMPTZ,
    processed_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX outbox_events_pending_idx
    ON outbox_events (available_at, created_at)
    WHERE status IN ('pending', 'failed');
CREATE INDEX outbox_events_aggregate_idx
    ON outbox_events (aggregate_type, aggregate_id, created_at DESC);
