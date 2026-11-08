package model

import "time"

// Booking — запись клиента на услугу.
type Booking struct {
	ID        int64     `json:"id"`
	ServiceID int64     `json:"service_id"`
	OwnerID   int64     `json:"owner_id"`
	ClientID  int64     `json:"client_id"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Interval — занятый промежуток времени (внутреннее использование).
type Interval struct {
	Start time.Time
	End   time.Time
}

// Slot — свободное время, которое можно забронировать.
type Slot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}
