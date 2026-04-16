# Benchmarking

Этот документ описывает локальный benchmark workflow для `go-avatar-service`.
Бенчмарки нужны для отслеживания стоимости hot paths, а не как жесткий performance gate для каждого изменения.

## Что покрыто

Текущие benchmark suites находятся рядом с пакетами:

- `internal/domain` - validation и status mapping.
- `internal/imageproc` - magic bytes sniffing, decode jpeg/png и thumbnail generation.
- `internal/service` - upload, metadata, list/fallback paths на in-memory core.
- `internal/http` - routing overhead для health, read и upload.
- `internal/worker` - thumbnail generation через upload handler и быстрый retry path.
- `internal/repository/postgres` - opt-in benchmarks PostgreSQL repository operations.
- `internal/broker/rabbitmq` - opt-in benchmarks RabbitMQ publish/get/ack roundtrip.

## Команды

Полный локальный прогон:

```bash
make bench
```

Эквивалентная Go-команда:

```bash
go test -run='^$' -bench=. -benchmem ./...
```

Быстрый compile/smoke-прогон benchmark-функций:

```bash
go test -run='^$' -bench=. -benchtime=1x -benchmem ./...
```

External adapter benchmarks пропускаются, если не заданы env-переменные подключения:

```bash
POSTGRES_DSN='postgres://avatars:avatars@localhost:5432/avatars?sslmode=disable' \
RABBITMQ_URL='amqp://guest:guest@localhost:5672/' \
make bench-external
```

Отдельные прогоны:

```bash
POSTGRES_DSN='postgres://avatars:avatars@localhost:5432/avatars?sslmode=disable' \
go test -run='^$' -bench='BenchmarkPostgres' -benchmem ./internal/repository/postgres

RABBITMQ_URL='amqp://guest:guest@localhost:5672/' \
go test -run='^$' -bench='BenchmarkRabbitMQ' -benchmem ./internal/broker/rabbitmq
```

Точечный прогон пакета:

```bash
go test -run='^$' -bench=. -benchmem ./internal/imageproc
go test -run='^$' -bench='BenchmarkThumbnailJPEG' -benchmem ./internal/imageproc
```

Стабильнее сравнивать несколько прогонов:

```bash
go test -run='^$' -bench=. -benchmem -count=5 ./internal/imageproc
```

Снять CPU и memory profiles для конкретного benchmark:

```bash
go test -run='^$' -bench='BenchmarkThumbnailJPEG' -benchmem \
  -cpuprofile=/tmp/avatar-cpu.out \
  -memprofile=/tmp/avatar-mem.out \
  ./internal/imageproc
```

Посмотреть основные hotspots:

```bash
go tool pprof -top /tmp/avatar-cpu.out
go tool pprof -top /tmp/avatar-mem.out
```

## Когда запускать

`make bench` стоит запускать опционально:

- перед изменениями в `internal/imageproc`, `internal/service`, `internal/http`, `internal/worker`, `internal/repository/postgres` или `internal/broker/rabbitmq`;
- после изменения алгоритмов thumbnail generation, fallback/list selection, storage/repository paths или middleware;
- перед PR, если изменение может повлиять на latency, allocations или CPU-bound обработку изображений.

Для обычных small docs/config changes достаточно `go test ./...`.

## Выявление проблемных мест

Используйте benchmarks как triage, а profiles - только после того, как есть повторяемый подозрительный сигнал.

1. Зафиксируйте корректность:

```bash
go test ./...
```

2. Получите общий baseline:

```bash
make bench
```

3. Повторите подозрительный пакет несколько раз:

```bash
go test -run='^$' -bench=. -benchmem -count=5 ./internal/<package>
```

4. Для конкретного benchmark снимите CPU/memory profiles в `/tmp` и посмотрите `top` output через `go tool pprof`.

5. Оптимизируйте только после измерения: проблема должна быть повторяемой регрессией или явным hot path с высокой стоимостью.

## Как читать результат

Go benchmark output показывает:

- `ns/op` - среднее время операции;
- `B/op` - выделенная память на операцию;
- `allocs/op` - количество allocation на операцию.

Для этого проекта важнее всего:

- `ThumbnailJPEG` и `UploadHandlerGenerateThumbnails`: CPU и allocations image processing path.
- `ListActiveByUser` и `ReadUserAvatarFallback`: стоимость selection/fallback при росте числа записей.
- `RouterUploadJPEG`: upload validation + multipart + service path.
- `Postgres*`: стоимость SQL repository операций при реальном PostgreSQL.
- `RabbitMQ*`: broker roundtrip publish/get/ack для upload/delete queues.

Не сравнивайте единичные прогоны как точные абсолютные значения: локальная нагрузка, CPU governor и кеши заметно влияют на цифры. Для регрессий используйте повторные прогоны одной и той же команды на одной машине.

## Triage matrix

| Сигнал | Вероятная зона | Что проверять дальше |
| --- | --- | --- |
| Высокий `ns/op` в `BenchmarkThumbnailJPEG` или `BenchmarkUploadHandlerGenerateThumbnails` | CPU-bound image processing | CPU profile, resize loop, encode/decode cost |
| Рост `B/op` или `allocs/op` в `BenchmarkRouterUploadJPEG` | HTTP upload path | multipart parsing, image decode, request/response allocations |
| Рост `BenchmarkListActiveByUser` | Repository selection/sort | линейный scan, sort cost, размер user history |
| Рост `BenchmarkPostgresCreate`, `BenchmarkPostgresGetByID` или `BenchmarkPostgresListActiveByUser` | PostgreSQL repository adapter | query plan, indexes, connection pool, локальная нагрузка БД |
| Рост `BenchmarkRabbitMQPublishGetAckUpload` или `BenchmarkRabbitMQPublishGetAckDelete` | RabbitMQ broker adapter | broker durability, queue depth, disk latency, network/localhost overhead |
| Рост `BenchmarkReadUserAvatarFallback` | User avatar fallback | количество skipped records, storage `Get`/`Exists`, copy allocations |
| Рост `BenchmarkRouterHealth` или `BenchmarkRouterReadAvatar` | Router/middleware overhead | access log middleware, JSON/binary response path |

Если profile показывает hotspot в стандартной библиотеке, сначала проверьте, не вызван ли он размером fixture, лишним копированием или repeated setup внутри benchmark loop.

## Ограничения

Базовые benchmarks используют in-memory repository/storage и подходят для CPU/allocation hot paths без внешней инфраструктуры. PostgreSQL и RabbitMQ benchmarks являются opt-in: они требуют поднятых сервисов и env-переменных подключения, поэтому их результаты нельзя напрямую сравнивать с in-memory пакетами. MinIO benchmark пока не добавлен.
