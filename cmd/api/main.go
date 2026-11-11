// Точка входа HTTP-сервиса онлайн-записи.
package main

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"
	_ "time/tzdata" // встроенная база часовых поясов (в образе Alpine её нет)

	"github.com/bogdan/booking-service/internal/config"
	"github.com/bogdan/booking-service/internal/handler"
	"github.com/bogdan/booking-service/internal/middleware"
	"github.com/bogdan/booking-service/internal/repository"
	"github.com/bogdan/booking-service/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Файлы фронтенда вшиваются в программу при сборке.
//
//go:embed static
var staticFiles embed.FS

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

	// Часовой пояс бизнеса: рабочие часы задаются в местном времени.
	tz := os.Getenv("TIMEZONE")
	if tz == "" {
		tz = "Asia/Almaty"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		slog.Error("неизвестный часовой пояс", "tz", tz, "err", err)
		os.Exit(1)
	}

	// Собираем слои снизу вверх: repository -> service -> handler.
	userRepo := repository.NewUserRepository(pool)
	authService := service.NewAuthService(userRepo, jwtSecret)
	authHandler := handler.NewAuthHandler(authService)

	serviceRepo := repository.NewServiceRepository(pool)
	hoursRepo := repository.NewWorkingHoursRepository(pool)
	catalogService := service.NewCatalogService(serviceRepo, hoursRepo)
	catalogHandler := handler.NewCatalogHandler(catalogService)

	bookingRepo := repository.NewBookingRepository(pool)
	bookingService := service.NewBookingService(serviceRepo, hoursRepo, bookingRepo, loc)
	bookingHandler := handler.NewBookingHandler(bookingService)

	router := chi.NewRouter()
	router.Get("/health", handler.HealthHandler{Pool: pool}.ServeHTTP)

	// Открытые маршруты: токен не нужен.
	router.Post("/auth/register", authHandler.Register)
	router.Post("/auth/login", authHandler.Login)

	// Маршруты внутри Group доступны только с токеном.
	router.Group(func(r chi.Router) {
		r.Use(middleware.Auth(authService))
		r.Get("/me", authHandler.Me)

		// Просмотр услуг, слотов и своих записей доступен любой роли.
		r.Get("/services", catalogHandler.ListServices)
		r.Get("/services/{id}", catalogHandler.GetService)
		r.Get("/services/{id}/slots", bookingHandler.ListSlots)
		r.Get("/bookings", bookingHandler.List)
		r.Patch("/bookings/{id}/cancel", bookingHandler.Cancel)

		// Создавать записи могут только клиенты.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("client"))
			r.Post("/bookings", bookingHandler.Create)
		})

		// Изменения каталога и графика доступны только владельцу (owner).
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("owner"))
			r.Post("/services", catalogHandler.CreateService)
			r.Put("/services/{id}", catalogHandler.UpdateService)
			r.Delete("/services/{id}", catalogHandler.DeleteService)
			r.Get("/working-hours", catalogHandler.GetWorkingHours)
			r.Put("/working-hours", catalogHandler.SetWorkingHours)
		})
	})

	// Всё остальное — страница фронтенда (главная страница index.html).
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		slog.Error("не удалось открыть встроенные файлы", "err", err)
		os.Exit(1)
	}
	router.Handle("/*", http.FileServer(http.FS(staticFS)))

	slog.Info("сервер запускается", "addr", cfg.HTTPAddr)
	err = http.ListenAndServe(cfg.HTTPAddr, router)
	if err != nil {
		slog.Error("сервер остановился с ошибкой", "err", err)
		os.Exit(1)
	}
}
