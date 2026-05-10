package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go-avatar-service/internal/domain"
	"go-avatar-service/internal/imageproc"
	"go-avatar-service/internal/observability"
	"go-avatar-service/internal/service"
)

var errTemporary = errors.New("temporary")

// UploadEvent requests thumbnail generation for an avatar.
type UploadEvent struct {
	AvatarID string `json:"avatar_id"`
}

// DeleteEvent requests removal of avatar objects from storage.
type DeleteEvent struct {
	AvatarID string `json:"avatar_id"`
}

// UploadHandler builds thumbnails for uploaded avatars.
type UploadHandler struct {
	repo    service.Repository
	storage service.Storage
	log     *slog.Logger
	metrics *observability.Metrics
}

// HandlerOption customizes worker handlers.
type HandlerOption func(*handlerConfig)

type handlerConfig struct {
	metrics *observability.Metrics
}

// WithHandlerObservability configures worker handler metrics.
func WithHandlerObservability(metrics *observability.Metrics) HandlerOption {
	return func(cfg *handlerConfig) {
		cfg.metrics = metrics
	}
}

// NewUploadHandler creates an UploadHandler backed by repository and storage ports.
func NewUploadHandler(repo service.Repository, storage service.Storage, log *slog.Logger, opts ...HandlerOption) *UploadHandler {
	var cfg handlerConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return &UploadHandler{repo: repo, storage: storage, log: log, metrics: cfg.metrics}
}

// Handle processes an upload event and stores generated thumbnail variants.
func (h *UploadHandler) Handle(ctx context.Context, event UploadEvent) error {
	a, err := h.repo.GetActiveByID(ctx, event.AvatarID)
	if err != nil {
		return nil
	}
	if a.Status == domain.StatusCompleted && a.Thumb100Available && a.Thumb300Available {
		h.log.InfoContext(ctx, "duplicate upload event", observability.Attrs(ctx, "avatar_id", event.AvatarID)...)
		return nil
	}
	data, _, err := h.storage.Get(ctx, a.OriginalKey)
	if err != nil {
		_ = h.repo.UpdateProcessingResult(ctx, a.ID, "", "")
		h.log.ErrorContext(ctx, "missing original", observability.Attrs(ctx, "avatar_id", event.AvatarID, "error", err.Error())...)
		return nil
	}
	var thumb100, thumb300 []byte
	if err := Retry(ctx, 2, func() error {
		var err error
		thumb100, err = imageproc.ThumbnailJPEG(data, 100)
		thumbStatus := "success"
		if err != nil {
			thumbStatus = "error"
		}
		h.metrics.IncWorkerThumbnail("100x100", thumbStatus)
		return err
	}); err != nil {
		_ = h.repo.UpdateProcessingResult(ctx, a.ID, "", "")
		return nil
	}
	if err := Retry(ctx, 2, func() error {
		var err error
		thumb300, err = imageproc.ThumbnailJPEG(data, 300)
		thumbStatus := "success"
		if err != nil {
			thumbStatus = "error"
		}
		h.metrics.IncWorkerThumbnail("300x300", thumbStatus)
		return err
	}); err != nil {
		_ = h.repo.UpdateProcessingResult(ctx, a.ID, "", "")
		return nil
	}
	key100 := "avatars/" + a.ID + "/thumb_100x100.jpg"
	key300 := "avatars/" + a.ID + "/thumb_300x300.jpg"
	if err := h.storage.Put(ctx, key100, thumb100, "image/jpeg"); err != nil {
		_ = h.repo.UpdateProcessingResult(ctx, a.ID, "", "")
		return nil
	}
	if err := h.storage.Put(ctx, key300, thumb300, "image/jpeg"); err != nil {
		_ = h.repo.UpdateProcessingResult(ctx, a.ID, "", "")
		return nil
	}
	if err := h.repo.UpdateProcessingResult(ctx, a.ID, key100, key300); err != nil {
		return err
	}
	h.metrics.SetStorageBytes("thumb_100x100", int64(len(thumb100)))
	h.metrics.SetStorageBytes("thumb_300x300", int64(len(thumb300)))
	h.log.InfoContext(ctx, "thumbnail generation success", observability.Attrs(ctx, "avatar_id", a.ID)...)
	return nil
}

// DeleteHandler removes avatar objects after a delete event.
type DeleteHandler struct {
	repo    service.Repository
	storage service.Storage
	log     *slog.Logger
	metrics *observability.Metrics
}

// NewDeleteHandler creates a DeleteHandler backed by repository and storage ports.
func NewDeleteHandler(repo service.Repository, storage service.Storage, log *slog.Logger, opts ...HandlerOption) *DeleteHandler {
	var cfg handlerConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return &DeleteHandler{repo: repo, storage: storage, log: log, metrics: cfg.metrics}
}

// Handle processes a delete event and removes known object keys.
func (h *DeleteHandler) Handle(ctx context.Context, event DeleteEvent) error {
	a, err := h.repo.GetByID(ctx, event.AvatarID)
	if err != nil {
		h.log.InfoContext(ctx, "duplicate delete event", observability.Attrs(ctx, "avatar_id", event.AvatarID)...)
		return nil
	}
	for _, key := range []string{a.OriginalKey, a.Thumb100Key, a.Thumb300Key} {
		if key == "" {
			continue
		}
		if err := h.storage.Delete(ctx, key); err != nil {
			return err
		}
	}
	h.log.InfoContext(ctx, "delete executed", observability.Attrs(ctx, "avatar_id", event.AvatarID)...)
	return nil
}

// Retry reruns fn with a short linear backoff until success or context cancellation.
func Retry(ctx context.Context, attempts int, fn func() error) error {
	if attempts < 1 {
		attempts = 1
	}
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(i+1) * time.Millisecond):
		}
	}
	return err
}
