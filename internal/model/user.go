package model

import "time"

// User — пользователь системы (владелец бизнеса или клиент).
type User struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // "-" значит: никогда не отдавать в JSON
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}
