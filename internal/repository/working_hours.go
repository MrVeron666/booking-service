package repository

import (
	"context"

	"github.com/bogdan/booking-service/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkingHoursRepository struct {
	pool *pgxpool.Pool
}

func NewWorkingHoursRepository(pool *pgxpool.Pool) *WorkingHoursRepository {
	return &WorkingHoursRepository{pool: pool}
}

// Get возвращает график владельца. to_char превращает TIME в строку "09:00".
func (r *WorkingHoursRepository) Get(ctx context.Context, ownerID int64) ([]model.WorkingHour, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT weekday, to_char(start_time, 'HH24:MI'), to_char(end_time, 'HH24:MI')
		 FROM working_hours WHERE owner_id = $1 ORDER BY weekday`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hours := []model.WorkingHour{}
	for rows.Next() {
		var h model.WorkingHour
		if err := rows.Scan(&h.Weekday, &h.StartTime, &h.EndTime); err != nil {
			return nil, err
		}
		hours = append(hours, h)
	}
	return hours, rows.Err()
}

// Replace заменяет весь график владельца в одной транзакции:
// либо удалится старое и запишется новое, либо не изменится ничего.
func (r *WorkingHoursRepository) Replace(ctx context.Context, ownerID int64, hours []model.WorkingHour) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Если до Commit что-то упадёт, Rollback откатит изменения.
	// После успешного Commit Rollback ничего не делает, его ошибку можно не проверять.
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM working_hours WHERE owner_id = $1`, ownerID); err != nil {
		return err
	}
	for _, h := range hours {
		_, err := tx.Exec(ctx,
			`INSERT INTO working_hours (owner_id, weekday, start_time, end_time)
			 VALUES ($1, $2, $3::text::time, $4::text::time)`,
			ownerID, h.Weekday, h.StartTime, h.EndTime)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
