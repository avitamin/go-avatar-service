package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go-avatar-service/internal/domain"
)

func BenchmarkPostgresCreate(b *testing.B) {
	ctx := context.Background()
	repo := benchmarkRepository(b, ctx)
	now := time.Unix(1700000000, 0).UTC()
	idPrefix := fmt.Sprintf("bench-create-%d", time.Now().UnixNano())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		avatar := benchmarkAvatar(fmt.Sprintf("%s-%d", idPrefix, i), "bench-user-create", now)
		if err := repo.Create(ctx, avatar); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPostgresGetByID(b *testing.B) {
	ctx := context.Background()
	repo := benchmarkRepository(b, ctx)
	now := time.Unix(1700000000, 0).UTC()
	avatar := benchmarkAvatar(fmt.Sprintf("bench-get-%d", time.Now().UnixNano()), "bench-user-get", now)
	if err := repo.Create(ctx, avatar); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := repo.GetByID(ctx, avatar.ID); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPostgresListActiveByUser(b *testing.B) {
	for _, count := range []int{10, 1000} {
		b.Run(fmt.Sprintf("records_%d", count), func(b *testing.B) {
			ctx := context.Background()
			repo := benchmarkRepository(b, ctx)
			now := time.Unix(1700000000, 0).UTC()
			userID := fmt.Sprintf("bench-user-list-%d-%d", count, time.Now().UnixNano())
			idPrefix := fmt.Sprintf("bench-list-%d-%d", count, time.Now().UnixNano())
			for i := 0; i < count; i++ {
				avatar := benchmarkAvatar(fmt.Sprintf("%s-%d", idPrefix, i), userID, now.Add(time.Duration(i)*time.Second))
				if err := repo.Create(ctx, avatar); err != nil {
					b.Fatal(err)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := repo.ListActiveByUser(ctx, userID); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func benchmarkRepository(b *testing.B, ctx context.Context) *Repository {
	b.Helper()
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		b.Skip("POSTGRES_DSN is not set")
	}
	repo, err := Open(ctx, dsn)
	if err != nil {
		b.Fatal(err)
	}
	if err := benchmarkEnsureSchema(ctx, repo); err != nil {
		_ = repo.Close()
		b.Fatal(err)
	}
	b.Cleanup(func() {
		_, _ = repo.db.ExecContext(context.Background(), `DELETE FROM avatars WHERE id LIKE 'bench-%' OR user_id LIKE 'bench-%'`)
		_ = repo.Close()
	})
	return repo
}

func benchmarkEnsureSchema(ctx context.Context, repo *Repository) error {
	_, err := repo.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS avatars (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			file_name TEXT NOT NULL,
			original_mime_type TEXT NOT NULL,
			size_bytes BIGINT NOT NULL,
			original_key TEXT NOT NULL,
			thumb100_key TEXT NOT NULL DEFAULT '',
			thumb300_key TEXT NOT NULL DEFAULT '',
			original_available BOOLEAN NOT NULL DEFAULT TRUE,
			thumb100_available BOOLEAN NOT NULL DEFAULT FALSE,
			thumb300_available BOOLEAN NOT NULL DEFAULT FALSE,
			status TEXT NOT NULL CHECK (status IN ('processing', 'completed', 'failed')),
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			deleted_at TIMESTAMPTZ
		);

		CREATE INDEX IF NOT EXISTS avatars_user_created_idx
			ON avatars (user_id, created_at DESC)
			WHERE deleted_at IS NULL;
	`)
	return err
}

func benchmarkAvatar(id, userID string, createdAt time.Time) *domain.Avatar {
	return &domain.Avatar{
		ID:                id,
		UserID:            userID,
		FileName:          "avatar.jpg",
		OriginalMimeType:  "image/jpeg",
		SizeBytes:         4,
		OriginalKey:       "avatars/" + id + "/original.jpg",
		OriginalAvailable: true,
		Status:            domain.StatusProcessing,
		CreatedAt:         createdAt,
		UpdatedAt:         createdAt,
	}
}
