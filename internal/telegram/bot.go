// Package telegram содержит Telegram-бота: привязку аккаунта, команды, уведомления и напоминания.
package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bogdan/booking-service/internal/model"
	"github.com/bogdan/booking-service/internal/repository"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// За сколько времени до визита отправляем напоминание.
const reminderBefore = 24 * time.Hour

const helpText = `Я бот сервиса онлайн-записи.

Как подключить уведомления:
1. Войдите на сайте.
2. В блоке «Уведомления в Telegram» нажмите «Подключить Telegram».
3. Перейдите по ссылке или отправьте мне /start КОД.

Команды:
/bookings — ближайшие записи
/unlink — отключить уведомления`

const notLinkedText = "Telegram ещё не подключён. Нажмите «Подключить Telegram» на сайте и отправьте мне полученный код."

type Bot struct {
	api      *tgbotapi.BotAPI
	repo     *repository.TelegramRepository
	bookings *repository.BookingRepository
	services *repository.ServiceRepository
	loc      *time.Location
}

// New создаёт бота. Конструктор обращается к Telegram, чтобы проверить токен.
func New(token string, repo *repository.TelegramRepository, bookings *repository.BookingRepository,
	services *repository.ServiceRepository, loc *time.Location) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Bot{api: api, repo: repo, bookings: bookings, services: services, loc: loc}, nil
}

// Username — имя бота (нужно для ссылки t.me/<имя>).
func (b *Bot) Username() string { return b.api.Self.UserName }

// Run запускает приём команд и отправку напоминаний. Блокирует вызывающий код, запускать через go.
func (b *Bot) Run(ctx context.Context) {
	go b.runReminders(ctx)

	cfg := tgbotapi.NewUpdate(0)
	cfg.Timeout = 30 // long polling: Telegram держит запрос до 30 секунд, пока нет новых сообщений
	updates := b.api.GetUpdatesChan(cfg)
	slog.Info("Telegram-бот запущен", "username", b.Username())

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return
		case upd := <-updates:
			if upd.Message == nil {
				continue
			}
			if upd.Message.IsCommand() {
				b.handleCommand(ctx, upd.Message)
			} else {
				b.send(upd.Message.Chat.ID, "Я понимаю только команды. Напишите /help.")
			}
		}
	}
}

func (b *Bot) handleCommand(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	switch msg.Command() {
	case "start":
		code := strings.TrimSpace(msg.CommandArguments())
		if code == "" {
			b.send(chatID, helpText)
			return
		}
		user, err := b.repo.LinkByCode(ctx, code, chatID)
		switch {
		case errors.Is(err, repository.ErrLinkCodeInvalid):
			b.send(chatID, "Код недействителен или просрочен. Получите новый на сайте в блоке «Уведомления в Telegram».")
		case err != nil:
			slog.Error("привязка Telegram", "err", err)
			b.send(chatID, "Что-то пошло не так, попробуйте позже.")
		default:
			b.send(chatID, fmt.Sprintf("Готово, %s! Telegram подключён.\nЗа сутки до записи я пришлю напоминание. Список записей: /bookings", user.Name))
		}
	case "bookings":
		b.cmdBookings(ctx, chatID)
	case "unlink":
		if err := b.repo.Unlink(ctx, chatID); err != nil {
			slog.Error("отвязка Telegram", "err", err)
			b.send(chatID, "Не удалось отключить, попробуйте позже.")
			return
		}
		b.send(chatID, "Уведомления отключены.")
	default:
		b.send(chatID, helpText)
	}
}

