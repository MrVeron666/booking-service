package service

import (
	"context"
	"strings"
	"time"

	"github.com/bogdan/booking-service/internal/model"
	"github.com/bogdan/booking-service/internal/repository"
)

// CatalogService — бизнес-логика услуг и рабочих часов.
type CatalogService struct {
	services *repository.ServiceRepository
	hours    *repository.WorkingHoursRepository
}

func NewCatalogService(services *repository.ServiceRepository, hours *repository.WorkingHoursRepository) *CatalogService {
	return &CatalogService{services: services, hours: hours}
}

func validateService(name string, duration int, price float64) error {
	if name == "" || duration < 5 || duration > 480 || price < 0 {
		return ErrInvalidInput
	}
	return nil
}

func (s *CatalogService) CreateService(ctx context.Context, ownerID int64, name string, duration int, price float64) (*model.Service, error) {
	name = strings.TrimSpace(name)
	if err := validateService(name, duration, price); err != nil {
		return nil, err
	}
	svc := &model.Service{OwnerID: ownerID, Name: name, DurationMinutes: duration, Price: price}
	if err := s.services.Create(ctx, svc); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *CatalogService) ListServices(ctx context.Context, ownerID int64) ([]model.Service, error) {
	return s.services.List(ctx, ownerID)
}

func (s *CatalogService) GetService(ctx context.Context, id int64) (*model.Service, error) {
	return s.services.GetByID(ctx, id)
}

func (s *CatalogService) UpdateService(ctx context.Context, ownerID, id int64, name string, duration int, price float64) (*model.Service, error) {
	name = strings.TrimSpace(name)
	if err := validateService(name, duration, price); err != nil {
		return nil, err
	}
	svc := &model.Service{ID: id, OwnerID: ownerID, Name: name, DurationMinutes: duration, Price: price}
	if err := s.services.Update(ctx, svc); err != nil {
		return nil, err
	}
	return s.services.GetByID(ctx, id)
}

func (s *CatalogService) DeleteService(ctx context.Context, ownerID, id int64) error {
	return s.services.Delete(ctx, id, ownerID)
}

func (s *CatalogService) GetWorkingHours(ctx context.Context, ownerID int64) ([]model.WorkingHour, error) {
	return s.hours.Get(ctx, ownerID)
}

// SetWorkingHours проверяет график и сохраняет его вместо старого.
func (s *CatalogService) SetWorkingHours(ctx context.Context, ownerID int64, hours []model.WorkingHour) error {
	// seen — та же идея, что в задаче hasDuplicate: один день недели нельзя указать дважды.
	seen := make(map[int]bool)
	for _, h := range hours {
		if h.Weekday < 0 || h.Weekday > 6 || seen[h.Weekday] {
			return ErrInvalidInput
		}
		seen[h.Weekday] = true

		start, errStart := time.Parse("15:04", h.StartTime)
		end, errEnd := time.Parse("15:04", h.EndTime)
		if errStart != nil || errEnd != nil || !end.After(start) {
			return ErrInvalidInput
		}
	}
	return s.hours.Replace(ctx, ownerID, hours)
}
