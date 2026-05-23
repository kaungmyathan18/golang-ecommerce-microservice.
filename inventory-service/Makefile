.PHONY: build-all test-all tidy-all build-api build-worker run-api run-worker

build-all: build-api build-worker

test-all:
	cd internal && go test ./...
	cd services/api && go test ./...
	cd services/worker && go test ./...

tidy-all:
	cd internal && go mod tidy
	cd services/api && go mod tidy
	cd services/worker && go mod tidy

build-api:
	cd services/api && go build -o ../../bin/api ./cmd/server

build-worker:
	cd services/worker && go build -o ../../bin/worker ./cmd/worker

run-api:
	cd services/api && go run ./cmd/server

run-worker:
	cd services/worker && go run ./cmd/worker
