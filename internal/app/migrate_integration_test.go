package app

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const migrateIntegrationDSNEnv = "MIGRATE_TEST_POSTGRES_DSN"

func TestRunMigratePostgresIntegration(t *testing.T) {
	baseDSN := os.Getenv(migrateIntegrationDSNEnv)
	if baseDSN == "" {
		t.Skipf("%s is not set", migrateIntegrationDSNEnv)
	}

	migrationsDir := repoMigrationsDir(t)

	t.Run("status up repeated up down status", func(t *testing.T) {
		schemaDSN := prepareMigrationTestSchema(t, baseDSN)
		t.Setenv("POSTGRES_DSN", schemaDSN)
		t.Setenv(migrationsDirEnv, migrationsDir)

		assertRunMigrateOutput(t, schemaDSN, migrationsDir, "status", "migrate status pending\n")
		assertRunMigrateOutput(t, schemaDSN, migrationsDir, "up", "migrate up ok\n")
		assertRunMigrateOutput(t, schemaDSN, migrationsDir, "up", "migrate up ok\n")
		assertRunMigrateOutput(t, schemaDSN, migrationsDir, "status", "migrate status ok version=1 dirty=false\n")
		assertRunMigrateOutput(t, schemaDSN, migrationsDir, "down", "migrate down ok\n")
		assertRunMigrateOutput(t, schemaDSN, migrationsDir, "down", "migrate down ok\n")
		assertRunMigrateOutput(t, schemaDSN, migrationsDir, "status", "migrate status pending\n")
	})

	t.Run("existing schema without metadata is adopted on up", func(t *testing.T) {
		schemaDSN := prepareMigrationTestSchema(t, baseDSN)
		execMigrationSQL(t, schemaDSN, filepath.Join(migrationsDir, "001_init.up.sql"))

		assertRunMigrateOutput(t, schemaDSN, migrationsDir, "up", "migrate up ok\n")
		assertRunMigrateOutput(t, schemaDSN, migrationsDir, "status", "migrate status ok version=1 dirty=false\n")
	})

	t.Run("dirty status is reported explicitly", func(t *testing.T) {
		schemaDSN := prepareMigrationTestSchema(t, baseDSN)
		execSQL(t, schemaDSN, `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version bigint NOT NULL PRIMARY KEY,
				dirty boolean NOT NULL
			);
			INSERT INTO schema_migrations (version, dirty) VALUES (2, true);
		`)

		assertRunMigrateOutput(t, schemaDSN, migrationsDir, "status", "migrate status dirty version=2\n")
	})
}

func assertRunMigrateOutput(t *testing.T, dsn, migrationsDir, direction, want string) {
	t.Helper()
	t.Setenv("POSTGRES_DSN", dsn)
	t.Setenv(migrationsDirEnv, migrationsDir)

	var out bytes.Buffer
	if err := RunMigrate(context.Background(), direction, &out); err != nil {
		t.Fatalf("RunMigrate(%q) error = %v", direction, err)
	}
	if out.String() != want {
		t.Fatalf("RunMigrate(%q) output = %q, want %q", direction, out.String(), want)
	}
}

func prepareMigrationTestSchema(t *testing.T, baseDSN string) string {
	t.Helper()

	adminDB := openTestDB(t, baseDSN)
	schemaName := "migrate_test_" + strings.ReplaceAll(t.Name(), "/", "_")
	schemaName = strings.ToLower(strings.ReplaceAll(schemaName, " ", "_"))
	if _, err := adminDB.Exec(`CREATE SCHEMA "` + schemaName + `"`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := adminDB.Exec(`DROP SCHEMA IF EXISTS "` + schemaName + `" CASCADE`); err != nil {
			t.Fatalf("drop schema: %v", err)
		}
	})

	return dsnWithSearchPath(t, baseDSN, schemaName)
}

func openTestDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("db.Close: %v", err)
		}
	})

	return db
}

func execMigrationSQL(t *testing.T, dsn, path string) {
	t.Helper()

	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	execSQL(t, dsn, string(sqlBytes))
}

func execSQL(t *testing.T, dsn, statement string) {
	t.Helper()

	db := openTestDB(t, dsn)
	if _, err := db.Exec(statement); err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
}

func dsnWithSearchPath(t *testing.T, dsn, schema string) string {
	t.Helper()

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", dsn, err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func repoMigrationsDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "migrations"))
}

func TestMigrationDirCandidatesIncludeExecutableParentFallback(t *testing.T) {
	tempDir := t.TempDir()
	executable := filepath.Join(tempDir, "bin", "avatars-service")
	if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
		t.Fatal(err)
	}

	candidates := []string{
		filepath.Join(tempDir, "cwd", "migrations"),
		filepath.Join(filepath.Dir(executable), "migrations"),
		filepath.Join(filepath.Dir(executable), "..", "migrations"),
	}
	want := filepath.Clean(filepath.Join(tempDir, "bin", "..", "migrations"))
	if got := filepath.Clean(candidates[2]); got != want {
		t.Fatalf("candidate = %q, want %q", got, want)
	}
}

func TestRunMigrateRequiresPostgresDSN(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "")

	err := RunMigrate(context.Background(), "status", &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "POSTGRES_DSN is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestFileSourceURLUsesAbsolutePath(t *testing.T) {
	abs := filepath.Join(string(filepath.Separator), "tmp", "migrations")
	got := fileSourceURL(abs)
	want := fmt.Sprintf("file://%s", filepath.ToSlash(abs))
	if got != want {
		t.Fatalf("fileSourceURL() = %q, want %q", got, want)
	}
}
