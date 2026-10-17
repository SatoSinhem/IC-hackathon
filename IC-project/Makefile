.PHONY: db-up db-down api web fmt migrate-up migrate-down

db-up:
	docker compose up -d db

db-down:
	docker compose down

api:
	@set -a; [ ! -f .env ] || . ./.env; set +a; cd apps/api && go run ./cmd/api

web:
	cd apps/web && npm run dev

fmt:
	cd apps/api && gofmt -w ./cmd ./internal

migrate-up:
	@set -a; [ ! -f .env ] || . ./.env; set +a; migrate -path apps/api/migrations -database "$$DATABASE_URL" up

migrate-down:
	@set -a; [ ! -f .env ] || . ./.env; set +a; migrate -path apps/api/migrations -database "$$DATABASE_URL" down 1
