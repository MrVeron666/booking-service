package repository

import (
	"context"
	"errors"

	"github.com/bogdan/booking-service/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrServiceNotFound = errors.New("услуга не найдена")

type ServiceRepository struct {
	pool *pgxpool.Pool
}

func NewServiceRepository(pool *pgxpool.Pool) *ServiceRepository {
	return &ServiceRepository{pool: pool}
}

func (r *ServiceRepository) Create(ctx context.Context, s *model.Service) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO services (owner_id, name, duration_minutes, price)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		s.OwnerID, s.Name, s.DurationMinutes, s.Price,
	).Scan(&s.ID, &s.CreatedAt)
}

// List возвращает услуги. Если ownerID = 0, возвращает услуги всех владельцев.
func (r *ServiceRepository) List(ctx context.Context, ownerID int64) ([]model.Service, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, owner_id, name, duration_minutes, price, created_at
		 FROM services
		 WHERE ($1::bigint = 0 OR owner_id = $1)
		 ORDER BY id`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	services := []model.Service{} // пустой слайс, чтобы в JSON было [] а не null
	for rows.Next() {
		var s model.Service
		if err := rows.Scan(&s.ID, &s.OwnerID, &s.Name, &s.DurationMinutes, &s.Price, &s.CreatedAt); err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	return services, rows.Err()
}

func (r *ServiceRepository) GetByID(ctx context.Context, id int64) (*model.Service, error) {
	s := &model.Service{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, owner_id, name, duration_minutes, price, created_at
		 FROM services WHERE id = $1`, id,
	).Scan(&s.ID, &s.OwnerID, &s.Name, &s.DurationMinutes, &s.Price, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrServiceNotFound
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Update меняет услугу, только если она принадлежит s.OwnerID.
// Проверка владельца прямо в SQL: чужую услугу изменить нельзя.
func (r *ServiceRepository) Update(ctx context.Context, s *model.Service) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE services SET name = $1, duration_minutes = $2, price = $3
		 WHERE id = $4 AND owner_id = $5`,
		s.Name, s.DurationMinutes, s.Price, s.ID, s.OwnerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrServiceNotFound
	}
	return nil
}

func (r *ServiceRepository) Delete(ctx context.Context, id, ownerID int64) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM services WHERE id = $1 AND owner_id = $2`, id, ownerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrServiceNotFound
	}
	return nil
}
