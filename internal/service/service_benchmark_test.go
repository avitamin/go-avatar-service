package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-avatar-service/internal/domain"
)

func BenchmarkUpload(b *testing.B) {
	ctx := context.Background()
	svc := NewAvatarService(NewMemoryRepository(), NewMemoryStorage(), nil)
	input := UploadInput{
		UserID:      "user-1",
		FileName:    "avatar.jpg",
		Content:     []byte{0xff, 0xd8, 0xff, 0xdb},
		ContentType: "image/jpeg",
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Upload(ctx, input); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkListActiveByUser(b *testing.B) {
	for _, count := range []int{10, 1000} {
		b.Run(fmt.Sprintf("records_%d", count), func(b *testing.B) {
			ctx := context.Background()
			repo := NewMemoryRepository()
			now := time.Unix(1700000000, 0).UTC()
			for i := 0; i < count; i++ {
				avatar := &domain.Avatar{
					ID:                fmt.Sprintf("avatar-%d", i),
					UserID:            "user-1",
					OriginalKey:       fmt.Sprintf("avatars/avatar-%d/original.jpg", i),
					OriginalAvailable: true,
					Status:            domain.StatusProcessing,
					CreatedAt:         now.Add(time.Duration(i) * time.Second),
					UpdatedAt:         now.Add(time.Duration(i) * time.Second),
				}
				if err := repo.Create(ctx, avatar); err != nil {
					b.Fatal(err)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := repo.ListActiveByUser(ctx, "user-1"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkReadUserAvatarFallback(b *testing.B) {
	ctx := context.Background()
	svc := NewAvatarService(NewMemoryRepository(), NewMemoryStorage(), nil)
	input := UploadInput{
		UserID:      "user-1",
		FileName:    "avatar.jpg",
		Content:     []byte{0xff, 0xd8, 0xff, 0xdb},
		ContentType: "image/jpeg",
	}
	for i := 0; i < 100; i++ {
		created, err := svc.Upload(ctx, input)
		if err != nil {
			b.Fatal(err)
		}
		if i > 0 {
			if err := svc.storage.Delete(ctx, "avatars/"+created.ID+"/original.jpg"); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := svc.ReadUserAvatar(ctx, "user-1", domain.SizeOriginal); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMetadata(b *testing.B) {
	ctx := context.Background()
	svc := NewAvatarService(NewMemoryRepository(), NewMemoryStorage(), nil)
	created, err := svc.Upload(ctx, UploadInput{
		UserID:      "user-1",
		FileName:    "avatar.jpg",
		Content:     []byte{0xff, 0xd8, 0xff, 0xdb},
		ContentType: "image/jpeg",
	})
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Metadata(ctx, created.ID); err != nil {
			b.Fatal(err)
		}
	}
}
