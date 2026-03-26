SHELL := /bin/sh

.PHONY: build test run-bridge run-worker run-tui migrate-up migrate-down fmt

build:
	go build -o bin/bridge ./cmd/bridge
	go build -o bin/worker ./cmd/worker
	go build -o bin/tui ./cmd/tui

test:
	go test ./...

fmt:
	go fmt ./...

run-bridge:
	go run ./cmd/bridge serve

run-worker:
	go run ./cmd/worker run

run-tui:
	go run ./cmd/tui

migrate-up:
	go run ./cmd/bridge migrate up

migrate-down:
	go run ./cmd/bridge migrate down

