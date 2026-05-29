package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"go-avatar-service/internal/domain"
	"go-avatar-service/internal/observability"
)

func TestUploadStoresOriginalAndPublishFailureMarksFailed(t *testing.T) {
	ctx := context.Background()
	svc := NewAvatarService(NewMemoryRepository(), NewMemoryStorage(), &stubBroker{err: errors.New("down")})

	got, err := svc.Upload(ctx, UploadInput{
		UserID:      "user-1",
		FileName:    "avatar.jpg",
		Content:     []byte{0xff, 0xd8, 0xff, 0xdb},
		ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if got.Status != domain.StatusFailed {
		t.Fatalf("status = %q, want failed", got.Status)
	}
	if got.URL != domain.AvatarURL(got.ID) {
		t.Fatalf("url = %q, want relative API URL", got.URL)
	}
	avatar, err := svc.repo.GetActiveByID(ctx, got.ID)
	if err != nil {
		t.Fatalf("repo avatar: %v", err)
	}
	if !avatar.OriginalAvailable {
		t.Fatal("original must remain available after publish failure")
	}
}

func TestServiceObservabilityMetrics(t *testing.T) {
	ctx := context.Background()
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	svc := NewAvatarService(NewMemoryRepository(), NewMemoryStorage(), &stubBroker{err: errors.New("down")}, WithObservability(metrics))

	got, err := svc.Upload(ctx, UploadInput{
		UserID:      "user-1",
		FileName:    "avatar.jpg",
		Content:     []byte{0xff, 0xd8, 0xff, 0xdb},
		ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if got.Status != domain.StatusFailed {
		t.Fatalf("status = %q, want failed", got.Status)
	}
	if err := svc.DeleteByID(ctx, got.ID, "user-1"); err != nil {
		t.Fatalf("DeleteByID() error = %v", err)
	}

	body := gatherMetrics(t, metrics)
	if !bytes.Contains(body, []byte(`avatars_uploads_total{mime_type="image/jpeg",status="publish_failed"} 1`)) {
		t.Fatalf("upload metric missing:\n%s", string(body))
	}
	if !bytes.Contains(body, []byte(`avatars_deletes_total{status="success"} 1`)) {
		t.Fatalf("delete metric missing:\n%s", string(body))
	}
}

func TestListIncludesFailedAndSortsCreatedDesc(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	svc := NewAvatarService(repo, NewMemoryStorage(), &stubBroker{})
	first, _ := svc.Upload(ctx, UploadInput{UserID: "user-1", FileName: "a.jpg", Content: []byte{0xff, 0xd8, 0xff}, ContentType: "image/jpeg"})
	second, _ := svc.Upload(ctx, UploadInput{UserID: "user-1", FileName: "b.jpg", Content: []byte{0xff, 0xd8, 0xff}, ContentType: "image/jpeg"})
	_ = repo.MarkPublishFailed(ctx, first.ID)

	got, err := svc.ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != second.ID {
		t.Fatalf("first id = %q, want latest %q", got[0].ID, second.ID)
	}
	if got[1].Status != domain.StatusFailed {
		t.Fatalf("failed record hidden or status wrong: %+v", got[1])
	}
}

func TestReadExactVariantRules(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	storage := NewMemoryStorage()
	svc := NewAvatarService(repo, storage, &stubBroker{})
	created, _ := svc.Upload(ctx, UploadInput{UserID: "user-1", FileName: "a.jpg", Content: []byte{0xff, 0xd8, 0xff}, ContentType: "image/jpeg"})

	if _, _, err := svc.ReadAvatar(ctx, created.ID, domain.Size100); !errors.Is(err, ErrVariantNotReady) {
		t.Fatalf("ReadAvatar thumb err = %v, want ErrVariantNotReady", err)
	}
	storage.Delete(ctx, "avatars/"+created.ID+"/original.jpg")
	if _, _, err := svc.ReadAvatar(ctx, created.ID, domain.SizeOriginal); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("ReadAvatar missing original err = %v, want ErrObjectNotFound", err)
	}
	meta, err := svc.Metadata(ctx, created.ID)
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}
	if meta.Status != domain.StatusFailed {
		t.Fatalf("metadata status = %q, want failed on drift", meta.Status)
	}
}

func TestUserFallbackAndDeleteRules(t *testing.T) {
	ctx := context.Background()
	svc := NewAvatarService(NewMemoryRepository(), NewMemoryStorage(), &stubBroker{})
	old, _ := svc.Upload(ctx, UploadInput{UserID: "user-1", FileName: "old.jpg", Content: []byte{0xff, 0xd8, 0xff}, ContentType: "image/jpeg"})
	newer, _ := svc.Upload(ctx, UploadInput{UserID: "user-1", FileName: "new.jpg", Content: []byte{0xff, 0xd8, 0xff}, ContentType: "image/jpeg"})
	svc.storage.Delete(ctx, "avatars/"+newer.ID+"/original.jpg")

	data, meta, err := svc.ReadUserAvatar(ctx, "user-1", domain.SizeOriginal)
	if err != nil {
		t.Fatalf("ReadUserAvatar() error = %v", err)
	}
	if meta.ID != old.ID || !bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}) {
		t.Fatalf("fallback returned meta=%+v data=%v, want old original", meta, data)
	}
	if err := svc.DeleteByID(ctx, newer.ID, "intruder"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("DeleteByID wrong owner err = %v, want ErrForbidden", err)
	}
	if err := svc.DeleteCurrentUserAvatar(ctx, "user-1", "intruder"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("DeleteCurrent wrong owner err = %v, want ErrForbidden", err)
	}
	if err := svc.DeleteCurrentUserAvatar(ctx, "user-1", "user-1"); err != nil {
		t.Fatalf("DeleteCurrentUserAvatar() error = %v", err)
	}
}

