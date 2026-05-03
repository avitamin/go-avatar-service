// Package domain defines core avatar entities and validation rules.
package domain

import (
	"errors"
	"regexp"
	"time"
)

// Status describes the externally visible state of an avatar.
type Status string

const (
	// StatusProcessing marks an avatar that is waiting for thumbnails.
	StatusProcessing Status = "processing"
	// StatusCompleted marks an avatar with all required variants available.
	StatusCompleted Status = "completed"
	// StatusFailed marks an avatar that cannot serve the requested content.
	StatusFailed Status = "failed"
)

// Size identifies a supported avatar variant.
type Size string

const (
	// SizeOriginal selects the original uploaded image.
	SizeOriginal Size = "original"
	// Size100 selects the 100x100 thumbnail.
	Size100 Size = "100x100"
	// Size300 selects the 300x300 thumbnail.
	Size300 Size = "300x300"
)

var userIDPattern = regexp.MustCompile(`^[A-Za-z0-9._@-]+$`)

var (
	// ErrInvalidUserID reports an invalid user identifier.
	ErrInvalidUserID = errors.New("invalid user id")
	// ErrInvalidSize reports an unsupported avatar size.
	ErrInvalidSize = errors.New("invalid size")
)

// Avatar stores metadata about an uploaded avatar and its variants.
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

// ValidateUserID checks that userID matches the API constraints.
func ValidateUserID(userID string) error {
	if len(userID) < 1 || len(userID) > 255 {
		return ErrInvalidUserID
	}
	if !userIDPattern.MatchString(userID) {
		return ErrInvalidUserID
	}
	return nil
}

// ParseSize converts a raw query value into a supported Size.
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

// ExternalStatus returns the status exposed by the public API.
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

// VariantKey returns the storage key and availability for the requested size.
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

// AvatarURL returns the API path for the avatar resource.
func AvatarURL(id string) string {
	return "/api/v1/avatars/" + id
}
