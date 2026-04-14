package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go-avatar-service/internal/domain"
	"go-avatar-service/internal/imageproc"
	"go-avatar-service/internal/service"
)

var errTemporary = errors.New("temporary")

type UploadEvent struct {
	AvatarID string `json:"avatar_id"`
}

type DeleteEvent struct {
	AvatarID string `json:"avatar_id"`
}

type UploadHandler struct {
	repo    service.Repository
	storage service.Storage
	log     *slog.Logger
}

func NewUploadHandler(repo service.Repository, storage service.Storage, log *slog.Logger) *UploadHandler {
	return &UploadHandler{repo: repo, storage: storage, log: log}
}

func (h *UploadHandler) Handle(ctx context.Context, event UploadEvent) error {
	a, err := h.repo.GetActiveByID(ctx, event.AvatarID)
	if err != nil {
		return nil
	}
	if a.Status == domain.StatusCompleted && a.Thumb100Available && a.Thumb300Available {
		h.log.Info("duplicate upload event", "avatar_id", event.AvatarID)
		return nil
	}
	data, _, err := h.storage.Get(ctx, a.OriginalKey)
	if err != nil {
		_ = h.repo.UpdateProcessingResult(ctx, a.ID, "", "")
		h.log.Error("missing original", "avatar_id", event.AvatarID, "error", err.Error())
		return nil
	}
	var thumb100, thumb300 []byte
	if err := Retry(ctx, 2, func() error {
		var err error
		thumb100, err = imageproc.ThumbnailJPEG(data, 100)
		return err
	}); err != nil {
		_ = h.repo.UpdateProcessingResult(ctx, a.ID, "", "")
		return nil
	}
	if err := Retry(ctx, 2, func() error {
		var err error
		thumb300, err = imageproc.ThumbnailJPEG(data, 300)
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
	h.log.Info("thumbnail generation success", "avatar_id", a.ID)
	return nil
}

type DeleteHandler struct {
	repo    service.Repository
	storage service.Storage
	log     *slog.Logger
}

func NewDeleteHandler(repo service.Repository, storage service.Storage, log *slog.Logger) *DeleteHandler {
	return &DeleteHandler{repo: repo, storage: storage, log: log}
}

func (h *DeleteHandler) Handle(ctx context.Context, event DeleteEvent) error {
	a, err := h.repo.GetByID(ctx, event.AvatarID)
	if err != nil {
		h.log.Info("duplicate delete event", "avatar_id", event.AvatarID)
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
	h.log.Info("delete executed", "avatar_id", event.AvatarID)
	return nil
}

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
