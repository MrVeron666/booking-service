package repository

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bogdan/booking-service/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixture создаёт владельца, клиента и услугу и удаляет их после теста.
type fixture struct {
	pool      *pgxpool.Pool
	ownerID   int64
	clientID  int64
	serviceID int64
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL не задан: интеграционный тест пропущен")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx), "не удалось подключиться к БД")

	f := &fixture{pool: pool}
	suffix := time.Now().UnixNano() // уникальные email, чтобы тесты не мешали друг другу

	err = pool.QueryRow(ctx,
		`INSERT INTO users (name, email, password_hash, role)
		 VALUES ('Owner', $1, 'x', 'owner') RETURNING id`,
		fmt.Sprintf("owner-%d@test.local", suffix)).Scan(&f.ownerID)
	require.NoError(t, err)

	err = pool.QueryRow(ctx,
		`INSERT INTO users (name, email, password_hash, role)
		 VALUES ('Client', $1, 'x', 'client') RETURNING id`,
		fmt.Sprintf("client-%d@test.local", suffix)).Scan(&f.clientID)
	require.NoError(t, err)

	err = pool.QueryRow(ctx,
		`INSERT INTO services (owner_id, name, duration_minutes, price)
		 VALUES ($1, 'Test', 60, 1000) RETURNING id`, f.ownerID).Scan(&f.serviceID)
	require.NoError(t, err)

	// t.Cleanup выполнится после теста, даже если он упал.
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM bookings WHERE owner_id = $1`, f.ownerID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2)`, f.ownerID, f.clientID)
		pool.Close()
	})
	return f
}

func (f *fixture) booking(start time.Time) *model.Booking {
	return &model.Booking{
		ServiceID: f.serviceID,
		OwnerID:   f.ownerID,
		ClientID:  f.clientID,
		StartAt:   start,
		EndAt:     start.Add(time.Hour),
	}
}

// 50 запросов одновременно пытаются занять один и тот же слот: должен победить ровно один.
func TestCreateBooking_ConcurrentSameSlot(t *testing.T) {
	f := newFixture(t)
	repo := NewBookingRepository(f.pool)
	ctx := context.Background()
	start := time.Date(2030, 1, 7, 10, 0, 0, 0, time.UTC)

	const n = 50
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		success   int
		conflicts int
		others    []error
	)
	gate := make(chan struct{}) // «стартовый пистолет»: все горутины ждут его закрытия

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			err := repo.Create(ctx, f.booking(start))

			mu.Lock()
			defer mu.Unlock()
			switch err {
			case nil:
				success++
			case ErrSlotTaken:
				conflicts++
			default:
				others = append(others, err)
			}
		}()
	}
	close(gate) // отпускаем все горутины одновременно
	wg.Wait()

	assert.Empty(t, others, "неожиданные ошибки")
	assert.Equal(t, 1, success, "должна пройти ровно одна запись")
	assert.Equal(t, n-1, conflicts, "остальные должны получить ErrSlotTaken")

	var count int
	err := f.pool.QueryRow(ctx,
		`SELECT count(*) FROM bookings WHERE owner_id = $1 AND status = 'booked'`, f.ownerID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "в базе должна остаться одна активная запись")
}

// После отмены слот снова можно занять.
func TestCreateBooking_SlotFreedAfterCancel(t *testing.T) {
	f := newFixture(t)
	repo := NewBookingRepository(f.pool)
	ctx := context.Background()
	start := time.Date(2030, 1, 7, 12, 0, 0, 0, time.UTC)

	first := f.booking(start)
	require.NoError(t, repo.Create(ctx, first))

	// Пока запись активна, вторая попытка отклоняется.
	assert.ErrorIs(t, repo.Create(ctx, f.booking(start)), ErrSlotTaken)

	_, err := repo.Cancel(ctx, first.ID, f.clientID)
	require.NoError(t, err)

	// После отмены то же время снова свободно.
	assert.NoError(t, repo.Create(ctx, f.booking(start)))
}

// Записи встык (10:00-11:00 и 11:00-12:00) не конфликтуют.
func TestCreateBooking_AdjacentSlotsAllowed(t *testing.T) {
	f := newFixture(t)
	repo := NewBookingRepository(f.pool)
	ctx := context.Background()
	start := time.Date(2030, 1, 7, 14, 0, 0, 0, time.UTC)

	require.NoError(t, repo.Create(ctx, f.booking(start)))
	assert.NoError(t, repo.Create(ctx, f.booking(start.Add(time.Hour))))
}
