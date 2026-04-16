package domain

import "testing"

func BenchmarkValidateUserID(b *testing.B) {
	benchmarks := []struct {
		name   string
		userID string
	}{
		{name: "valid", userID: "user_1.test@example"},
		{name: "invalid", userID: "bad user"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = ValidateUserID(bm.userID)
			}
		})
	}
}

func BenchmarkParseSize(b *testing.B) {
	benchmarks := []struct {
		name string
		raw  string
	}{
		{name: "default", raw: ""},
		{name: "thumb100", raw: "100x100"},
		{name: "invalid", raw: "42x42"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = ParseSize(bm.raw)
			}
		})
	}
}

func BenchmarkExternalStatus(b *testing.B) {
	avatar := Avatar{Status: StatusCompleted, OriginalAvailable: true}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = avatar.ExternalStatus()
	}
}
