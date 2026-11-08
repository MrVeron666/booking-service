package service

import (
	"context"
	"errors"
	"time"

	"github.com/bogdan/booking-service/internal/model"
	"github.com/bogdan/booking-service/internal/repository"
)

var ErrBookingTime = errors.New("время вне рабочих часов, в прошлом или не совпадает с сеткой слотов")

// BookingService — слоты и записи.
type BookingService struct {
	services *repository.ServiceRepository
	hours    *repository.WorkingHoursRepository
	bookings *repository.BookingRepository
	loc      *time.Location   // часовой пояс бизнеса
	now      func() time.Time // "текущее время" как поле, чтобы подменять его в тестах
}

func NewBookingService(
	services *repository.ServiceRepository,
	hours *repository.WorkingHoursRepository,
	bookings *repository.BookingRepository,
	loc *time.Location,
) *BookingService {
	return &BookingService{services: services, hours: hours, bookings: bookings, loc: loc, now: time.Now}
}

// clockOnDay склеивает дату из day и время "09:00" в часовом поясе loc.
func clockOnDay(day time.Time, clock string, loc *time.Location) (time.Time, error) {
	t, err := time.Parse("15:04", clock)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), 0, 0, loc), nil
}

// workingWindow возвращает рабочее окно владельца в день day. ok=false, если в этот день выходной.
func (s *BookingService) workingWindow(ctx context.Context, ownerID int64, day time.Time) (start, end time.Time, ok bool, err error) {
	hours, err := s.hours.Get(ctx, ownerID)
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	for _, h := range hours {
		if h.Weekday != int(day.Weekday()) {
			continue
		}
		start, err = clockOnDay(day, h.StartTime, s.loc)
		if err != nil {
			return time.Time{}, time.Time{}, false, err
		}
		end, err = clockOnDay(day, h.EndTime, s.loc)
		if err != nil {
			return time.Time{}, time.Time{}, false, err
		}
		return start, end, true, nil
	}
	return time.Time{}, time.Time{}, false, nil
}

// GetSlots возвращает свободные слоты услуги на дату вида "2026-11-16".
func (s *BookingService) GetSlots(ctx context.Context, serviceID int64, date string) ([]model.Slot, error) {
	day, err := time.ParseInLocation("2006-01-02", date, s.loc)
	if err != nil {
		return nil, ErrInvalidInput
	}
	svc, err := s.services.GetByID(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	winStart, winEnd, ok, err := s.workingWindow(ctx, svc.OwnerID, day)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []model.Slot{}, nil // в этот день владелец не работает
	}
	busy, err := s.bookings.ListBusy(ctx, svc.OwnerID, winStart, winEnd)
	if err != nil {
		return nil, err
	}
	dur := time.Duration(svc.DurationMinutes) * time.Minute
	return buildSlots(winStart, winEnd, dur, busy, s.now()), nil
}

// CreateBooking проверяет время и создаёт запись. Занято ли оно, решает база данных.
func (s *BookingService) CreateBooking(ctx context.Context, clientID, serviceID int64, startAt time.Time) (*model.Booking, error) {
	svc, err := s.services.GetByID(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	startAt = startAt.In(s.loc)
	dur := time.Duration(svc.DurationMinutes) * time.Minute
	endAt := startAt.Add(dur)

	if !startAt.After(s.now()) {
		return nil, ErrBookingTime // нельзя записаться в прошлое
	}

	winStart, winEnd, ok, err := s.workingWindow(ctx, svc.OwnerID, startAt)
	if err != nil {
		return nil, err
	}
	// Запись должна целиком лежать в рабочем окне и начинаться ровно на границе слота.
	if !ok || startAt.Before(winStart) || endAt.After(winEnd) || startAt.Sub(winStart)%dur != 0 {
		return nil, ErrBookingTime
	}

	b := &model.Booking{
		ServiceID: svc.ID,
		OwnerID:   svc.OwnerID,
		ClientID:  clientID,
		StartAt:   startAt,
		EndAt:     endAt,
	}
	if err := s.bookings.Create(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *BookingService) ListBookings(ctx context.Context, userID int64, role string) ([]model.Booking, error) {
	return s.bookings.ListByUser(ctx, userID, role)
}

func (s *BookingService) CancelBooking(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	return s.bookings.Cancel(ctx, bookingID, userID)
}
