// Package minio provides a MinIO-backed Storage implementation.
package minio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"go-avatar-service/internal/service"
)

// Storage stores avatar objects in a MinIO bucket.
type Storage struct {
	client *minio.Client
	bucket string
}

// Config defines the MinIO connection and bucket settings.
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

// Open connects to MinIO, ensures the bucket exists, and returns Storage.
func Open(ctx context.Context, cfg Config) (*Storage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}
	return &Storage{client: client, bucket: cfg.Bucket}, nil
}

// HealthCheck verifies that the configured bucket is accessible.
func (s *Storage) HealthCheck(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("bucket %q is not accessible", s.bucket)
	}
	return nil
}

// Put writes object bytes under key with the provided MIME type.
func (s *Storage) Put(ctx context.Context, key string, data []byte, mime string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: mime,
	})
	return err
}

// Get reads object bytes and MIME type for key.
func (s *Storage) Get(ctx context.Context, key string) ([]byte, string, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", mapError(err)
	}
	defer func() { _ = obj.Close() }()

	stat, err := obj.Stat()
	if err != nil {
		return nil, "", mapError(err)
	}
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, "", mapError(err)
	}
	return data, stat.ContentType, nil
}

// Delete removes key from the configured bucket.
func (s *Storage) Delete(ctx context.Context, key string) error {
	return mapError(s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}))
}

// Exists reports whether key is present in the configured bucket.
func (s *Storage) Exists(ctx context.Context, key string) bool {
	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
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
