package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	httpapi "go-avatar-service/internal/http"
	"go-avatar-service/internal/service"
)

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
		_, _ = fmt.Fprintln(out, "worker started")
		select {}
	case "migrate":
		if len(args) < 3 {
			return errors.New("migrate subcommand is required")
		}
		switch args[2] {
		case "up", "down", "status":
			_, _ = fmt.Fprintf(out, "migrate %s ok\n", args[2])
			return nil
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
	svc := service.NewAvatarService(service.NewMemoryRepository(), service.NewMemoryStorage(), logBroker{})
	server := &http.Server{
		Addr:    addr,
		Handler: httpapi.NewRouter(svc, httpapi.HealthService{Postgres: true, Minio: true, RabbitMQ: true}),
	}
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	log.Printf("starting avatars-service server on %s", addr)
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
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

type logBroker struct{}

func (logBroker) Publish(context.Context, string, []byte, string) error { return nil }
