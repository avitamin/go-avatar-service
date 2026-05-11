# E2E-автотесты метрик и алертинга

## Статус e2e-автотестов

Статус: e2e-автотесты еще не реализованы; этот документ описывает план их реализации через TDD.

Этот документ описывает отдельный e2e test harness для автопроверки observability stack. Он не меняет текущие alert rules и не заменяет existing unit/config tests. Основная цель - добавить e2e-автотесты, которые проверяют путь от пользовательского или test-only события до Prometheus metrics, Prometheus alert state и Alertmanager wiring.

## Источники и текущий baseline

Подтверждено текущими файлами проекта:

- `GET /metrics` на server отдает application metrics через `internal/observability`.
- Worker может отдавать `/metrics`, если задан `METRICS_ADDR`; observability compose задает `METRICS_ADDR=:9091`.
- `docker-compose.observability.yml` поднимает Prometheus, Alertmanager, Grafana, Jaeger, Loki, OpenTelemetry Collector, postgres-exporter и RabbitMQ Prometheus plugin.
- `configs/observability/prometheus/prometheus.yml` scrape-ит server, worker, postgres-exporter и RabbitMQ.
- `configs/observability/prometheus/alerts.yml` содержит 7 alert rules:
  - `HighHTTPErrorRate`
  - `HighUploadErrorRate`
  - `HighResponseTimeP95`
  - `UploadLatencyCritical`
  - `DependencyOperationErrors`
  - `RabbitMQQueueBacklog`
  - `WorkerProcessingFailures`
- `internal/observability/grafana_dashboards_test.go` уже проверяет YAML/JSON конфиги, наличие alert rules и базовый wiring Alertmanager.

## Цель e2e-автотестов

Добавить opt-in e2e-проверки, которые:

- запускаются против уже поднятого observability stack;
- не импортируют `internal/` код сервиса;
- проверяют Prometheus и Alertmanager через HTTP API;
- покрывают все текущие alert rules;
- используют TDD по вертикальным срезам: один сценарий -> минимальная реализация -> следующий сценарий.

Не цель этого плана:

- менять production thresholds и `for` windows в `alerts.yml`;
- делать долгий production-timing test обязательным PR gate;
- добавлять реальные external notification receivers;
- превращать test-only hooks в публичный API.

## Архитектурные решения

### Отдельный Go runner

Добавить отдельную команду:

```text
cmd/observability-e2e
```

Runner работает как black-box client и обращается только к HTTP endpoints:

- avatar-service server API;
- Prometheus HTTP API;
- Alertmanager HTTP API;
- RabbitMQ Management HTTP API, если сценарий требует публикации или проверки queue state.

Базовый CLI interface:

```bash
./bin/observability-e2e \
  -base-url http://localhost:8080 \
  -prometheus-url http://localhost:9090 \
  -alertmanager-url http://localhost:9093 \
  -rabbitmq-management-url http://localhost:15672 \
  -timeout 2m \
  -verbose
```

Флаги:

- `-base-url` - URL avatar-service server, default из `BASE_URL`.
- `-prometheus-url` - URL Prometheus, default из `PROMETHEUS_URL`.
- `-alertmanager-url` - URL Alertmanager, default из `ALERTMANAGER_URL`.
- `-rabbitmq-management-url` - URL RabbitMQ Management API, default из `RABBITMQ_MANAGEMENT_URL`.
- `-timeout` - общий timeout runner-а.
- `-verbose` - печатать durations, PromQL queries и последние observed states.

Exit codes:

- `0` - все scenarios прошли;
- `1` - минимум один scenario failed;
- `2` - configuration error.

### Make targets

Добавить opt-in targets:

```make
build-observability-e2e:
	go build -o ./bin/observability-e2e ./cmd/observability-e2e

observability-e2e: build-observability-e2e
	./bin/observability-e2e ...

docker-observability-e2e: build-observability-e2e
	./bin/observability-e2e ...
```

Defaults должны совпадать с compose ports:

- service: `http://localhost:8080`
- Prometheus: `http://localhost:9090`
- Alertmanager: `http://localhost:9093`
- RabbitMQ Management: `http://localhost:15672`

### Promtool rule tests плюс e2e smoke

Не делать production `for: 2m/5m/10m` обязательным быстрым gate. Для точной проверки выражений добавить `promtool` rule tests с synthetic time series:

- inactive case для каждого rule;
- firing case для каждого rule;
- проверка labels `severity`, `service`, `component` там, где component обязателен;
- проверка основных annotations.

E2E runner проверяет runtime wiring:

- targets scraped;
- rules loaded;
- alert state появляется в Prometheus `ALERTS`;
- Alertmanager доступен и получает alerts;
- реальные metrics series появляются после действий runner-а.

### Test-only hooks

Для детерминированных сложных симптомов добавить opt-in hooks, включаемые только при:

