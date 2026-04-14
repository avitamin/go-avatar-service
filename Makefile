.PHONY: test build-server build-worker build-contract-tests contract-tests

test:
	go test ./...

build-server:
	go build -o ./bin/server ./cmd/server

build-worker:
	go build -o ./bin/worker ./cmd/worker

build-contract-tests:
	go build -o ./bin/avatar-contract-tests ./cmd/avatar-contract-tests

contract-tests: build-contract-tests
	@test -n "$(BASE_URL)" || (echo "BASE_URL is required, example: BASE_URL=http://localhost:8080 make contract-tests" >&2; exit 2)
	./bin/avatar-contract-tests -base-url "$(BASE_URL)"