func (b *Bot) cmdBookings(ctx context.Context, chatID int64) {
	user, err := b.repo.UserByChat(ctx, chatID)
	if errors.Is(err, repository.ErrUserNotFound) {
		b.send(chatID, notLinkedText)
		return
	}
	if err != nil {
		slog.Error("поиск пользователя по чату", "err", err)
		b.send(chatID, "Что-то пошло не так, попробуйте позже.")
		return
	}
	items, err := b.bookings.ListByUser(ctx, user.ID, user.Role)
	if err != nil {
		slog.Error("список записей для бота", "err", err)
		b.send(chatID, "Что-то пошло не так, попробуйте позже.")
		return
	}

	var sb strings.Builder
	now := time.Now()
	for _, it := range items {
		if it.Status != "booked" || it.EndAt.Before(now) {
			continue // показываем только предстоящие активные записи
		}
		line := fmt.Sprintf("• %s — %s", b.when(it.StartAt), b.serviceName(ctx, it.ServiceID))
		if user.Role == "owner" {
			line += " (" + b.userName(ctx, it.ClientID) + ")"
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	if sb.Len() == 0 {
		b.send(chatID, "Предстоящих записей нет.")
		return
	}
	b.send(chatID, "Ближайшие записи:\n"+sb.String())
}

// BookingCreated уведомляет владельца о новой записи.
// Отправка идёт в горутине, чтобы Telegram не задерживал ответ клиенту на сайте.
func (b *Bot) BookingCreated(booking *model.Booking) {
	bk := *booking // копия: горутина не зависит от вызывающего кода
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		chatID, ok, err := b.repo.ChatByUser(ctx, bk.OwnerID)
		if err != nil {
			slog.Error("чат владельца", "err", err)
			return
		}
		if !ok {
			return // владелец не подключил Telegram
		}
		b.send(chatID, fmt.Sprintf("Новая запись: %s, %s.\nКлиент: %s.",
			b.serviceName(ctx, bk.ServiceID), b.when(bk.StartAt), b.userName(ctx, bk.ClientID)))
	}()
}

// BookingCancelled сообщает второй стороне об отмене: клиент отменил — узнаёт владелец, и наоборот.
func (b *Bot) BookingCancelled(booking *model.Booking, byUserID int64) {
	bk := *booking
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		service, when := b.serviceName(ctx, bk.ServiceID), b.when(bk.StartAt)
		target := bk.ClientID
		text := fmt.Sprintf("Владелец отменил вашу запись: %s, %s.", service, when)
		if byUserID == bk.ClientID {
			target = bk.OwnerID
			text = fmt.Sprintf("Клиент %s отменил запись: %s, %s.", b.userName(ctx, bk.ClientID), service, when)
		}

		chatID, ok, err := b.repo.ChatByUser(ctx, target)
		if err != nil {
			slog.Error("чат получателя", "err", err)
			return
		}
		if ok {
			b.send(chatID, text)
		}
	}()
}

// runReminders раз в минуту отправляет напоминания о записях в ближайшие 24 часа.
func (b *Bot) runReminders(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		b.sendDueReminders(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (b *Bot) sendDueReminders(ctx context.Context) {
	now := time.Now()
	due, err := b.repo.DueReminders(ctx, now, now.Add(reminderBefore))
	if err != nil {
		slog.Error("поиск напоминаний", "err", err)
		return
	}
	for _, r := range due {
		text := fmt.Sprintf("Напоминание: запись «%s» — %s.", r.ServiceName, b.when(r.StartAt))
		if _, err := b.api.Send(tgbotapi.NewMessage(r.ChatID, text)); err != nil {
			// Не отмечаем отправленным: попробуем снова на следующем шаге.
			slog.Error("не удалось отправить напоминание", "booking_id", r.BookingID, "err", err)
			continue
		}
		if err := b.repo.MarkReminded(ctx, r.BookingID); err != nil {
			slog.Error("не удалось отметить напоминание", "booking_id", r.BookingID, "err", err)
		}
	}
}

// --- вспомогательные функции ---

func (b *Bot) when(t time.Time) string {
	return t.In(b.loc).Format("02.01.2006 15:04")
}

func (b *Bot) serviceName(ctx context.Context, id int64) string {
	svc, err := b.services.GetByID(ctx, id)
	if err != nil {
		return fmt.Sprintf("услуга #%d", id)
	}
	return svc.Name
}

func (b *Bot) userName(ctx context.Context, id int64) string {
	name, err := b.repo.UserName(ctx, id)
	if err != nil {
		return fmt.Sprintf("пользователь #%d", id)
	}
	return name
}

// send отправляет обычный текст (без разметки, чтобы имена со спецсимволами не ломали сообщение).
func (b *Bot) send(chatID int64, text string) {
	if _, err := b.api.Send(tgbotapi.NewMessage(chatID, text)); err != nil {
		slog.Error("не удалось отправить сообщение в Telegram", "chat_id", chatID, "err", err)
	}
}
