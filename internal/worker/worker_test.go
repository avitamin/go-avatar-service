package worker

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"testing"

	"go-avatar-service/internal/domain"
	"go-avatar-service/internal/service"
)

func TestUploadHandlerSuccessDuplicateMissingOriginalAndRetry(t *testing.T) {
	ctx := context.Background()
	repo := service.NewMemoryRepository()
	storage := service.NewMemoryStorage()
	svc := service.NewAvatarService(repo, storage, nil)
	created, _ := svc.Upload(ctx, service.UploadInput{UserID: "user-1", FileName: "a.jpg", Content: encodeJPEG(t), ContentType: "image/jpeg"})

	var logs bytes.Buffer
	h := NewUploadHandler(repo, storage, slog.New(slog.NewJSONHandler(&logs, nil)))
	if err := h.Handle(ctx, UploadEvent{AvatarID: created.ID}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	a, _ := repo.GetActiveByID(ctx, created.ID)
	if a.Status != domain.StatusCompleted || !a.Thumb100Available || !a.Thumb300Available {
		t.Fatalf("avatar after upload worker = %+v, want completed thumbnails", a)
	}
	if _, mime, err := storage.Get(ctx, a.Thumb100Key); err != nil || mime != "image/jpeg" {
		t.Fatalf("thumb100 mime=%q err=%v, want image/jpeg", mime, err)
	}
	if err := h.Handle(ctx, UploadEvent{AvatarID: created.ID}); err != nil {
		t.Fatalf("duplicate Handle() error = %v", err)
	}
	if !bytes.Contains(logs.Bytes(), []byte("duplicate upload event")) {
		t.Fatalf("duplicate was not logged: %s", logs.String())
	}

	missing, _ := svc.Upload(ctx, service.UploadInput{UserID: "user-1", FileName: "b.jpg", Content: encodeJPEG(t), ContentType: "image/jpeg"})
	storage.Delete(ctx, "avatars/"+missing.ID+"/original.jpg")
	if err := h.Handle(ctx, UploadEvent{AvatarID: missing.ID}); err != nil {
		t.Fatalf("missing original Handle() error = %v", err)
	}
	a, _ = repo.GetActiveByID(ctx, missing.ID)
	if a.Status != domain.StatusFailed {
		t.Fatalf("missing original status = %q, want failed", a.Status)
	}

	calls := 0
	err := Retry(ctx, 3, func() error {
		calls++
		if calls < 2 {
			return errTemporary
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("Retry() err=%v calls=%d, want nil and 2", err, calls)
	}
}

func TestDeleteHandlerIdempotentPhysicalDelete(t *testing.T) {
	ctx := context.Background()
	repo := service.NewMemoryRepository()
	storage := service.NewMemoryStorage()
	svc := service.NewAvatarService(repo, storage, nil)
	created, _ := svc.Upload(ctx, service.UploadInput{UserID: "user-1", FileName: "a.jpg", Content: encodeJPEG(t), ContentType: "image/jpeg"})
	hUpload := NewUploadHandler(repo, storage, slog.Default())
	_ = hUpload.Handle(ctx, UploadEvent{AvatarID: created.ID})
	if err := svc.DeleteByID(ctx, created.ID, "user-1"); err != nil {
		t.Fatalf("DeleteByID() error = %v", err)
	}
	a, _ := repo.GetByID(ctx, created.ID)
	h := NewDeleteHandler(repo, storage, slog.Default())
	if err := h.Handle(ctx, DeleteEvent{AvatarID: created.ID}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if storage.Exists(ctx, a.OriginalKey) || storage.Exists(ctx, a.Thumb100Key) || storage.Exists(ctx, a.Thumb300Key) {
		t.Fatal("objects must be physically deleted by worker")
	}
	if err := h.Handle(ctx, DeleteEvent{AvatarID: created.ID}); err != nil {
		t.Fatalf("duplicate delete Handle() error = %v", err)
	}
}

func TestRunnerDispatchesUploadDeleteAndAcks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	repo := service.NewMemoryRepository()
	storage := service.NewMemoryStorage()
	svc := service.NewAvatarService(repo, storage, nil)
	created, _ := svc.Upload(ctx, service.UploadInput{UserID: "user-1", FileName: "a.jpg", Content: encodeJPEG(t), ContentType: "image/jpeg"})
	if err := svc.DeleteByID(ctx, created.ID, "user-1"); err != nil {
		t.Fatalf("DeleteByID() error = %v", err)
	}
	a, _ := repo.GetByID(ctx, created.ID)
	deliveries := make(chan Delivery, 2)
	consumer := &stubConsumer{deliveries: deliveries}
	acked := 0
	deliveries <- Delivery{RoutingKey: RoutingKeyUploaded, Body: []byte(`{"avatar_id":"` + created.ID + `"}`), Ack: func() error {
		acked++
		return nil
	}}
	deliveries <- Delivery{RoutingKey: RoutingKeyDeleteRequested, Body: []byte(created.ID), Ack: func() error {
		acked++
		cancel()
		return nil
	}}
	close(deliveries)

	runner := NewRunner(
		consumer,
		NewUploadHandler(repo, storage, slog.Default()),
		NewDeleteHandler(repo, storage, slog.Default()),
		slog.Default(),
	)
	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if acked != 2 {
		t.Fatalf("acked = %d, want 2", acked)
	}
	if storage.Exists(context.Background(), a.OriginalKey) {
		t.Fatal("delete event must physically remove original")
	}
}

func TestRunnerNacksHandlerErrors(t *testing.T) {
	ctx := context.Background()
	deliveries := make(chan Delivery, 1)
	consumer := &stubConsumer{deliveries: deliveries}
	nacked := false
	deliveries <- Delivery{RoutingKey: RoutingKeyUploaded, Body: nil, Nack: func(requeue bool) error {
		nacked = requeue
		return nil
	}}
	close(deliveries)

	runner := NewRunner(
		consumer,
		NewUploadHandler(service.NewMemoryRepository(), service.NewMemoryStorage(), slog.Default()),
		NewDeleteHandler(service.NewMemoryRepository(), service.NewMemoryStorage(), slog.Default()),
		slog.Default(),
	)
	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !nacked {
		t.Fatal("handler error must nack with requeue")
	}
}

type stubConsumer struct {
	deliveries <-chan Delivery
	closed     bool
}

func (c *stubConsumer) Consume(context.Context) (<-chan Delivery, error) { return c.deliveries, nil }
func (c *stubConsumer) Close() error {
	c.closed = true
	return nil
}

func encodeJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 20), G: uint8(y * 20), B: 180, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