```text
OBSERVABILITY_E2E_HOOKS_ENABLED=true
```

Правила hooks:

- routes или behavior недоступны без env flag;
- hooks не документируются как product API;
- hooks не должны менять confirmed requirements/v1 spec;
- hooks должны жить отдельно от business handlers, чтобы production paths оставались очевидными;
- response bodies не должны раскрывать секреты, env или storage paths.

Минимальный набор hooks для плана:

- controlled HTTP 5xx для `HighHTTPErrorRate`;
- controlled latency для `HighResponseTimeP95` и `UploadLatencyCritical`;
- controlled dependency operation error для `DependencyOperationErrors`;
- controlled worker failure или poison delivery path для `WorkerProcessingFailures`;
- controlled worker pause или compose orchestration для `RabbitMQQueueBacklog`.

Если hook можно заменить надежным внешним действием без flakiness, предпочтителен внешний action. Например, invalid upload через публичный API достаточно надежен для `HighUploadErrorRate`.

## TDD sequence

### Cycle 1: runner configuration

RED:

- test создает runner config без URLs и ожидает configuration error;
- test передает invalid URL и ожидает error с названием флага.

GREEN:

- добавить minimal config parser и URL normalization.

Acceptance:

- runner не стартует с неполной конфигурацией;
- defaults читаются из env;
- ошибки конфигурации дают exit code `2`.

### Cycle 2: Prometheus readiness and scrape targets

RED:

- scenario ожидает, что Prometheus API доступен и targets `avatar-service-server`, `avatar-service-worker`, `postgres`, `rabbitmq` находятся в healthy или диагностируемом state.

GREEN:

- добавить Prometheus client для `/api/v1/targets`;
- вывести понятную ошибку с unhealthy targets.

Acceptance:

- scenario fail-ит, если Prometheus недоступен;
- scenario fail-ит, если server/worker scrape отсутствует;
- scenario не требует Grafana.

### Cycle 3: server metrics appear after public API traffic

RED:

- scenario вызывает `/health` и проверяет Prometheus query для `http_requests_total{route="/health"}`.

GREEN:

- добавить helper `waitPromQL(query, wantNonEmpty, timeout)`;
- poll-ить до scrape/evaluation deadline.

Acceptance:

- `http_requests_total` появляется после scrape;
- labels используют route templates;
- raw `avatar_id` и `user_id` не появляются в route labels.

### Cycle 4: upload metrics appear after API scenarios

RED:

- scenario выполняет valid upload и invalid upload;
- проверяет `avatars_uploads_total{status="success"}` или фактический success label из кода;
- проверяет `avatars_uploads_total{status="error"}`.

GREEN:

- переиспользовать multipart helpers по публичному HTTP contract или вынести общие black-box helpers без импорта `internal/`.

Acceptance:

- upload success/error metrics видны в Prometheus;
- `mime_type` label не содержит user-controlled high-cardinality values beyond normalized MIME.

### Cycle 5: rules are loaded

RED:

- scenario проверяет `/api/v1/rules` и ожидает все 7 alert names.

GREEN:

- добавить Prometheus rules client и model для alerting rules.

Acceptance:

- missing rule перечисляется в ошибке;
- malformed Prometheus response fail-ит scenario.

### Cycle 6: promtool rule tests

RED:

- добавить rule test fixture, которая сначала fail-ит для одного expression или отсутствующего expected alert.

GREEN:

- добавить fixtures для всех 7 rules;
- добавить make target или documented command для `promtool test rules`.

Acceptance:

- каждый rule имеет inactive и firing fixture;
- fixtures проверяют labels and annotations;
- test не зависит от running Docker stack.

### Cycle 7: Alertmanager wiring

RED:

- scenario проверяет Alertmanager API readiness и ожидает минимум один alert после controlled trigger.

GREEN:

- добавить Alertmanager client для `/api/v2/status` и `/api/v2/alerts`;
- использовать самый быстрый controlled alert path из e2e overlay или hook.

Acceptance:

- runner отличает "Prometheus alert pending/firing" от "Alertmanager не получил alert";
- null receiver допустим, real notification receiver не требуется.

### Cycle 8: HighHTTPErrorRate

RED:

- scenario вызывает gated 5xx hook и ожидает `ALERTS{alertname="HighHTTPErrorRate"}`.

GREEN:

- добавить hook только при `OBSERVABILITY_E2E_HOOKS_ENABLED=true`;
- добавить runner scenario и PromQL wait.

Acceptance:

- без env flag hook возвращает 404 или 403;
- с env flag controlled 5xx увеличивает `http_requests_total{status=~"5.."}`;
- alert state подтверждается через Prometheus.

### Cycle 9: HighUploadErrorRate

RED:

- scenario выполняет серию invalid uploads через публичный API;
- ожидает error upload metric и alert state.

GREEN:

