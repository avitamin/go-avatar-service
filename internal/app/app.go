package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"go-avatar-service/internal/broker/rabbitmq"
	httpapi "go-avatar-service/internal/http"
	pgrepo "go-avatar-service/internal/repository/postgres"
	"go-avatar-service/internal/service"
	miniostore "go-avatar-service/internal/storage/minio"
	"go-avatar-service/internal/worker"
)

const healthCheckTimeout = 500 * time.Millisecond

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
		return RunServer(context.Background())
	case "worker":
		if hasCheck(args) {
			_, _ = fmt.Fprintln(out, "worker ok")
			return nil
		}
		ctx, stop := signalContext()
		defer stop()
		return RunWorker(ctx, out)
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

func RunServer(ctx context.Context) error {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	storeRuntime, err := newStoreFromEnv(ctx)
	if err != nil {
		return err
	}
	defer storeRuntime.close()
	brokerRuntime, err := newBrokerFromEnv()
	if err != nil {
		return err
	}
	defer brokerRuntime.close()
	svc := service.NewAvatarService(storeRuntime.repo, storeRuntime.storage, brokerRuntime.broker)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	server := &http.Server{
		Addr:    addr,
		Handler: httpapi.NewRouter(svc, newServerHealthService(logger, storeRuntime, brokerRuntime)),
	}
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	log.Printf("starting avatars-service server on %s", addr)
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func RunWorker(ctx context.Context, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@localhost:5672/"
	}
	client, err := rabbitmq.Dial(rabbitURL)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	storeRuntime, err := newStoreFromEnv(ctx)
	if err != nil {
		return err
	}
	defer storeRuntime.close()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	runner := worker.NewRunner(
		client,
		worker.NewUploadHandler(storeRuntime.repo, storeRuntime.storage, logger),
		worker.NewDeleteHandler(storeRuntime.repo, storeRuntime.storage, logger),
		logger,
	)
	_, _ = fmt.Fprintln(out, "worker started")
	return runner.Run(ctx)
}

func RunMigrate(ctx context.Context, direction string, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		return errors.New("POSTGRES_DSN is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	switch direction {
	case "up":
		return executeMigration(ctx, db, "migrations/001_init.up.sql", out, "migrate up ok")
	case "down":
		return executeMigration(ctx, db, "migrations/001_init.down.sql", out, "migrate down ok")
	case "status":
		var exists bool
		if err := db.QueryRowContext(ctx, "SELECT to_regclass('public.avatars') IS NOT NULL").Scan(&exists); err != nil {
			return err
		}
		if exists {
			_, _ = fmt.Fprintln(out, "migrate status ok")
		} else {
			_, _ = fmt.Fprintln(out, "migrate status pending")
		}
		return nil
	default:
		return fmt.Errorf("unknown migrate subcommand %q", direction)
	}
}

func executeMigration(ctx context.Context, db *sql.DB, path string, out io.Writer, message string) error {
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, string(sqlBytes)); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out, message)
	return nil
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

type logBroker struct{}

func (logBroker) Publish(context.Context, string, []byte, string) error { return nil }

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

func newBrokerFromEnv() (brokerRuntime, error) {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		return brokerRuntime{
			broker:   logBroker{},
			close:    func() {},
			rabbitmq: service.DegradedComponent("rabbitmq broker is running in noop log mode"),
		}, nil
	}
	client, err := rabbitmq.Dial(rabbitURL)
	if err != nil {
		return brokerRuntime{}, err
	}
	return brokerRuntime{
		broker:   client,
		close:    func() { _ = client.Close() },
		rabbitmq: service.HealthyComponent(client.HealthCheck),
	}, nil
}

func newStoreFromEnv(ctx context.Context) (storeRuntime, error) {
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
	repo, err := pgrepo.Open(ctx, dsn)
	if err != nil {
		return storeRuntime{}, err
	}
	storage, err := miniostore.Open(ctx, minioCfg)
	if err != nil {
		_ = repo.Close()
		return storeRuntime{}, err
	}
	return storeRuntime{
		repo:     repo,
		storage:  storage,
		close:    func() { _ = repo.Close() },
		postgres: service.HealthyComponent(repo.HealthCheck),
		minio:    service.HealthyComponent(storage.HealthCheck),
	}, nil
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
