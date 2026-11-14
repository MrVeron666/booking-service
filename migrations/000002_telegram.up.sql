-- Привязка аккаунта к Telegram-чату (один чат = один аккаунт).
ALTER TABLE users ADD COLUMN telegram_chat_id BIGINT UNIQUE;

-- Когда клиенту отправили напоминание (NULL = ещё не отправляли).
ALTER TABLE bookings ADD COLUMN reminded_at TIMESTAMPTZ;

-- Частичный индекс: ускоряет поиск записей, о которых ещё нужно напомнить.
CREATE INDEX idx_bookings_reminders ON bookings(start_at)
    WHERE status = 'booked' AND reminded_at IS NULL;

-- Одноразовые коды для привязки Telegram.
CREATE TABLE telegram_link_codes (
    code       TEXT        PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);