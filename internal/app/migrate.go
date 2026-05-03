package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const migrationsDirEnv = "MIGRATIONS_DIR"

type migrationRunner struct {
	db       *sql.DB
	migrator *migrate.Migrate
}

// RunMigrate executes migration commands against PostgreSQL.
func RunMigrate(ctx context.Context, direction string, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}

	runner, err := newMigrationRunner(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = runner.Close() }()

	switch direction {
	case "up":
		err = runner.Up()
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		_, _ = fmt.Fprintln(out, "migrate up ok")
		return nil
	case "down":
		err = runner.Down()
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		_, _ = fmt.Fprintln(out, "migrate down ok")
		return nil
	case "status":
		version, dirty, err := runner.Version()
		return writeMigrationStatus(out, version, dirty, err)
	default:
		return fmt.Errorf("unknown migrate subcommand %q", direction)
	}
}

func newMigrationRunner(ctx context.Context) (*migrationRunner, error) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		return nil, errors.New("POSTGRES_DSN is required")
	}

	migrationsDir, err := resolveMigrationsDir(os.Getenv(migrationsDirEnv))
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	driver, err := migratepostgres.WithInstance(db, &migratepostgres.Config{})
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	migrator, err := migrate.NewWithDatabaseInstance(fileSourceURL(migrationsDir), "postgres", driver)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &migrationRunner{db: db, migrator: migrator}, nil
}

func (r *migrationRunner) Up() error {
	return r.migrator.Up()
}

func (r *migrationRunner) Down() error {
	return r.migrator.Down()
}

func (r *migrationRunner) Version() (uint, bool, error) {
	return r.migrator.Version()
}

func (r *migrationRunner) Close() error {
	if r == nil {
		return nil
	}

	var err error
	if r.migrator != nil {
		sourceErr, databaseErr := r.migrator.Close()
		err = errors.Join(sourceErr, databaseErr)
	}
	if r.db != nil {
		err = errors.Join(err, r.db.Close())
	}
	return err
}

func writeMigrationStatus(out io.Writer, version uint, dirty bool, err error) error {
	switch {
	case errors.Is(err, migrate.ErrNilVersion):
		_, _ = fmt.Fprintln(out, "migrate status pending")
		return nil
	case err != nil:
		return err
	case dirty:
		_, _ = fmt.Fprintf(out, "migrate status dirty version=%d\n", version)
		return nil
	default:
		_, _ = fmt.Fprintf(out, "migrate status ok version=%d dirty=false\n", version)
		return nil
	}
}

func resolveMigrationsDir(override string) (string, error) {
	candidates, err := migrationDirCandidates(override)
	if err != nil {
		return "", err
	}
	return firstExistingDirectory(candidates)
}

func migrationDirCandidates(override string) ([]string, error) {
	if override != "" {
		return []string{override}, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	candidates := []string{filepath.Join(cwd, "migrations")}
	executable, err := os.Executable()
	if err != nil {
		return candidates, nil
	}

	exeDir := filepath.Dir(executable)
	candidates = append(candidates,
		filepath.Join(exeDir, "migrations"),
		filepath.Join(exeDir, "..", "migrations"),
	)
	return candidates, nil
}

func firstExistingDirectory(candidates []string) (string, error) {
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		absolutePath, err := filepath.Abs(candidate)
		if err != nil {
			return "", err
		}
		if _, ok := seen[absolutePath]; ok {
			continue
		}
		seen[absolutePath] = struct{}{}

		info, err := os.Stat(absolutePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", err
		}
		if info.IsDir() {
			return absolutePath, nil
		}
	}

	return "", fmt.Errorf("migrations directory not found; set %s", migrationsDirEnv)
}

func fileSourceURL(dir string) string {
	return "file://" + filepath.ToSlash(dir)
}
