// Точка входа HTTP-сервиса онлайн-записи.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/bogdan/booking-service/internal/config"
	"github.com/bogdan/booking-service/internal/handler"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("не удалось загрузить конфиг", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("не удалось создать пул подключений к БД", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	err = pool.Ping(ctx)
	if err != nil {
		slog.Error("не удалось подключиться к БД", "err", err)
		os.Exit(1)
	}
	slog.Info("подключение к PostgreSQL установлено")

	router := chi.NewRouter()
	router.Get("/health", handler.HealthHandler{Pool: pool}.ServeHTTP)

	slog.Info("сервер запускается", "addr", cfg.HTTPAddr)
	err = http.ListenAndServe(cfg.HTTPAddr, router)
	if err != nil {
		slog.Error("сервер остановился с ошибкой", "err", err)
		os.Exit(1)
	}
}
