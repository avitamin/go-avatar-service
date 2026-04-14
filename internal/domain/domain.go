package domain

import (
	"errors"
	"regexp"
	"time"
)

type Status string

const (
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

type Size string

const (
	SizeOriginal Size = "original"
	Size100      Size = "100x100"
	Size300      Size = "300x300"
)

var userIDPattern = regexp.MustCompile(`^[A-Za-z0-9._@-]+$`)

var (
	ErrInvalidUserID = errors.New("invalid user id")
	ErrInvalidSize   = errors.New("invalid size")
)

type Avatar struct {
	ID                string
	UserID            string
	FileName          string
	OriginalMimeType  string
	SizeBytes         int64
	OriginalKey       string
	Thumb100Key       string
	Thumb300Key       string
	OriginalAvailable bool
	Thumb100Available bool
	Thumb300Available bool
	Status            Status
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

func ValidateUserID(userID string) error {
	if len(userID) < 1 || len(userID) > 255 {
		return ErrInvalidUserID
	}
	if !userIDPattern.MatchString(userID) {
		return ErrInvalidUserID
	}
	return nil
}

func ParseSize(raw string) (Size, error) {
	switch raw {
	case "", string(SizeOriginal):
		return SizeOriginal, nil
	case string(Size100):
		return Size100, nil
	case string(Size300):
		return Size300, nil
	default:
		return "", ErrInvalidSize
	}
}

func (a Avatar) ExternalStatus() Status {
	if !a.OriginalAvailable {
		return StatusFailed
	}
	switch a.Status {
	case StatusProcessing, StatusCompleted, StatusFailed:
		return a.Status
	default:
		return StatusFailed
	}
}

func (a Avatar) VariantKey(size Size) (string, bool) {
	switch size {
	case SizeOriginal:
		return a.OriginalKey, a.OriginalAvailable
	case Size100:
		return a.Thumb100Key, a.Thumb100Available
	case Size300:
		return a.Thumb300Key, a.Thumb300Available
	default:
		return "", false
	}
}

func AvatarURL(id string) string {
	return "/api/v1/avatars/" + id
}
