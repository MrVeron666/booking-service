package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/bogdan/booking-service/internal/repository"
)

var ErrTelegramDisabled = errors.New("Telegram-бот не настроен")

const linkCodeTTL = 15 * time.Minute

// TelegramService выдаёт коды привязки и сообщает, подключён ли Telegram.
type TelegramService struct {
	repo        *repository.TelegramRepository
	botUsername string // пустой, если бот не запущен
}

func NewTelegramService(repo *repository.TelegramRepository, botUsername string) *TelegramService {
	return &TelegramService{repo: repo, botUsername: botUsername}
}

type TelegramStatus struct {
	Enabled bool `json:"enabled"` // бот настроен на сервере
	Linked  bool `json:"linked"`  // этот пользователь подключил Telegram
}

type LinkInfo struct {
	Code             string `json:"code"`
	Link             string `json:"link"`
	ExpiresInMinutes int    `json:"expires_in_minutes"`
}

func (s *TelegramService) Status(ctx context.Context, userID int64) (TelegramStatus, error) {
	if s.botUsername == "" {
		return TelegramStatus{}, nil
	}
	_, linked, err := s.repo.ChatByUser(ctx, userID)
	if err != nil {
		return TelegramStatus{}, err
	}
	return TelegramStatus{Enabled: true, Linked: linked}, nil
}

// CreateLink создаёт одноразовый код и ссылку вида https://t.me/<бот>?start=<код>.
func (s *TelegramService) CreateLink(ctx context.Context, userID int64) (*LinkInfo, error) {
	if s.botUsername == "" {
		return nil, ErrTelegramDisabled
	}
	// crypto/rand — криптостойкий генератор: код нельзя угадать.
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	code := hex.EncodeToString(raw)

	if err := s.repo.CreateLinkCode(ctx, userID, code, linkCodeTTL); err != nil {
		return nil, err
	}
	return &LinkInfo{
		Code:             code,
		Link:             "https://t.me/" + s.botUsername + "?start=" + code,
		ExpiresInMinutes: int(linkCodeTTL.Minutes()),
	}, nil
}
