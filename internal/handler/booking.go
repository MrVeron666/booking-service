package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bogdan/booking-service/internal/repository"
	"github.com/bogdan/booking-service/internal/service"
)

type BookingHandler struct {
	bookings *service.BookingService
}

func NewBookingHandler(bookings *service.BookingService) *BookingHandler {
	return &BookingHandler{bookings: bookings}
}

func respondBookingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		respondError(w, http.StatusBadRequest, "некорректные данные (дата в формате ГГГГ-ММ-ДД)")
	case errors.Is(err, service.ErrBookingTime):
		respondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, repository.ErrSlotTaken):
		respondError(w, http.StatusConflict, "это время уже занято")
	case errors.Is(err, repository.ErrServiceNotFound):
		respondError(w, http.StatusNotFound, "услуга не найдена")
	case errors.Is(err, repository.ErrBookingNotFound):
		respondError(w, http.StatusNotFound, "запись не найдена")
	default:
		slog.Error("ошибка записей", "err", err)
		respondError(w, http.StatusInternalServerError, "внутренняя ошибка")
	}
}

// ListSlots: GET /services/{id}/slots?date=2026-11-16.
func (h *BookingHandler) ListSlots(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		respondError(w, http.StatusBadRequest, "некорректный id")
		return
	}
	slots, err := h.bookings.GetSlots(r.Context(), id, r.URL.Query().Get("date"))
	if err != nil {
		respondBookingError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, slots)
}

// Create: POST /bookings (только client).
func (h *BookingHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUser(w, r)
	if !ok {
		return
	}
	var req struct {
		ServiceID int64     `json:"service_id"`
		StartAt   time.Time `json:"start_at"` // формат RFC3339: 2026-11-16T10:00:00+05:00
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ServiceID <= 0 || req.StartAt.IsZero() {
		respondError(w, http.StatusBadRequest, "нужны service_id и start_at (например 2026-11-16T10:00:00+05:00)")
		return
	}
	booking, err := h.bookings.CreateBooking(r.Context(), claims.UserID, req.ServiceID, req.StartAt)
	if err != nil {
		respondBookingError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, booking)
}

// List: GET /bookings — свои записи (клиент) или записи на свои услуги (владелец).
func (h *BookingHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUser(w, r)
	if !ok {
		return
	}
	bookings, err := h.bookings.ListBookings(r.Context(), claims.UserID, claims.Role)
	if err != nil {
		respondBookingError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, bookings)
}

// Cancel: PATCH /bookings/{id}/cancel.
func (h *BookingHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUser(w, r)
	if !ok {
		return
	}
	id, ok := parseID(r)
	if !ok {
		respondError(w, http.StatusBadRequest, "некорректный id")
		return
	}
	booking, err := h.bookings.CancelBooking(r.Context(), id, claims.UserID)
	if err != nil {
		respondBookingError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, booking)
}
