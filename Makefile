.PHONY: up down logs build run test fmt vet tidy

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f api

build:
	docker compose build api

# Optional host-based development; requires Go installed locally.
run:
	set -a && . ./.env && set +a && go run ./cmd/api

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

tidy:
	go mod tidy
