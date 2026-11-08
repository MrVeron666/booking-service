# Booking Service

Backend-сервис онлайн-записи клиентов для малого бизнеса (дипломный проект на Go).

Сейчас готов **этап 1**: каркас, конфиг, подключение к PostgreSQL, health-check.

## Запуск

```bash
docker compose up --build
```

Проверка:

```bash
curl http://localhost:8080/health
```

Ожидаемый ответ: `{"status":"ok"}`.
