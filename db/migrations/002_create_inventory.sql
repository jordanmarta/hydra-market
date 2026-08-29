CREATE TABLE inventory (
    product_id BIGINT PRIMARY KEY REFERENCES products(id),
    quantity INTEGER NOT NULL CHECK (quantity >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);