- добавить сценарий без test-only hook.

Acceptance:

- invalid uploads возвращают expected client error;
- `avatars_uploads_total{status="error"}` растет;
- alert expression покрыт promtool fixture, e2e подтверждает runtime series.

### Cycle 10: HighResponseTimeP95

RED:

- scenario включает controlled HTTP latency и ожидает histogram buckets, достаточные для alert expression.

GREEN:

- добавить gated latency hook или middleware branch, активный только для e2e;
- добавить runner scenario.

Acceptance:

- latency path не доступен в normal runtime;
- `http_request_duration_seconds_bucket` series появляются;
- rule firing behavior покрыт promtool, runtime series подтверждены e2e.

### Cycle 11: UploadLatencyCritical

RED:

- scenario выполняет delayed upload path и ожидает upload duration buckets.

GREEN:

- добавить gated upload delay hook, не меняющий обычный upload contract;
- добавить runner scenario.

Acceptance:

- `avatars_upload_duration_seconds_bucket` получает controlled observations;
- promtool fixture подтверждает critical alert expression;
- e2e подтверждает runtime metric path.

### Cycle 12: DependencyOperationErrors

RED:

- scenario вызывает controlled dependency error и ожидает `avatar_dependency_operations_total{status="error"}`.

GREEN:

- добавить gated dependency failure hook или надежную dependency outage orchestration;
- добавить runner scenario.

Acceptance:

- component label один из ожидаемых dependency components;
- alert expression покрыт promtool fixture;
- e2e подтверждает runtime metric path.

### Cycle 13: RabbitMQQueueBacklog

RED:

- scenario создает backlog и ожидает RabbitMQ queue depth metric.

GREEN:

- выбрать один детерминированный механизм:
  - pause worker через e2e hook; или
  - compose orchestration для остановки worker; или
  - publish messages через RabbitMQ Management API в test queue/routing key, если это соответствует текущему topology.

Acceptance:

- `rabbitmq_queue_messages{queue=~"avatars\\.uploads|avatars\\.deletes"}` появляется;
- promtool fixture подтверждает threshold behavior;
- e2e не оставляет stack в paused state после завершения.

### Cycle 14: WorkerProcessingFailures

RED:

- scenario вызывает worker failure и ожидает `avatar_worker_messages_total{status="error"}`.

GREEN:

- добавить poison delivery publisher или gated worker failure hook;
- добавить cleanup/retry handling.

Acceptance:

- worker продолжает работу после test failure message;
- metric имеет expected `routing_key`;
- promtool fixture подтверждает alert behavior.

## E2E scenario list

Итоговый runner должен содержать scenarios:

- `prometheus readiness`
- `prometheus scrape targets`
- `server http metrics`
- `upload business metrics`
- `alert rules loaded`
- `alertmanager readiness`
- `alertmanager receives controlled alert`
- `high http error rate signal`
- `high upload error rate signal`
- `high response p95 latency signal`
- `upload latency critical signal`
- `dependency operation errors signal`
- `rabbitmq queue backlog signal`
- `worker processing failures signal`

Сценарии должны выполняться независимо настолько, насколько возможно. Если один сценарий создает runtime state, он обязан использовать уникальный test id/user id prefix и cleanup where practical.

## Acceptance criteria

- Новый e2e runner не импортирует `internal/` packages.
- Runner проверяет Prometheus и Alertmanager через HTTP API.
- Все 7 alert rules имеют `promtool` fixtures.
- E2E smoke подтверждает scrape, loaded rules, runtime metrics и Alertmanager wiring.
- Test-only hooks выключены по умолчанию и доступны только при explicit env flag.
- Обязательный быстрый gate не ждет production `for: 5m/10m`.
- Существующие `make contract-tests` и `make docker-contract-tests` остаются без изменений.
- `go test ./...` остается базовой проверкой.

## Verification commands после реализации

Минимальный набор:

```bash
go test ./...
promtool test rules configs/observability/prometheus/alert-rule-tests.yml
make docker-observability-up
make docker-observability-e2e
```

Если `promtool` недоступен локально, использовать официальный Prometheus container или добавить отдельный make target, который запускает `promtool` внутри container.

## Риски и ограничения

- Alert windows и scrape interval могут сделать прямой firing e2e медленным. Поэтому точность expressions проверяется через `promtool`, а runtime e2e проверяет wiring and series.
- Test-only hooks несут риск попадания в production runtime. Этот риск закрывается explicit env flag, отрицательными тестами и отсутствием hooks из обычной документации API.
- RabbitMQ backlog и worker failure зависят от topology. Перед реализацией этих cycles нужно подтвердить реальные queue names и routing keys в RabbitMQ adapter/worker tests.
- Invalid upload path может возвращать 4xx, но все равно должен увеличивать upload error metric; если текущий код меняет это поведение, сначала зафиксировать расхождение test-first.
