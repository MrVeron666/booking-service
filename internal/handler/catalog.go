package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bogdan/booking-service/internal/middleware"
	"github.com/bogdan/booking-service/internal/model"
	"github.com/bogdan/booking-service/internal/repository"
	"github.com/bogdan/booking-service/internal/service"
	"github.com/go-chi/chi/v5"
)

type CatalogHandler struct {
	catalog *service.CatalogService
}

func NewCatalogHandler(catalog *service.CatalogService) *CatalogHandler {
	return &CatalogHandler{catalog: catalog}
}

type serviceRequest struct {
	Name            string  `json:"name"`
	DurationMinutes int     `json:"duration_minutes"`
	Price           float64 `json:"price"`
}

// parseID достаёт число из адреса вида /services/{id}.
func parseID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id, err == nil && id > 0
}

// currentUser берёт данные пользователя, которые положил middleware.Auth.
func currentUser(w http.ResponseWriter, r *http.Request) (*service.Claims, bool) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "нет данных авторизации")
	}
	return claims, ok
}

func respondCatalogError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		respondError(w, http.StatusBadRequest, "некорректные данные")
	case errors.Is(err, repository.ErrServiceNotFound):
		respondError(w, http.StatusNotFound, "услуга не найдена")
	default:
		slog.Error("ошибка каталога", "err", err)
		respondError(w, http.StatusInternalServerError, "внутренняя ошибка")
	}
}

// ListServices: GET /services?owner_id=1 (owner_id необязателен).
func (h *CatalogHandler) ListServices(w http.ResponseWriter, r *http.Request) {
	var ownerID int64
	if v := r.URL.Query().Get("owner_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "owner_id должен быть числом")
			return
		}
		ownerID = id
	}
	services, err := h.catalog.ListServices(r.Context(), ownerID)
	if err != nil {
		respondCatalogError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, services)
}

// GetService: GET /services/{id}.
func (h *CatalogHandler) GetService(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		respondError(w, http.StatusBadRequest, "некорректный id")
		return
	}
	svc, err := h.catalog.GetService(r.Context(), id)
	if err != nil {
		respondCatalogError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, svc)
}

// CreateService: POST /services (только owner).
func (h *CatalogHandler) CreateService(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUser(w, r)
	if !ok {
		return
	}
	var req serviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	svc, err := h.catalog.CreateService(r.Context(), claims.UserID, req.Name, req.DurationMinutes, req.Price)
	if err != nil {
		respondCatalogError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, svc)
}

// UpdateService: PUT /services/{id} (только owner своей услуги).
func (h *CatalogHandler) UpdateService(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUser(w, r)
	if !ok {
		return
	}
	id, ok := parseID(r)
	if !ok {
		respondError(w, http.StatusBadRequest, "некорректный id")
		return
	}
	var req serviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	svc, err := h.catalog.UpdateService(r.Context(), claims.UserID, id, req.Name, req.DurationMinutes, req.Price)
	if err != nil {
		respondCatalogError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, svc)
}

// DeleteService: DELETE /services/{id} (только owner своей услуги).
func (h *CatalogHandler) DeleteService(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUser(w, r)
	if !ok {
		return
	}
	id, ok := parseID(r)
	if !ok {
		respondError(w, http.StatusBadRequest, "некорректный id")
		return
	}
	if err := h.catalog.DeleteService(r.Context(), claims.UserID, id); err != nil {
		respondCatalogError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent) // 204: успех, тела ответа нет
}

// GetWorkingHours: GET /working-hours (график текущего owner).
func (h *CatalogHandler) GetWorkingHours(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUser(w, r)
	if !ok {
		return
	}
	hours, err := h.catalog.GetWorkingHours(r.Context(), claims.UserID)
	if err != nil {
		respondCatalogError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, hours)
}

// SetWorkingHours: PUT /working-hours — заменяет весь график.
func (h *CatalogHandler) SetWorkingHours(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Hours []model.WorkingHour `json:"hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if err := h.catalog.SetWorkingHours(r.Context(), claims.UserID, req.Hours); err != nil {
		respondCatalogError(w, err)
		return
	}
	hours, err := h.catalog.GetWorkingHours(r.Context(), claims.UserID)
	if err != nil {
		respondCatalogError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, hours)
}
