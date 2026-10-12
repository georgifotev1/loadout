-- +goose Up
-- Delivery to the courier's offices and parcel lockers (APTs), besides an address.

-- The courier's offices and lockers, copied from its API once a day so checkout never waits on
-- the courier. One courier at a time: the sync replaces the lot. Orders keep a copy of the one
-- they chose, so an office that closes doesn't change them.
CREATE TABLE orders.offices (
    id        text PRIMARY KEY,  -- the courier's id
    kind      text NOT NULL CHECK (kind IN ('office', 'apt')),
    name      text NOT NULL,
    city      text NOT NULL,
    postcode  text NOT NULL,
    address   text NOT NULL,     -- street and number, without the city
    hours     text NOT NULL,     -- as people read it; '' if the courier doesn't say
    -- Name, city, postcode and address in lower case, for checkout's search. Lowered by the sync
    -- in Go, so it doesn't depend on the database's locale for Cyrillic.
    search    text NOT NULL,
    synced_at timestamptz NOT NULL
);

ALTER TABLE orders.orders
    -- Where the parcel goes. For an office or a locker, city, postcode and address are the
    -- office's, copied at checkout.
    ADD COLUMN delivery text NOT NULL DEFAULT 'address' CHECK (delivery IN ('address', 'office', 'apt')),
    ADD COLUMN office_id text,
    ADD COLUMN office_name text,
    ADD CONSTRAINT orders_office_check CHECK (
        (delivery = 'address') = (office_id IS NULL AND office_name IS NULL)
    );
-- Existing orders went to an address; new ones say where they go.
ALTER TABLE orders.orders ALTER COLUMN delivery DROP DEFAULT;

-- +goose Down
-- Orders to an office would read as delivery to the office's address, so the down migration
-- refuses them.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM orders.orders WHERE delivery <> 'address') THEN
        RAISE EXCEPTION 'orders to an office or a locker exist';
    END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE orders.orders
    DROP CONSTRAINT orders_office_check,
    DROP COLUMN office_name,
    DROP COLUMN office_id,
    DROP COLUMN delivery;
DROP TABLE orders.offices;
