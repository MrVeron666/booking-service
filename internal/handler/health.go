// Пакет handler содержит HTTP-обработчики (то, что вызывается при запросе).
package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthHandler отвечает на GET /health и проверяет, жива ли база данных.
type HealthHandler struct {
	Pool *pgxpool.Pool
}

func (h HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	err := h.Pool.Ping(ctx)
	if err != nil {
		slog.Error("база данных недоступна", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, writeErr := w.Write([]byte(`{"status":"error","message":"database unavailable"}`))
		if writeErr != nil {
			slog.Error("не удалось записать ответ", "err", writeErr)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, writeErr := w.Write([]byte(`{"status":"ok"}`))
	if writeErr != nil {
		slog.Error("не удалось записать ответ", "err", writeErr)
	}
}
