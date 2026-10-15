CREATE TABLE events(
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL
);
CREATE TABLE users(
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL
);
CREATE TABLE seats(
    id BIGSERIAL PRIMARY KEY,
    event_id BIGINT NOT NULL,
    status TEXT NOT NULL,
    price_paise BIGINT NOT NULL DEFAULT 200000,
    seat_number BIGINT NOT NULL
);
CREATE TABLE reservations(
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    event_id BIGINT NOT NULL,
    seat_id BIGINT NOT NULL,
    status TEXT NOT NULL
);