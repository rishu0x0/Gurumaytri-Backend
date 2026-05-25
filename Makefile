.PHONY: run build test lint tidy

run:
	set -a && source ../.env && set +a && go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./... -v

lint:
	go vet ./...

tidy:
	go mod tidy
