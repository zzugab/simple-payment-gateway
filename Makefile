.PHONY: run test build fmt vet tidy

run:
	set -a && . ./.env && set +a && go run ./cmd/api

test:
	go test ./...

build:
	go build -o bin/api ./cmd/api

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

tidy:
	go mod tidy
