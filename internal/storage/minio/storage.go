// Package minio provides a MinIO-backed Storage implementation.
package minio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go-avatar-service/internal/observability"
	"go-avatar-service/internal/service"
)

// Storage stores avatar objects in a MinIO bucket.
type Storage struct {
	client  *minio.Client
	bucket  string
	metrics *observability.Metrics
}

// Config defines the MinIO connection and bucket settings.
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

// Option customizes storage wiring.
type Option func(*Storage)

// WithObservability configures storage metrics and traces.
func WithObservability(metrics *observability.Metrics) Option {
	return func(s *Storage) {
		s.metrics = metrics
	}
}

// Open connects to MinIO, ensures the bucket exists, and returns Storage.
func Open(ctx context.Context, cfg Config, opts ...Option) (*Storage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	storage := &Storage{client: client, bucket: cfg.Bucket}
	for _, opt := range opts {
		opt(storage)
	}
	ctx, span, done := storage.start(ctx, "Open", "")
	defer done(nil)
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		recordMinioError(span, err)
		done(err)
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			recordMinioError(span, err)
			done(err)
			return nil, err
		}
	}
	return storage, nil
}

// HealthCheck verifies that the configured bucket is accessible.
func (s *Storage) HealthCheck(ctx context.Context) error {
	ctx, span, done := s.start(ctx, "HealthCheck", "")
	defer done(nil)
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		recordMinioError(span, err)
		done(err)
		return err
	}
	if !exists {
		err := fmt.Errorf("bucket %q is not accessible", s.bucket)
		recordMinioError(span, err)
		done(err)
		return err
	}
	return nil
}

// Put writes object bytes under key with the provided MIME type.
func (s *Storage) Put(ctx context.Context, key string, data []byte, mime string) error {
	ctx, span, done := s.start(ctx, "Put", key)
	defer done(nil)
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: mime,
	})
	if err != nil {
		recordMinioError(span, err)
		done(err)
	}
	return err
}

// Get reads object bytes and MIME type for key.
func (s *Storage) Get(ctx context.Context, key string) ([]byte, string, error) {
	ctx, span, done := s.start(ctx, "Get", key)
	defer done(nil)
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		err = mapError(err)
		recordMinioError(span, err)
		done(err)
		return nil, "", err
	}
	defer func() { _ = obj.Close() }()

	stat, err := obj.Stat()
	if err != nil {
		err = mapError(err)
		recordMinioError(span, err)
		done(err)
		return nil, "", err
	}
	data, err := io.ReadAll(obj)
	if err != nil {
		err = mapError(err)
		recordMinioError(span, err)
		done(err)
		return nil, "", err
	}
	return data, stat.ContentType, nil
}

// Delete removes key from the configured bucket.
func (s *Storage) Delete(ctx context.Context, key string) error {
	ctx, span, done := s.start(ctx, "Delete", key)
	defer done(nil)
	err := mapError(s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}))
	if err != nil {
		recordMinioError(span, err)
		done(err)
	}
	return err
}

// Exists reports whether key is present in the configured bucket.
func (s *Storage) Exists(ctx context.Context, key string) bool {
	ctx, _, done := s.start(ctx, "Exists", key)
	defer done(nil)
	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		done(err)
	}
	return err == nil
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	var resp minio.ErrorResponse
	if errors.As(err, &resp) && resp.Code == "NoSuchKey" {
		return service.ErrObjectNotFound
	}
	return err
}

func (s *Storage) start(ctx context.Context, operation, key string) (context.Context, trace.Span, func(error)) {
	start := time.Now()
	ctx, span := otel.Tracer("avatar-service/minio").Start(ctx, "minio "+operation)
	attrs := []attribute.KeyValue{
		attribute.String("storage.system", "s3"),
		attribute.String("s3.bucket", s.bucket),
		attribute.String("s3.operation", operation),
	}
	if key != "" {
		attrs = append(attrs, attribute.String("object.key_hash", safeKeyHash(key)))
	}
	span.SetAttributes(attrs...)
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
		s.metrics.ObserveDependencyOperation("minio", operation, status, time.Since(start))
		span.End()
	}
}

func recordMinioError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func safeKeyHash(key string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return fmt.Sprintf("%x", h.Sum64())
}
