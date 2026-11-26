CREATE TABLE refunds (
    refund_key VARCHAR(64) PRIMARY KEY,
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE RESTRICT,
    amount BIGINT NOT NULL CHECK (amount > 0),
    reason VARCHAR(255) NOT NULL,
    status VARCHAR(24) NOT NULL CHECK (status IN ('uncertain','confirmed')),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX refunds_payment_idx ON refunds(payment_id, created_at);
CREATE UNIQUE INDEX refunds_unresolved_idx ON refunds(payment_id) WHERE status='uncertain';
