package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bogdan/booking-service/internal/model"
	"github.com/bogdan/booking-service/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidInput       = errors.New("некорректные данные")
	ErrInvalidCredentials = errors.New("неверный email или пароль")
)

// Claims — данные, которые мы кладём внутрь токена.
type Claims struct {
	UserID int64  `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type AuthService struct {
	users     *repository.UserRepository
	jwtSecret []byte
	tokenTTL  time.Duration
}

func NewAuthService(users *repository.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{
		users:     users,
		jwtSecret: []byte(jwtSecret),
		tokenTTL:  24 * time.Hour, // токен живёт сутки
	}
}

// Register проверяет данные, хеширует пароль и сохраняет пользователя.
func (s *AuthService) Register(ctx context.Context, name, email, password, role string) (*model.User, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))

	if name == "" || !strings.Contains(email, "@") || len(password) < 8 {
		return nil, ErrInvalidInput
	}
	if role != "owner" && role != "client" {
		return nil, ErrInvalidInput
	}

	// bcrypt сам добавляет "соль", поэтому одинаковые пароли дают разные хеши.
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	u := &model.User{Name: name, Email: email, PasswordHash: string(hash), Role: role}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Login проверяет пароль и возвращает JWT-токен.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	u, err := s.users.GetByEmail(ctx, email)
	if errors.Is(err, repository.ErrUserNotFound) {
		// Отвечаем одинаково и для "нет email", и для "неверный пароль",
		// чтобы нельзя было выяснить, какие email зарегистрированы.
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}
	return s.generateToken(u)
}

func (s *AuthService) generateToken(u *model.User) (string, error) {
	claims := Claims{
		UserID: u.ID,
		Role:   u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ParseToken проверяет подпись и срок действия токена и возвращает данные из него.
func (s *AuthService) ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		// Принимаем только тот алгоритм подписи, которым пользуемся сами.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("неожиданный алгоритм подписи")
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("недействительный токен")
	}
	return claims, nil
}
