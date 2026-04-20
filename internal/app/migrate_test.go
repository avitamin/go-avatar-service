package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
)

func TestWriteMigrationStatus(t *testing.T) {
	tests := []struct {
		name    string
		version uint
		dirty   bool
		err     error
		want    string
		wantErr string
	}{
		{
			name: "pending",
			err:  migrate.ErrNilVersion,
			want: "migrate status pending\n",
		},
		{
			name:    "ok",
			version: 1,
			want:    "migrate status ok version=1 dirty=false\n",
		},
		{
			name:    "dirty",
			version: 2,
			dirty:   true,
			want:    "migrate status dirty version=2\n",
		},
		{
			name:    "error",
			err:     errors.New("boom"),
			wantErr: "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := writeMigrationStatus(&out, tt.version, tt.dirty, tt.err)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("writeMigrationStatus() error = %v", err)
			}
			if out.String() != tt.want {
				t.Fatalf("output = %q, want %q", out.String(), tt.want)
			}
		})
	}
}

func TestResolveMigrationsDirUsesOverride(t *testing.T) {
	tempDir := t.TempDir()
	override := filepath.Join(tempDir, "db-migrations")
	if err := os.MkdirAll(override, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := resolveMigrationsDir(override)
	if err != nil {
		t.Fatalf("resolveMigrationsDir() error = %v", err)
	}
	if got != override {
		t.Fatalf("resolveMigrationsDir() = %q, want %q", got, override)
	}
}

func TestFirstExistingDirectoryReturnsFirstExistingCandidate(t *testing.T) {
	tempDir := t.TempDir()
	first := filepath.Join(tempDir, "missing")
	second := filepath.Join(tempDir, "migrations")
	if err := os.MkdirAll(second, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := firstExistingDirectory([]string{first, second})
	if err != nil {
		t.Fatalf("firstExistingDirectory() error = %v", err)
	}
	if got != second {
		t.Fatalf("firstExistingDirectory() = %q, want %q", got, second)
	}
}

func TestFirstExistingDirectoryReturnsHelpfulErrorWhenNotFound(t *testing.T) {
	_, err := firstExistingDirectory([]string{filepath.Join(t.TempDir(), "missing")})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "migrations directory not found; set MIGRATIONS_DIR" {
		t.Fatalf("error = %q", err)
	}
}
