package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"go-avatar-service/internal/domain"
	"go-avatar-service/internal/service"
)

type Repository struct {
	db *sql.DB
}

func Open(ctx context.Context, dsn string) (*Repository, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Repository{db: db}, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) HealthCheck(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *Repository) Create(ctx context.Context, a *domain.Avatar) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO avatars (
			id, user_id, file_name, original_mime_type, size_bytes,
			original_key, thumb100_key, thumb300_key,
			original_available, thumb100_available, thumb300_available,
			status, created_at, updated_at, deleted_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`,
		a.ID, a.UserID, a.FileName, a.OriginalMimeType, a.SizeBytes,
		a.OriginalKey, a.Thumb100Key, a.Thumb300Key,
		a.OriginalAvailable, a.Thumb100Available, a.Thumb300Available,
		a.Status, a.CreatedAt, a.UpdatedAt, a.DeletedAt,
	)
	return err
}

func (r *Repository) GetActiveByID(ctx context.Context, id string) (*domain.Avatar, error) {
	a, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.DeletedAt != nil {
		return nil, service.ErrNotFound
	}
	return a, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*domain.Avatar, error) {
	return r.scanOne(ctx, `SELECT `+avatarColumns+` FROM avatars WHERE id = $1`, id)
}

func (r *Repository) ListActiveByUser(ctx context.Context, userID string) ([]domain.Avatar, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+avatarColumns+`
		FROM avatars
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.Avatar
	for rows.Next() {
		a, err := scanAvatar(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (r *Repository) SoftDeleteByID(ctx context.Context, id string, deletedAt time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE avatars
		SET deleted_at = $2, updated_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`, id, deletedAt)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

func (r *Repository) MarkPublishFailed(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE avatars
		SET status = 'failed', updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

func (r *Repository) UpdateProcessingResult(ctx context.Context, id, thumb100, thumb300 string) error {
	status := domain.StatusFailed
	if thumb100 != "" && thumb300 != "" {
		status = domain.StatusCompleted
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE avatars
		SET thumb100_key = $2,
			thumb300_key = $3,
			thumb100_available = $2 <> '',
			thumb300_available = $3 <> '',
			status = $4,
			updated_at = NOW()
		WHERE id = $1
	`, id, thumb100, thumb300, status)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

const avatarColumns = `
	id, user_id, file_name, original_mime_type, size_bytes,
	original_key, thumb100_key, thumb300_key,
	original_available, thumb100_available, thumb300_available,
	status, created_at, updated_at, deleted_at
`

func (r *Repository) scanOne(ctx context.Context, query string, args ...any) (*domain.Avatar, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	a, err := scanAvatar(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrNotFound
	}
	return a, err
}

type scanner interface {
	Scan(...any) error
}

func scanAvatar(s scanner) (*domain.Avatar, error) {
	var a domain.Avatar
	if err := s.Scan(
		&a.ID, &a.UserID, &a.FileName, &a.OriginalMimeType, &a.SizeBytes,
		&a.OriginalKey, &a.Thumb100Key, &a.Thumb300Key,
		&a.OriginalAvailable, &a.Thumb100Available, &a.Thumb300Available,
		&a.Status, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	); err != nil {
		return nil, err
	}
	return &a, nil
}

func requireAffected(res sql.Result) error {
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return service.ErrNotFound
	}
	return nil
}
