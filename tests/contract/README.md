# Avatar Contract Tests

Black-box smoke runner for the future Avatar Service HTTP API.

Build:

```bash
go build -o ./bin/avatar-contract-tests ./cmd/avatar-contract-tests
```

Run against an already started service:

```bash
BASE_URL=http://localhost:8080 ./bin/avatar-contract-tests
```

Equivalent flags:

```bash
./bin/avatar-contract-tests -base-url http://localhost:8080 -timeout 30s -user-id contract-user
```

The runner does not import service internals and does not manage Docker Compose,
PostgreSQL, MinIO, RabbitMQ, migrations, or worker processes. It only verifies
the public API contract through HTTP.

Exit codes:

- `0`: all scenarios passed.
- `1`: at least one scenario failed.
- `2`: runner configuration is invalid.

