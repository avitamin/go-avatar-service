// Package app wires CLI commands to the service runtime.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"go-avatar-service/internal/broker/rabbitmq"
	httpapi "go-avatar-service/internal/http"
	"go-avatar-service/internal/observability"
	pgrepo "go-avatar-service/internal/repository/postgres"
	"go-avatar-service/internal/service"
	miniostore "go-avatar-service/internal/storage/minio"
	"go-avatar-service/internal/worker"
)

const healthCheckTimeout = 500 * time.Millisecond
const serverShutdownTimeout = 5 * time.Second

var signalContextFn = signalContext
var runServerFn = RunServer
var runWorkerFn = RunWorker
var newHTTPServer = func(addr string, handler http.Handler) httpServer {
	return &stdHTTPServer{
		server: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
	}
}

// Run executes the avatars-service CLI subcommands.
func Run(args []string, out io.Writer) error {
	if len(args) < 2 {
		return usage(out)
	}
	if out == nil {
		out = io.Discard
	}
	switch args[1] {
	case "server":
		if hasCheck(args) {
			_, _ = fmt.Fprintln(out, "server ok")
			return nil
		}
		ctx, stop := signalContextFn()
		defer stop()
		return runServerFn(ctx)
	case "worker":
		if hasCheck(args) {
			_, _ = fmt.Fprintln(out, "worker ok")
			return nil
		}
		ctx, stop := signalContextFn()
		defer stop()
		return runWorkerFn(ctx, out)
	case "migrate":
		if len(args) < 3 {
			return errors.New("migrate subcommand is required")
		}
		if hasCheck(args) {
			_, _ = fmt.Fprintf(out, "migrate %s ok\n", args[2])
			return nil
		}
		switch args[2] {
		case "up", "down", "status":
			return RunMigrate(context.Background(), args[2], out)
		default:
			return fmt.Errorf("unknown migrate subcommand %q", args[2])
		}
	default:
		return usage(out)
	}
}

// RunServer starts the HTTP server using environment-driven runtime adapters.
func RunServer(ctx context.Context) error {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	obsCfg := observability.ConfigFromEnv()
	shutdownTracing, err := observability.InitTracing(ctx, obsCfg)
	if err != nil {
		return err
	}
	defer func() { _ = shutdownTracing(context.Background()) }()
	shutdownLogging, err := observability.InitLogging(ctx, obsCfg)
	if err != nil {
		return err
	}
	defer func() { _ = shutdownLogging(context.Background()) }()
	metrics := observability.NewMetrics(nil)
	logger := observability.NewLogger(obsCfg.ServiceName, "server", os.Stdout)
	rateLimitCfg, err := rateLimitConfigFromEnv()
	if err != nil {
		return err
	}
	storeRuntime, err := newStoreFromEnv(ctx, metrics)
	if err != nil {
		return err
	}
	defer storeRuntime.close()
	brokerRuntime, err := newBrokerFromEnv(metrics)
	if err != nil {
		return err
	}
	defer brokerRuntime.close()
	svc := service.NewAvatarService(storeRuntime.repo, storeRuntime.storage, brokerRuntime.broker, service.WithObservability(metrics))
	server := newHTTPServer(addr, httpapi.NewRouter(
		svc,
		newServerHealthService(logger, storeRuntime, brokerRuntime),
		httpapi.WithObservability(observability.RouterOptions{Logger: logger, Metrics: metrics}),
		httpapi.WithRateLimiter(rateLimitCfg),
	))
	shutdownErrCh := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()
		shutdownErrCh <- server.Shutdown(shutdownCtx)
	}()
	logger.Info("starting avatars-service server", "addr", addr)
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		select {
		case shutdownErr := <-shutdownErrCh:
			return shutdownErr
		default:
		}
		return nil
	}
	return err
}

// RunWorker starts the worker process using environment-driven runtime adapters.
func RunWorker(ctx context.Context, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	obsCfg := observability.ConfigFromEnv()
	shutdownTracing, err := observability.InitTracing(ctx, obsCfg)
	if err != nil {
		return err
	}
	defer func() { _ = shutdownTracing(context.Background()) }()
	shutdownLogging, err := observability.InitLogging(ctx, obsCfg)
	if err != nil {
		return err
	}
	defer func() { _ = shutdownLogging(context.Background()) }()
	metrics := observability.NewMetrics(nil)
	logger := observability.NewLogger(obsCfg.ServiceName, "worker", os.Stdout)
	if _, err := circuitBreakerConfigFromEnv(); err != nil {
		return err
	}
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@localhost:5672/"
	}
	client, err := rabbitmq.Dial(rabbitURL, rabbitmq.WithObservability(metrics))
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	storeRuntime, err := newStoreFromEnv(ctx, metrics)
	if err != nil {
		return err
	}
	defer storeRuntime.close()
	metricsServer, stopMetrics := maybeStartMetricsServer(ctx, obsCfg.MetricsAddr, metrics)
	defer stopMetrics()
	runner := worker.NewRunner(
		client,
		worker.NewUploadHandler(storeRuntime.repo, storeRuntime.storage, logger, worker.WithHandlerObservability(metrics)),
		worker.NewDeleteHandler(storeRuntime.repo, storeRuntime.storage, logger, worker.WithHandlerObservability(metrics)),
		logger,
		worker.WithObservability(metrics),
	)
	_, _ = fmt.Fprintln(out, "worker started")
	err = runner.Run(ctx)
	if metricsServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()
		if shutdownErr := metricsServer.Shutdown(shutdownCtx); err == nil {
			err = shutdownErr
		}
	}
	return err
}

