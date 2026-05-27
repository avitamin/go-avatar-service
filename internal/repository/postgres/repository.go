// Package postgres provides a PostgreSQL Repository implementation.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go-avatar-service/internal/domain"
	"go-avatar-service/internal/observability"
	"go-avatar-service/internal/service"
)

// Repository persists avatar metadata in PostgreSQL.
type Repository struct {
	pool    *pgxpool.Pool
	metrics *observability.Metrics
}

// Option customizes repository wiring.
type Option func(*Repository)

// WithObservability configures repository metrics and traces.
func WithObservability(metrics *observability.Metrics) Option {
	return func(r *Repository) {
		r.metrics = metrics
	}
}

// Open connects to PostgreSQL and returns a Repository.
func Open(ctx context.Context, dsn string, opts ...Option) (*Repository, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	r := &Repository{pool: pool}
	for _, opt := range opts {
		opt(r)
	}
	if err := r.HealthCheck(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return r, nil
}

// Close releases the underlying database connection pool.
func (r *Repository) Close() error {
	r.pool.Close()
	return nil
}

// HealthCheck verifies that PostgreSQL is reachable.
func (r *Repository) HealthCheck(ctx context.Context) (err error) {
	ctx, span, done := r.start(ctx, "HealthCheck")
	defer func() { done(err) }()
	err = r.pool.Ping(ctx)
	if err != nil {
		recordPostgresError(span, err)
	}
	return err
}

// Create inserts a new avatar metadata row.
func (r *Repository) Create(ctx context.Context, a *domain.Avatar) error {
	ctx, span, done := r.start(ctx, "Create")
	defer done(nil)
	_, err := r.pool.Exec(ctx, `
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
	if err != nil {
		recordPostgresError(span, err)
		done(err)
	}
	return err
}

// GetActiveByID returns a non-deleted avatar by identifier.
func (r *Repository) GetActiveByID(ctx context.Context, id string) (*domain.Avatar, error) {
	ctx, span, done := r.start(ctx, "GetActiveByID")
	defer done(nil)
	a, err := r.GetByID(ctx, id)
	if err != nil {
		recordPostgresError(span, err)
		done(err)
		return nil, err
	}
	if a.DeletedAt != nil {
		recordPostgresError(span, service.ErrNotFound)
		done(service.ErrNotFound)
		return nil, service.ErrNotFound
	}
	return a, nil
}

// GetByID returns an avatar by identifier, including soft-deleted rows.
func (r *Repository) GetByID(ctx context.Context, id string) (*domain.Avatar, error) {
	ctx, span, done := r.start(ctx, "GetByID")
	defer done(nil)
	a, err := r.scanOne(ctx, `SELECT `+avatarColumns+` FROM avatars WHERE id = $1`, id)
	if err != nil {
		recordPostgresError(span, err)
		done(err)
	}
	return a, err
}

// ListActiveByUser returns active avatars for a user sorted by creation time.
func (r *Repository) ListActiveByUser(ctx context.Context, userID string) ([]domain.Avatar, error) {
	ctx, span, done := r.start(ctx, "ListActiveByUser")
	defer done(nil)
	rows, err := r.pool.Query(ctx, `
		SELECT `+avatarColumns+`
		FROM avatars
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		recordPostgresError(span, err)
		done(err)
		return nil, err
	}
	defer rows.Close()

	var out []domain.Avatar
	for rows.Next() {
		a, err := scanAvatar(rows)
		if err != nil {
			recordPostgresError(span, err)
			done(err)
			return nil, err
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		recordPostgresError(span, err)
		done(err)
		return nil, err
	}
	return out, nil
}

// SoftDeleteByID marks an avatar deleted and updates its timestamp.
func (r *Repository) SoftDeleteByID(ctx context.Context, id string, deletedAt time.Time) error {
	ctx, span, done := r.start(ctx, "SoftDeleteByID")
	defer done(nil)
	res, err := r.pool.Exec(ctx, `
		UPDATE avatars
		SET deleted_at = $2, updated_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`, id, deletedAt)
	if err != nil {
		recordPostgresError(span, err)
		done(err)
		return err
	}
	err = requireAffected(res)
	if err != nil {
		recordPostgresError(span, err)
		done(err)
	}
	return err
}

// MarkPublishFailed marks an avatar as failed after broker publication errors.
func (r *Repository) MarkPublishFailed(ctx context.Context, id string) error {
	ctx, span, done := r.start(ctx, "MarkPublishFailed")
	defer done(nil)
	res, err := r.pool.Exec(ctx, `
		UPDATE avatars
		SET status = 'failed', updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		recordPostgresError(span, err)
		done(err)
		return err
	}
	err = requireAffected(res)
	if err != nil {
		recordPostgresError(span, err)
		done(err)
	}
	return err
}

// UpdateProcessingResult stores generated thumbnail keys and final processing status.
func (r *Repository) UpdateProcessingResult(ctx context.Context, id, thumb100, thumb300 string) error {
	ctx, span, done := r.start(ctx, "UpdateProcessingResult")
	defer done(nil)
	status := domain.StatusFailed
	if thumb100 != "" && thumb300 != "" {
		status = domain.StatusCompleted
	}
	res, err := r.pool.Exec(ctx, `
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
		recordPostgresError(span, err)
		done(err)
		return err
	}
	err = requireAffected(res)
	if err != nil {
		recordPostgresError(span, err)
		done(err)
	}
	return err
}

const avatarColumns = `
	id, user_id, file_name, original_mime_type, size_bytes,
	original_key, thumb100_key, thumb300_key,
	original_available, thumb100_available, thumb300_available,
	status, created_at, updated_at, deleted_at
`

func (r *Repository) scanOne(ctx context.Context, query string, args ...any) (*domain.Avatar, error) {
	row := r.pool.QueryRow(ctx, query, args...)
	a, err := scanAvatar(row)
	if errors.Is(err, pgx.ErrNoRows) {
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

func requireAffected(res pgconn.CommandTag) error {
	rows := res.RowsAffected()
	if rows == 0 {
		return service.ErrNotFound
	}
	return nil
}

func (r *Repository) start(ctx context.Context, operation string) (context.Context, trace.Span, func(error)) {
	start := time.Now()
	ctx, span := otel.Tracer("avatar-service/postgres").Start(ctx, "postgres "+operation)
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", operation),
		attribute.String("db.statement.name", operation),
	)
	called := false
	return ctx, span, func(err error) {
		if called {
			return
		}
		called = true
		status := "success"
		if err != nil {
			status = "error"
		}
		r.metrics.ObserveDependencyOperation("postgres", operation, status, time.Since(start))
		span.End()
	}
}

func recordPostgresError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
