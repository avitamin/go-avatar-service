package domain

import "testing"

func TestValidateUserID(t *testing.T) {
	long := make([]byte, 256)
	for i := range long {
		long[i] = 'a'
	}

	tests := []struct {
		name    string
		userID  string
		wantErr bool
	}{
		{name: "one character", userID: "a"},
		{name: "max length", userID: string(long[:255])},
		{name: "underscore dot dash at", userID: "user_1.test@example"},
		{name: "empty", userID: "", wantErr: true},
		{name: "too long", userID: string(long), wantErr: true},
		{name: "space", userID: "bad user", wantErr: true},
		{name: "slash", userID: "bad/user", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserID(tt.userID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateUserID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Size
		wantErr bool
	}{
		{name: "default", input: "", want: SizeOriginal},
		{name: "original", input: "original", want: SizeOriginal},
		{name: "100", input: "100x100", want: Size100},
		{name: "300", input: "300x300", want: Size300},
		{name: "unsupported", input: "42x42", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSize() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("ParseSize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExternalStatus(t *testing.T) {
	tests := []struct {
		name string
		a    Avatar
		want Status
	}{
		{name: "processing", a: Avatar{Status: StatusProcessing, OriginalAvailable: true}, want: StatusProcessing},
		{name: "completed", a: Avatar{Status: StatusCompleted, OriginalAvailable: true}, want: StatusCompleted},
		{name: "failed", a: Avatar{Status: StatusFailed, OriginalAvailable: true}, want: StatusFailed},
		{name: "missing original normalizes failed", a: Avatar{Status: StatusCompleted}, want: StatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.ExternalStatus(); got != tt.want {
				t.Fatalf("ExternalStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}
