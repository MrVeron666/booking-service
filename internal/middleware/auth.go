package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bogdan/booking-service/internal/service"
)

type ctxKey string

const claimsKey ctxKey = "claims"

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		slog.Error("не удалось записать ответ", "err", err)
	}
}

// Auth требует заголовок "Authorization: Bearer <токен>".
// Если токен верный — кладёт данные пользователя в контекст запроса.
func Auth(auth *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "нужен токен авторизации")
				return
			}
			claims, err := auth.ParseToken(strings.TrimPrefix(header, "Bearer "))
			if err != nil {
				writeError(w, http.StatusUnauthorized, "токен недействителен или просрочен")
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole пропускает только пользователей с нужной ролью (используем на этапе 4).
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok || claims.Role != role {
				writeError(w, http.StatusForbidden, "недостаточно прав")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClaimsFromContext достаёт данные пользователя, которые положил Auth.
func ClaimsFromContext(ctx context.Context) (*service.Claims, bool) {
	claims, ok := ctx.Value(claimsKey).(*service.Claims)
	return claims, ok
}
