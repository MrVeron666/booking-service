package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/bogdan/booking-service/internal/middleware"
	"github.com/bogdan/booking-service/internal/repository"
	"github.com/bogdan/booking-service/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// Register обрабатывает POST /auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	user, err := h.auth.Register(r.Context(), req.Name, req.Email, req.Password, req.Role)
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		respondError(w, http.StatusBadRequest, "проверьте имя, email, пароль (минимум 8 символов) и роль (owner/client)")
	case errors.Is(err, repository.ErrEmailTaken):
		respondError(w, http.StatusConflict, "email уже зарегистрирован")
	case err != nil:
		slog.Error("ошибка регистрации", "err", err)
		respondError(w, http.StatusInternalServerError, "внутренняя ошибка")
	default:
		respondJSON(w, http.StatusCreated, user)
	}
}

// Login обрабатывает POST /auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	token, err := h.auth.Login(r.Context(), req.Email, req.Password)
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		respondError(w, http.StatusUnauthorized, "неверный email или пароль")
	case err != nil:
		slog.Error("ошибка входа", "err", err)
		respondError(w, http.StatusInternalServerError, "внутренняя ошибка")
	default:
		respondJSON(w, http.StatusOK, map[string]string{"token": token})
	}
}

// Me обрабатывает GET /me — показывает, кто сейчас авторизован.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "нет данных авторизации")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"user_id": claims.UserID, "role": claims.Role})
}
