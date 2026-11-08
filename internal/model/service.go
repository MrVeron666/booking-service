package model

import "time"

// Service — услуга, которую оказывает владелец (стрижка, диагностика и т.д.).
type Service struct {
	ID              int64     `json:"id"`
	OwnerID         int64     `json:"owner_id"`
	Name            string    `json:"name"`
	DurationMinutes int       `json:"duration_minutes"`
	Price           float64   `json:"price"`
	CreatedAt       time.Time `json:"created_at"`
}

// WorkingHour — рабочее время владельца в один из дней недели (0 = воскресенье).
type WorkingHour struct {
	Weekday   int    `json:"weekday"`
	StartTime string `json:"start_time"` // формат "09:00"
	EndTime   string `json:"end_time"`   // формат "18:00"
}
