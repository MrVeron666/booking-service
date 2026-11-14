package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrLinkCodeInvalid = errors.New("код недействителен или просрочен")

// TgUser — пользователь, привязанный к Telegram-чату.
type TgUser struct {
	ID   int64
	Name string
	Role string
}

// Reminder — запись, о которой нужно напомнить клиенту.
type Reminder struct {
	BookingID   int64
	ChatID      int64
	ServiceName string
	StartAt     time.Time
}

type TelegramRepository struct {
	pool *pgxpool.Pool
}

func NewTelegramRepository(pool *pgxpool.Pool) *TelegramRepository {
	return &TelegramRepository{pool: pool}
}

// CreateLinkCode сохраняет одноразовый код привязки. Старые коды пользователя удаляются.
func (r *TelegramRepository) CreateLinkCode(ctx context.Context, userID int64, code string, ttl time.Duration) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now()
	if _, err := tx.Exec(ctx,
		`DELETE FROM telegram_link_codes WHERE user_id = $1 OR expires_at < $2`, userID, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO telegram_link_codes (code, user_id, expires_at) VALUES ($1, $2, $3)`,
		code, userID, now.Add(ttl)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// LinkByCode проверяет код и привязывает чат к пользователю.
// Код удаляется в том же запросе, поэтому использовать его второй раз нельзя.
func (r *TelegramRepository) LinkByCode(ctx context.Context, code string, chatID int64) (*TgUser, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID int64
	err = tx.QueryRow(ctx,
		`DELETE FROM telegram_link_codes WHERE code = $1 AND expires_at > $2 RETURNING user_id`,
		code, time.Now()).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLinkCodeInvalid
	}
	if err != nil {
		return nil, err
	}

	// Один чат — один аккаунт: отвязываем этот чат от других пользователей.
	if _, err := tx.Exec(ctx,
		`UPDATE users SET telegram_chat_id = NULL WHERE telegram_chat_id = $1 AND id <> $2`,
		chatID, userID); err != nil {
		return nil, err
	}

	u := &TgUser{ID: userID}
	err = tx.QueryRow(ctx,
		`UPDATE users SET telegram_chat_id = $1 WHERE id = $2 RETURNING name, role`,
		chatID, userID).Scan(&u.Name, &u.Role)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return u, nil
}

// UserByChat находит пользователя, привязанного к чату.
func (r *TelegramRepository) UserByChat(ctx context.Context, chatID int64) (*TgUser, error) {
	u := &TgUser{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, role FROM users WHERE telegram_chat_id = $1`, chatID,
	).Scan(&u.ID, &u.Name, &u.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// ChatByUser возвращает чат пользователя. ok=false, если Telegram не подключён.
func (r *TelegramRepository) ChatByUser(ctx context.Context, userID int64) (chatID int64, ok bool, err error) {
	var chat *int64 // указатель, потому что в базе может быть NULL
	err = r.pool.QueryRow(ctx,
		`SELECT telegram_chat_id FROM users WHERE id = $1`, userID).Scan(&chat)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if chat == nil {
		return 0, false, nil
	}
	return *chat, true, nil
}

// Unlink отключает уведомления для чата.
func (r *TelegramRepository) Unlink(ctx context.Context, chatID int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET telegram_chat_id = NULL WHERE telegram_chat_id = $1`, chatID)
	return err
}

// UserName возвращает имя пользователя по id.
func (r *TelegramRepository) UserName(ctx context.Context, userID int64) (string, error) {
	var name string
	err := r.pool.QueryRow(ctx, `SELECT name FROM users WHERE id = $1`, userID).Scan(&name)
	return name, err
}

// DueReminders возвращает активные записи, которые начнутся в окне (from, until],
// если клиент подключил Telegram и напоминание ещё не отправлялось.
func (r *TelegramRepository) DueReminders(ctx context.Context, from, until time.Time) ([]Reminder, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT b.id, u.telegram_chat_id, s.name, b.start_at
		 FROM bookings b
		 JOIN users u ON u.id = b.client_id
		 JOIN services s ON s.id = b.service_id
		 WHERE b.status = 'booked'
		   AND b.reminded_at IS NULL
		   AND u.telegram_chat_id IS NOT NULL
		   AND b.start_at > $1 AND b.start_at <= $2`, from, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Reminder
	for rows.Next() {
		var rem Reminder
		if err := rows.Scan(&rem.BookingID, &rem.ChatID, &rem.ServiceName, &rem.StartAt); err != nil {
			return nil, err
		}
		list = append(list, rem)
	}
	return list, rows.Err()
}

// MarkReminded запоминает, что напоминание отправлено.
func (r *TelegramRepository) MarkReminded(ctx context.Context, bookingID int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE bookings SET reminded_at = now() WHERE id = $1`, bookingID)
	return err
}
