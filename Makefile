ifneq (,$(wildcard .env))
include .env
endif

LOCAL_HTTP_ADDR ?= :18080
LOCAL_BASE_URL ?= http://localhost:18080
HTTP_ADDR ?= $(LOCAL_HTTP_ADDR)
BASE_URL ?= $(LOCAL_BASE_URL)
COMPOSE ?= docker compose
COMPOSE_FILE ?= docker-compose.yml
COMPOSE_HTTP_PORT ?= 8080
COMPOSE_BASE_URL ?= http://localhost:$(COMPOSE_HTTP_PORT)

.PHONY: test bench bench-external build build-server build-worker build-contract-tests contract-tests run-server run-worker migrate-up migrate-down migrate-status docker-build docker-up docker-up-build docker-up-detached docker-down docker-down-volumes docker-ps docker-logs docker-contract-tests

test:
	go test ./...

bench:
	go test -run='^$$' -bench=. -benchmem ./...

bench-external:
	go test -run='^$$' -bench='Benchmark(Postgres|RabbitMQ)' -benchmem ./internal/repository/postgres ./internal/broker/rabbitmq

build:
	go build -o ./bin/avatars-service ./cmd/avatars-service

build-server:
	go build -o ./bin/server ./cmd/server

build-worker:
	go build -o ./bin/worker ./cmd/worker

build-contract-tests:
	go build -o ./bin/avatar-contract-tests ./cmd/avatar-contract-tests

contract-tests: build-contract-tests
	@test -n "$(BASE_URL)" || (echo "BASE_URL is required, example: BASE_URL=http://localhost:18080 make contract-tests" >&2; exit 2)
	./bin/avatar-contract-tests -base-url "$(BASE_URL)"

run-server:
	HTTP_ADDR="$(HTTP_ADDR)" go run ./cmd/avatars-service server

run-worker:
	go run ./cmd/avatars-service worker

migrate-up:
	go run ./cmd/avatars-service migrate up

migrate-down:
	go run ./cmd/avatars-service migrate down

migrate-status:
	go run ./cmd/avatars-service migrate status

docker-build:
	$(COMPOSE) -f $(COMPOSE_FILE) build

docker-up:
	$(COMPOSE) -f $(COMPOSE_FILE) up

docker-up-build:
	$(COMPOSE) -f $(COMPOSE_FILE) up --build

docker-up-detached:
	$(COMPOSE) -f $(COMPOSE_FILE) up -d --build

docker-down:
	$(COMPOSE) -f $(COMPOSE_FILE) down

docker-down-volumes:
	$(COMPOSE) -f $(COMPOSE_FILE) down -v --remove-orphans

docker-ps:
	$(COMPOSE) -f $(COMPOSE_FILE) ps

docker-logs:
	$(COMPOSE) -f $(COMPOSE_FILE) logs -f

docker-contract-tests: build-contract-tests
	$(MAKE) contract-tests BASE_URL="$(COMPOSE_BASE_URL)"
