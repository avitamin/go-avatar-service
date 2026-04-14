.PHONY: test build build-server build-worker build-contract-tests contract-tests run-server run-worker migrate-up migrate-down migrate-status

test:
	go test ./...

build:
	go build -o ./bin/avatars-service ./cmd/avatars-service

build-server:
	go build -o ./bin/server ./cmd/server

build-worker:
	go build -o ./bin/worker ./cmd/worker

build-contract-tests:
	go build -o ./bin/avatar-contract-tests ./cmd/avatar-contract-tests

contract-tests: build-contract-tests
	@test -n "$(BASE_URL)" || (echo "BASE_URL is required, example: BASE_URL=http://localhost:8080 make contract-tests" >&2; exit 2)
	./bin/avatar-contract-tests -base-url "$(BASE_URL)"

run-server:
	go run ./cmd/avatars-service server

run-worker:
	go run ./cmd/avatars-service worker

migrate-up:
	go run ./cmd/avatars-service migrate up

migrate-down:
	go run ./cmd/avatars-service migrate down

migrate-status:
	go run ./cmd/avatars-service migrate status
