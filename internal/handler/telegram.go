package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/bogdan/booking-service/internal/service"
)

type TelegramHandler struct {
	tg *service.TelegramService
}

func NewTelegramHandler(tg *service.TelegramService) *TelegramHandler {
	return &TelegramHandler{tg: tg}
}

// Status: GET /telegram/status.
func (h *TelegramHandler) Status(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUser(w, r)
	if !ok {
		return
	}
	status, err := h.tg.Status(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("статус Telegram", "err", err)
		respondError(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	respondJSON(w, http.StatusOK, status)
}

// CreateLink: POST /telegram/link — выдаёт одноразовый код привязки.
func (h *TelegramHandler) CreateLink(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUser(w, r)
	if !ok {
		return
	}
	info, err := h.tg.CreateLink(r.Context(), claims.UserID)
	switch {
	case errors.Is(err, service.ErrTelegramDisabled):
		respondError(w, http.StatusServiceUnavailable, "Telegram-бот не настроен")
	case err != nil:
		slog.Error("создание кода Telegram", "err", err)
		respondError(w, http.StatusInternalServerError, "внутренняя ошибка")
	default:
		respondJSON(w, http.StatusCreated, info)
	}
}