func usage(out io.Writer) error {
	_, _ = fmt.Fprintln(out, "usage: avatars-service server|worker|migrate")
	return errors.New("invalid command")
}

func hasCheck(args []string) bool {
	for _, arg := range args {
		if arg == "--check" {
			return true
		}
	}
	return false
}

type noopBroker struct{}

func (noopBroker) Publish(context.Context, string, []byte, string) error { return nil }

type storeRuntime struct {
	repo     service.Repository
	storage  service.Storage
	close    func()
	postgres service.ComponentProbe
	minio    service.ComponentProbe
}

type brokerRuntime struct {
	broker   service.Broker
	close    func()
	rabbitmq service.ComponentProbe
}

func newServerHealthService(logger *slog.Logger, store storeRuntime, broker brokerRuntime) service.RuntimeHealthChecker {
	return service.NewRuntimeHealthService(logger, healthCheckTimeout, service.RuntimeProbes{
		Postgres: store.postgres,
		Minio:    store.minio,
		RabbitMQ: broker.rabbitmq,
	})
}

func newBrokerFromEnv(metricsOpt ...*observability.Metrics) (brokerRuntime, error) {
	var metrics *observability.Metrics
	if len(metricsOpt) > 0 {
		metrics = metricsOpt[0]
	}
	circuitBreakerCfg, err := circuitBreakerConfigFromEnv()
	if err != nil {
		return brokerRuntime{}, err
	}
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		return brokerRuntime{
			broker:   noopBroker{},
			close:    func() {},
			rabbitmq: service.DegradedComponent("rabbitmq broker is running in noop mode"),
		}, nil
	}
	client, err := rabbitmq.Dial(rabbitURL, rabbitmq.WithObservability(metrics))
	if err != nil {
		return brokerRuntime{}, err
	}
	return brokerRuntime{
		broker:   service.NewGuardedBroker(client, service.NewCircuitBreaker(circuitBreakerCfg)),
		close:    func() { _ = client.Close() },
		rabbitmq: service.HealthyComponent(client.HealthCheck),
	}, nil
}

func newStoreFromEnv(ctx context.Context, metricsOpt ...*observability.Metrics) (storeRuntime, error) {
	var metrics *observability.Metrics
	if len(metricsOpt) > 0 {
		metrics = metricsOpt[0]
	}
	circuitBreakerCfg, err := circuitBreakerConfigFromEnv()
	if err != nil {
		return storeRuntime{}, err
	}
	dsn := os.Getenv("POSTGRES_DSN")
	minioCfg, hasMinio, err := minioConfigFromEnv()
	if err != nil {
		return storeRuntime{}, err
	}
	if dsn == "" && !hasMinio {
		return storeRuntime{
			repo:     service.NewMemoryRepository(),
			storage:  service.NewMemoryStorage(),
			close:    func() {},
			postgres: service.DegradedComponent("postgres repository is running in memory fallback mode"),
			minio:    service.DegradedComponent("minio storage is running in memory fallback mode"),
		}, nil
	}
	if dsn == "" || !hasMinio {
		return storeRuntime{}, errors.New("POSTGRES_DSN and MINIO_ENDPOINT/MINIO_ACCESS_KEY/MINIO_SECRET_KEY/MINIO_BUCKET must be configured together")
	}
	repo, err := pgrepo.Open(ctx, dsn, pgrepo.WithObservability(metrics))
	if err != nil {
		return storeRuntime{}, err
	}
	storage, err := miniostore.Open(ctx, minioCfg, miniostore.WithObservability(metrics))
	if err != nil {
		_ = repo.Close()
		return storeRuntime{}, err
	}
	return storeRuntime{
		repo:     service.NewGuardedRepository(repo, service.NewCircuitBreaker(circuitBreakerCfg)),
		storage:  service.NewGuardedStorage(storage, service.NewCircuitBreaker(circuitBreakerCfg)),
		close:    func() { _ = repo.Close() },
		postgres: service.HealthyComponent(repo.HealthCheck),
		minio:    service.HealthyComponent(storage.HealthCheck),
	}, nil
}

