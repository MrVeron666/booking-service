package repository

import (
	"context"
	"errors"
	"time"

	"github.com/bogdan/booking-service/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSlotTaken       = errors.New("время уже занято")
	ErrBookingNotFound = errors.New("запись не найдена")
)

type BookingRepository struct {
	pool *pgxpool.Pool
}

func NewBookingRepository(pool *pgxpool.Pool) *BookingRepository {
	return &BookingRepository{pool: pool}
}

const bookingColumns = `id, service_id, owner_id, client_id, start_at, end_at, status, created_at`

func scanBooking(row pgx.Row, b *model.Booking) error {
	return row.Scan(&b.ID, &b.ServiceID, &b.OwnerID, &b.ClientID, &b.StartAt, &b.EndAt, &b.Status, &b.CreatedAt)
}

// Create пытается вставить запись. Если время пересекается с другой активной записью
// владельца, PostgreSQL сам отклоняет вставку (EXCLUDE-ограничение, код 23P01).
func (r *BookingRepository) Create(ctx context.Context, b *model.Booking) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO bookings (service_id, owner_id, client_id, start_at, end_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, status, created_at`,
		b.ServiceID, b.OwnerID, b.ClientID, b.StartAt, b.EndAt,
	).Scan(&b.ID, &b.Status, &b.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		// 23P01 — нарушено ограничение EXCLUDE (пересечение по времени)
		if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
			return ErrSlotTaken
		}
		return err
	}
	return nil
}

// ListBusy возвращает занятые интервалы владельца, пересекающиеся с окном [from, to).
func (r *BookingRepository) ListBusy(ctx context.Context, ownerID int64, from, to time.Time) ([]model.Interval, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT start_at, end_at FROM bookings
		 WHERE owner_id = $1 AND status = 'booked' AND start_at < $3 AND end_at > $2`,
		ownerID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	busy := []model.Interval{}
	for rows.Next() {
		var iv model.Interval
		if err := rows.Scan(&iv.Start, &iv.End); err != nil {
			return nil, err
		}
		busy = append(busy, iv)
	}
	return busy, rows.Err()
}

// ListByUser: владелец видит записи на свои услуги, клиент — свои записи.
func (r *BookingRepository) ListByUser(ctx context.Context, userID int64, role string) ([]model.Booking, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+bookingColumns+` FROM bookings
		 WHERE ($2::text = 'owner' AND owner_id = $1)
		    OR ($2::text <> 'owner' AND client_id = $1)
		 ORDER BY start_at`, userID, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bookings := []model.Booking{}
	for rows.Next() {
		var b model.Booking
		if err := scanBooking(rows, &b); err != nil {
			return nil, err
		}
		bookings = append(bookings, b)
	}
	return bookings, rows.Err()
}

// Cancel отменяет активную запись, если она принадлежит пользователю
// (как клиенту или как владельцу услуги). Проверка прав — прямо в SQL.
func (r *BookingRepository) Cancel(ctx context.Context, id, userID int64) (*model.Booking, error) {
	b := &model.Booking{}
	err := scanBooking(r.pool.QueryRow(ctx,
		`UPDATE bookings SET status = 'cancelled'
		 WHERE id = $1 AND status = 'booked' AND (client_id = $2 OR owner_id = $2)
		 RETURNING `+bookingColumns, id, userID), b)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBookingNotFound
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}
