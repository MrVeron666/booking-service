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
	"github.com/bogdan/booking-service/internal/middleware"
	"github.com/bogdan/booking-service/internal/repository"
	"github.com/bogdan/booking-service/internal/service"
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

	// Секрет для подписи JWT-токенов берём из переменной окружения.
	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 16 {
		slog.Error("JWT_SECRET не задан или слишком короткий (минимум 16 символов)")
		os.Exit(1)
	}

	// Собираем слои снизу вверх: repository -> service -> handler.
	userRepo := repository.NewUserRepository(pool)
	authService := service.NewAuthService(userRepo, jwtSecret)
	authHandler := handler.NewAuthHandler(authService)

	router := chi.NewRouter()
	router.Get("/health", handler.HealthHandler{Pool: pool}.ServeHTTP)

	// Открытые маршруты: токен не нужен.
	router.Post("/auth/register", authHandler.Register)
	router.Post("/auth/login", authHandler.Login)

	// Маршруты внутри Group доступны только с токеном.
	router.Group(func(r chi.Router) {
		r.Use(middleware.Auth(authService))
		r.Get("/me", authHandler.Me)
	})

	slog.Info("сервер запускается", "addr", cfg.HTTPAddr)
	err = http.ListenAndServe(cfg.HTTPAddr, router)
	if err != nil {
		slog.Error("сервер остановился с ошибкой", "err", err)
		os.Exit(1)
	}
}
