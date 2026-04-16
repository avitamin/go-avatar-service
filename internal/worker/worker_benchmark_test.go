package worker

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"testing"

	"go-avatar-service/internal/service"
)

func BenchmarkUploadHandlerGenerateThumbnails(b *testing.B) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	imageData := benchmarkWorkerJPEG(b)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		repo := service.NewMemoryRepository()
		storage := service.NewMemoryStorage()
		svc := service.NewAvatarService(repo, storage, nil)
		created, err := svc.Upload(ctx, service.UploadInput{
			UserID:      "user-1",
			FileName:    "avatar.jpg",
			Content:     imageData,
			ContentType: "image/jpeg",
		})
		if err != nil {
			b.Fatal(err)
		}
		handler := NewUploadHandler(repo, storage, slog.Default())
		if err := handler.Handle(ctx, UploadEvent{AvatarID: created.ID}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRetrySuccess(b *testing.B) {
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := Retry(ctx, 3, func() error { return nil }); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkWorkerJPEG(b *testing.B) []byte {
	b.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 180, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		b.Fatal(err)
	}
	return buf.Bytes()
}
