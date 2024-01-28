CREATE TABLE IF NOT EXISTS addresses (
    id SERIAL PRIMARY KEY,
    zipcode VARCHAR(20),
    city VARCHAR(20),
    address VARCHAR(50),
    region VARCHAR(20),
    CONSTRAINT unique_address UNIQUE (address, zipcode, city, region)
);

CREATE TABLE IF NOT EXISTS users (
    phonenumber VARCHAR(13) UNIQUE PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    email VARCHAR(50) UNIQUE,
    address_id INTEGER,
    CONSTRAINT fk_address FOREIGN KEY(address_id) REFERENCES addresses(id)
);

CREATE TABLE IF NOT EXISTS payments (
    transaction_id VARCHAR(50) PRIMARY KEY,
    request_id VARCHAR(50) UNIQUE NOT NULL DEFAULT '',
    currency VARCHAR(10) NOT NULL,
    provider_id VARCHAR(20) NOT NULL,
    amount DOUBLE PRECISION NOT NULL,
    payment_dt TIMESTAMPTZ NOT NULL,
    bank VARCHAR(20) NOT NULL,
    delivery_cost DOUBLE PRECISION NOT NULL,
    goods_total DOUBLE PRECISION NOT NULL,
    custom_fee DOUBLE PRECISION NOT NULL
);

CREATE TABLE IF NOT EXISTS orders (
    order_id VARCHAR(50) PRIMARY KEY NOT NULL,
    track_number VARCHAR(50) UNIQUE,
    entry VARCHAR(10),
    delivery_user VARCHAR(13) NOT NULL,
    CONSTRAINT fk_delivery_user FOREIGN KEY(delivery_user) REFERENCES users(phonenumber),
    transaction_id VARCHAR(50) UNIQUE,
    CONSTRAINT fk_transaction_id_orders FOREIGN KEY(transaction_id) REFERENCES payments(transaction_id),
    locale VARCHAR(5) NOT NULL,
    internal_signature VARCHAR(5) NOT NULL DEFAULT '',
    customer_id VARCHAR(20) NOT NULL,
    delivery_service VARCHAR(20) NOT NULL,
    shardkey VARCHAR(20) NOT NULL,
    sm_id INTEGER NOT NULL,
    oof_shard VARCHAR(20) NOT NULL,
    date_created TIMESTAMPTZ NOT NULL
);

-- DO $$
-- BEGIN
--     IF NOT EXISTS(
--         SELECT constraint_name
--         FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS
--         WHERE table_name = 'payments'
--         AND constraint_name = 'fk_order_id_payments'
--     ) THEN
--         ALTER TABLE payments ADD CONSTRAINT fk_order_id_payments FOREIGN KEY(order_id) REFERENCES orders(order_id);
--     END IF;
--     RETURN;
-- END
-- $$;

CREATE TABLE IF NOT EXISTS items (
    chrt_id BIGINT PRIMARY KEY NOT NULL,
    track_number VARCHAR(50) NOT NULL,
    price DOUBLE PRECISION NOT NULL,
    rid VARCHAR(50) NOT NULL,
    name VARCHAR(50) NOT NULL,
    sale INTEGER DEFAULT 0 NOT NULL,
    size VARCHAR(10) NOT NULL DEFAULT '0',
    total_price DOUBLE PRECISION NOT NULL,
    nm_id INTEGER NOT NULL,
    brand VARCHAR(20) NOT NULL,
    order_id VARCHAR(50) NOT NULL,
    CONSTRAINT fk_order_id_items FOREIGN KEY(order_id) REFERENCES orders(order_id)
);

CREATE OR REPLACE FUNCTION add_address(add VARCHAR(50), zip VARCHAR(50), c VARCHAR(50), r VARCHAR(50)) RETURNS INTEGER AS $$
DECLARE founded_id INTEGER;
BEGIN
SELECT id FROM addresses WHERE address = add and zipcode = zip and city = c and region = r INTO founded_id;
IF NOT FOUND THEN
    INSERT INTO addresses(zipcode, city, address, region) VALUES(zip, c, add, r) RETURNING id INTO founded_id;
END IF;
RETURN founded_id;
END;
$$ LANGUAGE plpgsql;