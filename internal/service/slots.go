package service

import (
	"time"

	"github.com/bogdan/booking-service/internal/model"
)

// overlaps сообщает, пересекаются ли промежутки [start, end) и b.
func overlaps(start, end time.Time, b model.Interval) bool {
	return start.Before(b.End) && b.Start.Before(end)
}

// buildSlots нарезает рабочее окно на слоты длиной dur и оставляет только свободные:
// не в прошлом и не пересекающиеся с занятыми интервалами.
func buildSlots(winStart, winEnd time.Time, dur time.Duration, busy []model.Interval, now time.Time) []model.Slot {
	slots := []model.Slot{}
	if dur <= 0 {
		return slots
	}
	for start := winStart; !start.Add(dur).After(winEnd); start = start.Add(dur) {
		end := start.Add(dur)
		if !start.After(now) {
			continue // слот уже начался или прошёл
		}
		taken := false
		for _, b := range busy {
			if overlaps(start, end, b) {
				taken = true
				break
			}
		}
		if !taken {
			slots = append(slots, model.Slot{Start: start, End: end})
		}
	}
	return slots
}