func rateLimitConfigFromEnv() (httpapi.RateLimitConfig, error) {
	enabled, err := parseBoolEnvDefault("RATE_LIMIT_ENABLED", true)
	if err != nil {
		return httpapi.RateLimitConfig{}, err
	}
	requestsPerSec, err := parseFloatEnvDefault("RATE_LIMIT_REQUESTS_PER_SECOND", 20)
	if err != nil {
		return httpapi.RateLimitConfig{}, err
	}
	burst, err := parseIntEnvDefault("RATE_LIMIT_BURST", 40)
	if err != nil {
		return httpapi.RateLimitConfig{}, err
	}
	trustForwardedHeaders, err := parseBoolEnvDefault("RATE_LIMIT_TRUST_FORWARDED_HEADERS", false)
	if err != nil {
		return httpapi.RateLimitConfig{}, err
	}
	return httpapi.RateLimitConfig{
		Enabled:               enabled,
		RequestsPerSec:        requestsPerSec,
		Burst:                 burst,
		TrustForwardedHeaders: trustForwardedHeaders,
	}, nil
}

func newDependencyCircuitBreaker() (*service.CircuitBreaker, error) {
	cfg, err := circuitBreakerConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return service.NewCircuitBreaker(cfg), nil
}

func circuitBreakerConfigFromEnv() (service.CircuitBreakerConfig, error) {
	enabled, err := parseBoolEnvDefault("CIRCUIT_BREAKER_ENABLED", true)
	if err != nil {
		return service.CircuitBreakerConfig{}, err
	}
	failureThreshold, err := parseIntEnvDefault("CIRCUIT_BREAKER_FAILURE_THRESHOLD", 5)
	if err != nil {
		return service.CircuitBreakerConfig{}, err
	}
	openTimeoutSeconds, err := parseIntEnvDefault("CIRCUIT_BREAKER_OPEN_TIMEOUT_SECONDS", 30)
	if err != nil {
		return service.CircuitBreakerConfig{}, err
	}
	return service.CircuitBreakerConfig{
		Enabled:          enabled,
		FailureThreshold: failureThreshold,
		OpenTimeout:      time.Duration(openTimeoutSeconds) * time.Second,
		IsFailure:        isDependencyFailure,
	}, nil
}

func isDependencyFailure(err error) bool {
	if err == nil {
		return false
	}
	return !errors.Is(err, service.ErrNotFound) &&
		!errors.Is(err, service.ErrForbidden) &&
		!errors.Is(err, service.ErrVariantNotReady) &&
		!errors.Is(err, service.ErrObjectNotFound)
}

func parseBoolEnvDefault(key string, fallback bool) (bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	if raw == "" {
		return false, fmt.Errorf("invalid %s: value must not be empty", key)
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("invalid %s: %w", key, err)
	}
	return value, nil
}

func parseFloatEnvDefault(key string, fallback float64) (float64, error) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	if raw == "" {
		return 0, fmt.Errorf("invalid %s: value must not be empty", key)
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("invalid %s: value must be greater than 0", key)
	}
	return value, nil
}

func parseIntEnvDefault(key string, fallback int) (int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	if raw == "" {
		return 0, fmt.Errorf("invalid %s: value must not be empty", key)
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("invalid %s: value must be greater than 0", key)
	}
	return value, nil
}

func minioConfigFromEnv() (miniostore.Config, bool, error) {
	cfg := miniostore.Config{
		Endpoint:  os.Getenv("MINIO_ENDPOINT"),
		AccessKey: os.Getenv("MINIO_ACCESS_KEY"),
		SecretKey: os.Getenv("MINIO_SECRET_KEY"),
		Bucket:    os.Getenv("MINIO_BUCKET"),
	}
	rawUseSSL := os.Getenv("MINIO_USE_SSL")
	if rawUseSSL != "" {
		useSSL, err := strconv.ParseBool(rawUseSSL)
		if err != nil {
			return cfg, false, fmt.Errorf("invalid MINIO_USE_SSL: %w", err)
		}
		cfg.UseSSL = useSSL
	}
	hasAny := cfg.Endpoint != "" || cfg.AccessKey != "" || cfg.SecretKey != "" || cfg.Bucket != "" || rawUseSSL != ""
	hasAll := cfg.Endpoint != "" && cfg.AccessKey != "" && cfg.SecretKey != "" && cfg.Bucket != ""
	if hasAny && !hasAll {
		return cfg, false, errors.New("MINIO_ENDPOINT, MINIO_ACCESS_KEY, MINIO_SECRET_KEY and MINIO_BUCKET are required together")
	}
	return cfg, hasAll, nil
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

type httpServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

type stdHTTPServer struct {
	server *http.Server
}

func (s *stdHTTPServer) ListenAndServe() error {
	return s.server.ListenAndServe()
}

func (s *stdHTTPServer) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func maybeStartMetricsServer(ctx context.Context, addr string, metrics *observability.Metrics) (*http.Server, func()) {
	if addr == "" {
		return nil, func() {}
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	server := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("worker metrics server failed", "error", err.Error())
		}
	}()
	return server, func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}
}
