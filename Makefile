.PHONY: up down logs migrate-up migrate-down smoke-test

# .env が存在すれば読み込んで export する
-include .env
export

# マイグレーション用 DSN（コンテナ内からは db ホスト名で接続）
DB_DSN := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@db:5432/$(POSTGRES_DB)?sslmode=disable
COMPOSE_FILE := deploy/compose.yml
DOCKER_COMPOSE := docker compose -f $(COMPOSE_FILE)

up:
	$(DOCKER_COMPOSE) up -d

down:
	$(DOCKER_COMPOSE) down

logs:
	$(DOCKER_COMPOSE) logs -f

migrate-up:
	$(DOCKER_COMPOSE) run --rm migrate -path /migrations -database $(DB_DSN) up

migrate-down:
	$(DOCKER_COMPOSE) run --rm migrate -path /migrations -database $(DB_DSN) down

smoke-test:
	./scripts/test/verify_event_flow.sh
