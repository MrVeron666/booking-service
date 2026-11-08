.PHONY: run docker-up docker-down tidy

# Локальный запуск (Postgres должен уже работать, например через docker compose).
run:
	go run ./cmd/api

tidy:
	go mod tidy

docker-up:
	docker compose up --build

docker-down:
	docker compose down
