package httpapi

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"go-avatar-service/internal/service"
)

func BenchmarkRouterHealth(b *testing.B) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc := service.NewAvatarService(service.NewMemoryRepository(), service.NewMemoryStorage(), noopBroker{})
	router := NewRouter(svc, HealthService{Postgres: true, Minio: true, RabbitMQ: true})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	}
}

func BenchmarkRouterReadAvatar(b *testing.B) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc := service.NewAvatarService(service.NewMemoryRepository(), service.NewMemoryStorage(), noopBroker{})
	created, err := svc.Upload(context.Background(), service.UploadInput{
		UserID:      "user-1",
		FileName:    "avatar.jpg",
		Content:     benchmarkJPEG(b),
		ContentType: "image/jpeg",
	})
	if err != nil {
		b.Fatal(err)
	}
	router := NewRouter(svc, HealthService{Postgres: true, Minio: true, RabbitMQ: true})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/"+created.ID, nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	}
}

func BenchmarkRouterUploadJPEG(b *testing.B) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc := service.NewAvatarService(service.NewMemoryRepository(), service.NewMemoryStorage(), noopBroker{})
	router := NewRouter(svc, HealthService{Postgres: true, Minio: true, RabbitMQ: true})
	body, contentType := benchmarkMultipartPayload(b, benchmarkJPEG(b))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/avatars", bytes.NewReader(body))
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-User-ID", "user-1")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			b.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
		}
	}
}

func benchmarkMultipartPayload(b *testing.B, data []byte) ([]byte, string) {
	b.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="avatar.jpg"`)
	header.Set("Content-Type", "image/jpeg")
	part, err := w.CreatePart(header)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		b.Fatal(err)
	}
	if err := w.Close(); err != nil {
		b.Fatal(err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

func benchmarkJPEG(b *testing.B) []byte {
	b.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: 30, G: uint8(x * 4), B: uint8(y * 4), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		b.Fatal(err)
	}
	return buf.Bytes()
}
