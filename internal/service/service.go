// Package service implements application services and in-memory test adapters.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go-avatar-service/internal/domain"
	"go-avatar-service/internal/observability"
)

var (
	// ErrNotFound reports a missing avatar or metadata record.
	ErrNotFound = errors.New("not found")
	// ErrForbidden reports an ownership or access violation.
	ErrForbidden = errors.New("forbidden")
	// ErrVariantNotReady reports a requested variant that is not available yet.
	ErrVariantNotReady = errors.New("variant not ready")
	// ErrObjectNotFound reports a missing stored object.
	ErrObjectNotFound = errors.New("object not found")
)

// UploadInput contains the data required to create a new avatar.
type UploadInput struct {
	UserID      string
	FileName    string
	Content     []byte
	ContentType string
}

// AvatarDTO is the API-facing avatar representation returned by services.
type AvatarDTO struct {
	ID         string        `json:"id"`
	UserID     string        `json:"user_id"`
	URL        string        `json:"url"`
	Status     domain.Status `json:"status"`
	CreatedAt  time.Time     `json:"created_at"`
	Thumbnails []string      `json:"thumbnails,omitempty"`
}

// Broker publishes asynchronous avatar events.
type Broker interface {
	Publish(ctx context.Context, topic string, msg []byte, messageID string) error
}

// Repository persists avatar metadata.
type Repository interface {
	Create(context.Context, *domain.Avatar) error
	GetActiveByID(context.Context, string) (*domain.Avatar, error)
	GetByID(context.Context, string) (*domain.Avatar, error)
	ListActiveByUser(context.Context, string) ([]domain.Avatar, error)
	SoftDeleteByID(context.Context, string, time.Time) error
	MarkPublishFailed(context.Context, string) error
	UpdateProcessingResult(context.Context, string, string, string) error
}

// Storage stores avatar objects and generated variants.
type Storage interface {
	Put(context.Context, string, []byte, string) error
	Get(context.Context, string) ([]byte, string, error)
	Delete(context.Context, string) error
	Exists(context.Context, string) (bool, error)
}

// AvatarService coordinates avatar uploads, reads, and deletes.
type AvatarService struct {
	repo    Repository
	storage Storage
	broker  Broker
	now     func() time.Time
	nextID  func() string
	metrics *observability.Metrics
}

// Option customizes AvatarService wiring.
type Option func(*AvatarService)

// WithObservability configures service-level metrics and traces.
func WithObservability(metrics *observability.Metrics) Option {
	return func(s *AvatarService) {
		s.metrics = metrics
	}
}

