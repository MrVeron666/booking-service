DROP TABLE IF EXISTS telegram_link_codes;
DROP INDEX IF EXISTS idx_bookings_reminders;
ALTER TABLE bookings DROP COLUMN IF EXISTS reminded_at;
ALTER TABLE users DROP COLUMN IF EXISTS telegram_chat_id;