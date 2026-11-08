package service

import (
	"testing"
	"time"

	"github.com/bogdan/booking-service/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testLoc = time.FixedZone("ALA", 5*3600) // UTC+5, как в Алматы

// at возвращает момент времени 16 ноября 2026 в заданные часы и минуты.
func at(h, m int) time.Time {
	return time.Date(2026, 11, 16, h, m, 0, 0, testLoc)
}

// Далёкое прошлое: все слоты считаются будущими.
var longAgo = time.Date(2026, 11, 1, 0, 0, 0, 0, testLoc)

func TestOverlaps(t *testing.T) {
	busy := model.Interval{Start: at(10, 0), End: at(11, 0)}

	// Табличный тест: список случаев и один цикл, который их проверяет.
	tests := []struct {
		name       string
		start, end time.Time
		want       bool
	}{
		{"раньше без касания", at(8, 0), at(9, 0), false},
		{"встык до занятого", at(9, 0), at(10, 0), false},
		{"встык после занятого", at(11, 0), at(12, 0), false},
		{"пересекает начало", at(9, 30), at(10, 30), true},
		{"пересекает конец", at(10, 30), at(11, 30), true},
		{"внутри занятого", at(10, 15), at(10, 45), true},
		{"накрывает занятое", at(9, 0), at(12, 0), true},
		{"совпадает полностью", at(10, 0), at(11, 0), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, overlaps(tt.start, tt.end, busy))
		})
	}
}

func TestBuildSlots_FullDay(t *testing.T) {
	slots := buildSlots(at(9, 0), at(18, 0), time.Hour, nil, longAgo)

	require.Len(t, slots, 9) // 09:00, 10:00, ..., 17:00
	assert.Equal(t, at(9, 0), slots[0].Start)
	assert.Equal(t, at(10, 0), slots[0].End)
	assert.Equal(t, at(17, 0), slots[8].Start)
	assert.Equal(t, at(18, 0), slots[8].End)
}

func TestBuildSlots_ExcludesBusy(t *testing.T) {
	busy := []model.Interval{{Start: at(10, 0), End: at(11, 0)}}
	slots := buildSlots(at(9, 0), at(18, 0), time.Hour, busy, longAgo)

	require.Len(t, slots, 8)
	for _, s := range slots {
		assert.NotEqual(t, at(10, 0), s.Start, "занятый слот не должен попасть в список")
	}
}

func TestBuildSlots_BusyCoversTwoSlots(t *testing.T) {
	// Запись 10:30-11:30 мешает и слоту 10:00, и слоту 11:00.
	busy := []model.Interval{{Start: at(10, 30), End: at(11, 30)}}
	slots := buildSlots(at(9, 0), at(18, 0), time.Hour, busy, longAgo)

	require.Len(t, slots, 7)
	for _, s := range slots {
		assert.NotEqual(t, at(10, 0), s.Start)
		assert.NotEqual(t, at(11, 0), s.Start)
	}
}

func TestBuildSlots_SkipsPast(t *testing.T) {
	// Сейчас 12:30: слоты 09:00..12:00 уже начались, остаются 13:00..17:00.
	slots := buildSlots(at(9, 0), at(18, 0), time.Hour, nil, at(12, 30))

	require.Len(t, slots, 5)
	assert.Equal(t, at(13, 0), slots[0].Start)
}

func TestBuildSlots_DurationNotDividingWindow(t *testing.T) {
	// 90-минутные слоты в окне 9:00-18:00: последний слот 16:30-18:00 влезает ровно.
	slots := buildSlots(at(9, 0), at(18, 0), 90*time.Minute, nil, longAgo)

	require.Len(t, slots, 6)
	assert.Equal(t, at(16, 30), slots[5].Start)
	assert.Equal(t, at(18, 0), slots[5].End)
}

func TestBuildSlots_LastSlotMustFit(t *testing.T) {
	// Окно 9:00-10:30, услуга 60 минут: влезает только 9:00-10:00.
	slots := buildSlots(at(9, 0), at(10, 30), time.Hour, nil, longAgo)

	require.Len(t, slots, 1)
	assert.Equal(t, at(9, 0), slots[0].Start)
}

func TestBuildSlots_ZeroDuration(t *testing.T) {
	slots := buildSlots(at(9, 0), at(18, 0), 0, nil, longAgo)
	assert.Empty(t, slots) // без защиты в коде был бы бесконечный цикл
}
