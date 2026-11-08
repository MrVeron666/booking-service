-- Расширение нужно, чтобы запретить пересечение записей (см. bookings)
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT        NOT NULL,
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    role          TEXT        NOT NULL CHECK (role IN ('owner', 'client')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE services (
    id               BIGSERIAL PRIMARY KEY,
    owner_id         BIGINT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name             TEXT          NOT NULL,
    duration_minutes INT           NOT NULL CHECK (duration_minutes > 0),
    price            NUMERIC(10,2) NOT NULL CHECK (price >= 0),
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE TABLE working_hours (
    id         BIGSERIAL PRIMARY KEY,
    owner_id   BIGINT   NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    weekday    SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_time TIME     NOT NULL,
    end_time   TIME     NOT NULL,
    CHECK (end_time > start_time),
    UNIQUE (owner_id, weekday)
);

CREATE TABLE bookings (
    id         BIGSERIAL PRIMARY KEY,
    service_id BIGINT      NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    owner_id   BIGINT      NOT NULL REFERENCES users(id),
    client_id  BIGINT      NOT NULL REFERENCES users(id),
    start_at   TIMESTAMPTZ NOT NULL,
    end_at     TIMESTAMPTZ NOT NULL,
    status     TEXT        NOT NULL DEFAULT 'booked'
               CHECK (status IN ('booked', 'cancelled', 'completed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (end_at > start_at),
    -- У одного владельца две активные записи не могут пересекаться по времени
    EXCLUDE USING gist (
        owner_id WITH =,
        tstzrange(start_at, end_at) WITH &&
    ) WHERE (status = 'booked')
);

CREATE INDEX idx_services_owner ON services(owner_id);
CREATE INDEX idx_bookings_client ON bookings(client_id, start_at);
CREATE INDEX idx_bookings_owner_start ON bookings(owner_id, start_at);