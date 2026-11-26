CREATE TABLE order_items (
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    line_number INTEGER NOT NULL CHECK (line_number > 0),
    item_id VARCHAR(50) NOT NULL,
    name VARCHAR(50) NOT NULL,
    price BIGINT NOT NULL CHECK (price > 0),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (order_id, line_number)
);
ALTER TABLE orders ADD COLUMN customer_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;