// NewAvatarService creates an AvatarService with the provided ports.
func NewAvatarService(repo Repository, storage Storage, broker Broker, opts ...Option) *AvatarService {
	var mu sync.Mutex
	seq := 0
	svc := &AvatarService{
		repo:    repo,
		storage: storage,
		broker:  broker,
		now:     time.Now,
		nextID: func() string {
			id, err := randomAvatarID()
			if err == nil {
				return id
			}
			mu.Lock()
			defer mu.Unlock()
			seq++
			return fmt.Sprintf("avatar-%d", seq)
		},
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

func randomAvatarID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "avatar-" + hex.EncodeToString(b[:]), nil
}

// Upload stores the original image, creates metadata, and emits a worker event.
func (s *AvatarService) Upload(ctx context.Context, in UploadInput) (AvatarDTO, error) {
	start := time.Now()
	ctx, span := otel.Tracer("avatar-service/service").Start(ctx, "AvatarService.Upload")
	defer span.End()
	span.SetAttributes(attribute.String("user_id", in.UserID), attribute.String("file_name", in.FileName), attribute.Int("file_size", len(in.Content)), attribute.String("mime_type", in.ContentType))
	status := "success"
	defer func() {
		s.metrics.ObserveAvatarUpload(status, in.ContentType, time.Since(start))
		s.metrics.SetStorageBytes("original", int64(len(in.Content)))
	}()
	if err := domain.ValidateUserID(in.UserID); err != nil {
		status = "error"
		recordSpanError(span, err)
		return AvatarDTO{}, err
	}
	id := s.nextID()
	span.SetAttributes(attribute.String("avatar_id", id))
	now := s.now().UTC()
	ext := extForMime(in.ContentType)
	key := fmt.Sprintf("avatars/%s/original%s", id, ext)
	a := &domain.Avatar{
		ID:                id,
		UserID:            in.UserID,
		FileName:          in.FileName,
		OriginalMimeType:  in.ContentType,
		SizeBytes:         int64(len(in.Content)),
		OriginalKey:       key,
		OriginalAvailable: true,
		Status:            domain.StatusProcessing,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.storage.Put(ctx, key, in.Content, in.ContentType); err != nil {
		status = "error"
		recordSpanError(span, err)
		return AvatarDTO{}, err
	}
	if err := s.repo.Create(ctx, a); err != nil {
		status = "error"
		recordSpanError(span, err)
		return AvatarDTO{}, err
	}
	if s.broker != nil {
		if err := s.broker.Publish(ctx, "avatar.uploaded", []byte(id), id); err != nil {
			status = "publish_failed"
			recordSpanError(span, err)
			_ = s.repo.MarkPublishFailed(ctx, id)
			a.Status = domain.StatusFailed
		}
	}
	return dto(*a), nil
}

// ReadAvatar returns the bytes and MIME type for a specific avatar variant.
func (s *AvatarService) ReadAvatar(ctx context.Context, id string, size domain.Size) ([]byte, string, error) {
	ctx, span := otel.Tracer("avatar-service/service").Start(ctx, "AvatarService.ReadAvatar")
	defer span.End()
	span.SetAttributes(attribute.String("avatar_id", id), attribute.String("size", string(size)))
	a, err := s.repo.GetActiveByID(ctx, id)
	if err != nil {
		recordSpanError(span, err)
		return nil, "", err
	}
	key, ok := a.VariantKey(size)
	if !ok {
		recordSpanError(span, ErrVariantNotReady)
		return nil, "", ErrVariantNotReady
	}
	data, mime, err := s.storage.Get(ctx, key)
	if errors.Is(err, ErrObjectNotFound) {
		recordSpanError(span, ErrObjectNotFound)
		return nil, "", ErrObjectNotFound
	}
	if err != nil {
		recordSpanError(span, err)
	}
	return data, mime, err
}

// Metadata returns API metadata for a single avatar.
func (s *AvatarService) Metadata(ctx context.Context, id string) (AvatarDTO, error) {
	ctx, span := otel.Tracer("avatar-service/service").Start(ctx, "AvatarService.Metadata")
	defer span.End()
	span.SetAttributes(attribute.String("avatar_id", id))
	a, err := s.repo.GetActiveByID(ctx, id)
	if err != nil {
		recordSpanError(span, err)
		return AvatarDTO{}, err
	}
	out := dto(*a)
	exists, err := s.storage.Exists(ctx, a.OriginalKey)
	if err != nil {
		recordSpanError(span, err)
		return AvatarDTO{}, err
	}
	if !exists {
		out.Status = domain.StatusFailed
	}
	if a.Thumb100Available {
		exists, err = s.storage.Exists(ctx, a.Thumb100Key)
		if err != nil {
			recordSpanError(span, err)
			return AvatarDTO{}, err
		}
		if exists {
			out.Thumbnails = append(out.Thumbnails, string(domain.Size100))
		}
	}
	if a.Thumb300Available {
		exists, err = s.storage.Exists(ctx, a.Thumb300Key)
		if err != nil {
			recordSpanError(span, err)
			return AvatarDTO{}, err
		}
		if exists {
			out.Thumbnails = append(out.Thumbnails, string(domain.Size300))
		}
	}
	return out, nil
}

// ListByUser returns active avatars for a user ordered by creation time.
func (s *AvatarService) ListByUser(ctx context.Context, userID string) ([]AvatarDTO, error) {
	ctx, span := otel.Tracer("avatar-service/service").Start(ctx, "AvatarService.ListByUser")
	defer span.End()
	span.SetAttributes(attribute.String("user_id", userID))
	if err := domain.ValidateUserID(userID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	items, err := s.repo.ListActiveByUser(ctx, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	out := make([]AvatarDTO, 0, len(items))
	for _, a := range items {
		out = append(out, dto(a))
	}
	return out, nil
}

// ReadUserAvatar returns the newest available avatar variant for a user.
func (s *AvatarService) ReadUserAvatar(ctx context.Context, userID string, size domain.Size) ([]byte, AvatarDTO, error) {
	ctx, span := otel.Tracer("avatar-service/service").Start(ctx, "AvatarService.ReadUserAvatar")
	defer span.End()
	span.SetAttributes(attribute.String("user_id", userID), attribute.String("size", string(size)))
	if err := domain.ValidateUserID(userID); err != nil {
		recordSpanError(span, err)
		return nil, AvatarDTO{}, err
	}
	items, err := s.repo.ListActiveByUser(ctx, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, AvatarDTO{}, err
	}
	for _, a := range items {
		key, ok := a.VariantKey(size)
		if !ok {
			continue
		}
		data, mime, err := s.storage.Get(ctx, key)
		if err == nil {
			meta := dto(a)
			meta.URL = domain.AvatarURL(a.ID) + "?size=" + string(size)
			_ = mime
			return data, meta, nil
		}
	}
	recordSpanError(span, ErrNotFound)
	return nil, AvatarDTO{}, ErrNotFound
}

// DeleteByID soft-deletes an avatar owned by owner and emits a delete event.
func (s *AvatarService) DeleteByID(ctx context.Context, id, owner string) error {
	ctx, span := otel.Tracer("avatar-service/service").Start(ctx, "AvatarService.DeleteByID")
	defer span.End()
	span.SetAttributes(attribute.String("avatar_id", id), attribute.String("user_id", owner))
	status := "success"
	defer func() {
		s.metrics.IncAvatarDelete(status)
	}()
	if err := domain.ValidateUserID(owner); err != nil {
		status = "error"
		recordSpanError(span, err)
		return err
	}
	a, err := s.repo.GetActiveByID(ctx, id)
	if err != nil {
		status = "error"
		recordSpanError(span, err)
		return err
	}
	if a.UserID != owner {
		status = "forbidden"
		recordSpanError(span, ErrForbidden)
		return ErrForbidden
	}
	if err := s.repo.SoftDeleteByID(ctx, id, s.now().UTC()); err != nil {
		status = "error"
		recordSpanError(span, err)
		return err
	}
	if s.broker != nil {
		_ = s.broker.Publish(ctx, "avatar.delete_requested", []byte(id), id)
	}
	return nil
}

// DeleteCurrentUserAvatar deletes the current user's latest available avatar.
func (s *AvatarService) DeleteCurrentUserAvatar(ctx context.Context, pathUserID, owner string) error {
	ctx, span := otel.Tracer("avatar-service/service").Start(ctx, "AvatarService.DeleteCurrentUserAvatar")
	defer span.End()
	span.SetAttributes(attribute.String("user_id", pathUserID))
	if err := domain.ValidateUserID(pathUserID); err != nil {
		recordSpanError(span, err)
		return err
	}
	if err := domain.ValidateUserID(owner); err != nil {
		recordSpanError(span, err)
		return err
	}
	if pathUserID != owner {
		recordSpanError(span, ErrForbidden)
		return ErrForbidden
	}
	items, err := s.repo.ListActiveByUser(ctx, pathUserID)
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	for _, a := range items {
		if !a.OriginalAvailable {
			continue
		}
		exists, err := s.storage.Exists(ctx, a.OriginalKey)
		if err != nil {
			recordSpanError(span, err)
			return err
		}
		if exists {
			return s.DeleteByID(ctx, a.ID, owner)
		}
	}
	recordSpanError(span, ErrNotFound)
	return ErrNotFound
}

// GalleryByUser returns avatar cards for the web gallery and whether the user exists.
func (s *AvatarService) GalleryByUser(ctx context.Context, userID string) ([]AvatarDTO, bool, error) {
	ctx, span := otel.Tracer("avatar-service/service").Start(ctx, "AvatarService.GalleryByUser")
	defer span.End()
	span.SetAttributes(attribute.String("user_id", userID))
	items, err := s.repo.ListActiveByUser(ctx, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, false, err
	}
	out := make([]AvatarDTO, 0, len(items))
	for _, a := range items {
		if !a.OriginalAvailable {
			continue
		}
		exists, err := s.storage.Exists(ctx, a.OriginalKey)
		if err != nil {
			recordSpanError(span, err)
			return nil, false, err
		}
		if exists {
			out = append(out, dto(a))
		}
	}
	return out, len(items) > 0, nil
}

func recordSpanError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func dto(a domain.Avatar) AvatarDTO {
	return AvatarDTO{
		ID:        a.ID,
		UserID:    a.UserID,
		URL:       domain.AvatarURL(a.ID),
		Status:    a.ExternalStatus(),
		CreatedAt: a.CreatedAt,
	}
}

func extForMime(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		ext := filepath.Ext(mime)
		if ext == "" || strings.Contains(ext, "/") {
			return ".bin"
		}
		return ext
	}
}

// MemoryRepository is an in-memory Repository implementation for tests and fallback mode.
type MemoryRepository struct {
	mu      sync.RWMutex
	avatars map[string]domain.Avatar
}

// NewMemoryRepository creates an empty in-memory avatar repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{avatars: map[string]domain.Avatar{}}
}

// Create stores avatar metadata in memory.
func (r *MemoryRepository) Create(_ context.Context, a *domain.Avatar) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.avatars[a.ID] = *a
	return nil
}

// GetActiveByID returns an active avatar by identifier.
func (r *MemoryRepository) GetActiveByID(ctx context.Context, id string) (*domain.Avatar, error) {
	a, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.DeletedAt != nil {
		return nil, ErrNotFound
	}
	return a, nil
}

// GetByID returns an avatar by identifier, including soft-deleted records.
func (r *MemoryRepository) GetByID(_ context.Context, id string) (*domain.Avatar, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.avatars[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &a, nil
}

// ListActiveByUser lists active avatars for a user in reverse creation order.
func (r *MemoryRepository) ListActiveByUser(_ context.Context, userID string) ([]domain.Avatar, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []domain.Avatar
	for _, a := range r.avatars {
		if a.UserID == userID && a.DeletedAt == nil {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// SoftDeleteByID marks an avatar deleted without removing its metadata row.
func (r *MemoryRepository) SoftDeleteByID(_ context.Context, id string, deletedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.avatars[id]
	if !ok || a.DeletedAt != nil {
		return ErrNotFound
	}
	a.DeletedAt = &deletedAt
	a.UpdatedAt = deletedAt
	r.avatars[id] = a
	return nil
}

// MarkPublishFailed marks an avatar as failed after event publication problems.
func (r *MemoryRepository) MarkPublishFailed(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.avatars[id]
	if !ok {
		return ErrNotFound
	}
	a.Status = domain.StatusFailed
	a.UpdatedAt = time.Now().UTC()
	r.avatars[id] = a
	return nil
}

// UpdateProcessingResult stores generated thumbnail keys and completion status.
func (r *MemoryRepository) UpdateProcessingResult(_ context.Context, id, thumb100, thumb300 string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.avatars[id]
	if !ok {
		return ErrNotFound
	}
	a.Thumb100Key = thumb100
	a.Thumb300Key = thumb300
	a.Thumb100Available = thumb100 != ""
	a.Thumb300Available = thumb300 != ""
	if a.Thumb100Available && a.Thumb300Available {
		a.Status = domain.StatusCompleted
	} else {
		a.Status = domain.StatusFailed
	}
	a.UpdatedAt = time.Now().UTC()
	r.avatars[id] = a
	return nil
}

type object struct {
	data []byte
	mime string
}

// MemoryStorage is an in-memory Storage implementation for tests and fallback mode.
type MemoryStorage struct {
	mu      sync.RWMutex
	objects map[string]object
}

// NewMemoryStorage creates an empty in-memory object storage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{objects: map[string]object{}}
}

// Put stores object bytes under the provided key.
func (s *MemoryStorage) Put(_ context.Context, key string, data []byte, mime string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := append([]byte(nil), data...)
	s.objects[key] = object{data: cp, mime: mime}
	return nil
}

// Get returns object bytes and MIME type by key.
func (s *MemoryStorage) Get(_ context.Context, key string) ([]byte, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	obj, ok := s.objects[key]
	if !ok {
		return nil, "", ErrObjectNotFound
	}
	return append([]byte(nil), obj.data...), obj.mime, nil
}

// Delete removes an object key from memory.
func (s *MemoryStorage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

// Exists reports whether the key is currently present in memory.
func (s *MemoryStorage) Exists(_ context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.objects[key]
	return ok, nil
}