func TestExistsStorageErrorsPropagate(t *testing.T) {
	ctx := context.Background()
	storageErr := errors.New("storage unavailable")

	t.Run("metadata", func(t *testing.T) {
		baseStorage := NewMemoryStorage()
		svc := NewAvatarService(NewMemoryRepository(), baseStorage, &stubBroker{})
		created, _ := svc.Upload(ctx, UploadInput{UserID: "user-1", FileName: "a.jpg", Content: []byte{0xff, 0xd8, 0xff}, ContentType: "image/jpeg"})
		svc.storage = &failingExistsStorage{Storage: baseStorage, err: storageErr}

		_, err := svc.Metadata(ctx, created.ID)
		if !errors.Is(err, storageErr) {
			t.Fatalf("Metadata() err = %v, want %v", err, storageErr)
		}
	})

	t.Run("gallery", func(t *testing.T) {
		baseStorage := NewMemoryStorage()
		svc := NewAvatarService(NewMemoryRepository(), baseStorage, &stubBroker{})
		_, _ = svc.Upload(ctx, UploadInput{UserID: "user-1", FileName: "a.jpg", Content: []byte{0xff, 0xd8, 0xff}, ContentType: "image/jpeg"})
		svc.storage = &failingExistsStorage{Storage: baseStorage, err: storageErr}

		_, _, err := svc.GalleryByUser(ctx, "user-1")
		if !errors.Is(err, storageErr) {
			t.Fatalf("GalleryByUser() err = %v, want %v", err, storageErr)
		}
	})

	t.Run("delete current", func(t *testing.T) {
		baseStorage := NewMemoryStorage()
		svc := NewAvatarService(NewMemoryRepository(), baseStorage, &stubBroker{})
		_, _ = svc.Upload(ctx, UploadInput{UserID: "user-1", FileName: "a.jpg", Content: []byte{0xff, 0xd8, 0xff}, ContentType: "image/jpeg"})
		svc.storage = &failingExistsStorage{Storage: baseStorage, err: storageErr}

		err := svc.DeleteCurrentUserAvatar(ctx, "user-1", "user-1")
		if !errors.Is(err, storageErr) {
			t.Fatalf("DeleteCurrentUserAvatar() err = %v, want %v", err, storageErr)
		}
	})
}

type stubBroker struct {
	err error
}

func (b *stubBroker) Publish(context.Context, string, []byte, string) error {
	return b.err
}

type failingExistsStorage struct {
	Storage
	err error
}

func (s *failingExistsStorage) Exists(context.Context, string) (bool, error) {
	return false, s.err
}

func gatherMetrics(t *testing.T, metrics *observability.Metrics) []byte {
	t.Helper()
	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rr, req)
	body, err := io.ReadAll(rr.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
