# Booking Service

[![CI](https://github.com/ТВОЙ_ЛОГИН/booking-service/actions/workflows/ci.yml/badge.svg)](https://github.com/ТВОЙ_ЛОГИН/booking-service/actions/workflows/ci.yml)

Веб-сервис онлайн-записи клиентов для малого бизнеса (салон, барбершоп, автосервис, репетитор).
Дипломный проект: backend на Go + веб-интерфейс.

**Главная особенность:** двойная бронь одного времени невозможна даже при одновременных запросах.
Пересечение интервалов запрещает ограничение `EXCLUDE USING gist` в PostgreSQL,
а тест из 50 параллельных запросов на один слот подтверждает, что успешным будет ровно один.

## Возможности

- Регистрация и вход (JWT), роли: владелец (`owner`) и клиент (`client`).
- Владелец: управляет услугами, задаёт недельный график, видит записи на свои услуги.
- Клиент: выбирает услугу и дату, видит свободные слоты, записывается и отменяет запись.
- Слоты считаются из рабочих часов и длительности услуги, прошедшее время недоступно.
- Веб-интерфейс встроен в сервер (`go:embed`), отдельно запускать ничего не нужно.

## Технологии

Go, chi (роутер), PostgreSQL, pgx, golang-migrate, JWT, bcrypt, Docker Compose,
testify, GitHub Actions, HTML/CSS/JavaScript (без фреймворков).

## Быстрый старт

Нужны Docker Desktop и Git.

```bash
git clone https://github.com/ТВОЙ_ЛОГИН/booking-service.git
cd booking-service
cp .env.example .env
```

Открой `.env` и задай `JWT_SECRET` (случайная строка от 16 символов, например результат `openssl rand -hex 32`).

```bash
docker compose up --build
```

Миграции применяются автоматически. Открой http://localhost:8080:
зарегистрируйся как владелец, добавь услугу и график, затем в другом окне (режим инкогнито)
зарегистрируйся как клиент и запишись.

## Тесты

Unit-тесты (база не нужна):

```bash
go test ./internal/service/ -v
```

Интеграционные тесты (нужна запущенная база из `docker compose up`):

```bash
DATABASE_URL="postgres://booking:booking@localhost:5432/booking?sslmode=disable" go test ./... -count=1
```

## Архитектура

```
cmd/api            точка входа, сборка зависимостей, раздача фронтенда
internal/handler   HTTP: разбор запросов и ответы
internal/service   бизнес-логика (слоты, правила записи, авторизация)
internal/repository SQL-запросы к PostgreSQL
internal/middleware проверка JWT и роли
internal/model     структуры данных
migrations         SQL-миграции (применяются при старте)
```

Схемы (ER-диаграмма, диаграмма вариантов использования, последовательность записи)
лежат в [docs/DIAGRAMS.md](docs/DIAGRAMS.md).

## API

Полная спецификация: [openapi.yaml](openapi.yaml) (можно открыть на editor.swagger.io).

| Метод | Путь | Доступ | Описание |
|---|---|---|---|
| POST | /auth/register | все | регистрация |
| POST | /auth/login | все | вход, выдача токена |
| GET | /me | авторизованные | кто я |
| GET | /services | авторизованные | список услуг |
| GET | /services/{id} | авторизованные | одна услуга |
| POST | /services | owner | создать услугу |
| PUT | /services/{id} | owner | изменить свою услугу |
| DELETE | /services/{id} | owner | удалить свою услугу |
| GET, PUT | /working-hours | owner | график работы |
| GET | /services/{id}/slots?date= | авторизованные | свободные слоты |
| POST | /bookings | client | создать запись (409, если время занято) |
| GET | /bookings | авторизованные | мои записи / записи на мои услуги |
| PATCH | /bookings/{id}/cancel | участник записи | отменить запись |

## Конфигурация

| Переменная | Описание |
|---|---|
| `DATABASE_URL` | строка подключения к PostgreSQL |
| `HTTP_ADDR` | адрес сервера, по умолчанию `:8080` |
| `JWT_SECRET` | секрет подписи токенов (минимум 16 символов) |
| `TIMEZONE` | часовой пояс бизнеса, по умолчанию `Asia/Almaty` |