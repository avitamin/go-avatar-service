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

	"go-avatar-service/internal/domain"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrForbidden       = errors.New("forbidden")
	ErrVariantNotReady = errors.New("variant not ready")
	ErrObjectNotFound  = errors.New("object not found")
)

type UploadInput struct {
	UserID      string
	FileName    string
	Content     []byte
	ContentType string
}

type AvatarDTO struct {
	ID         string        `json:"id"`
	UserID     string        `json:"user_id"`
	URL        string        `json:"url"`
	Status     domain.Status `json:"status"`
	CreatedAt  time.Time     `json:"created_at"`
	Thumbnails []string      `json:"thumbnails,omitempty"`
}

type Broker interface {
	Publish(ctx context.Context, topic string, msg []byte, messageID string) error
}

type Repository interface {
	Create(context.Context, *domain.Avatar) error
	GetActiveByID(context.Context, string) (*domain.Avatar, error)
	GetByID(context.Context, string) (*domain.Avatar, error)
	ListActiveByUser(context.Context, string) ([]domain.Avatar, error)
	SoftDeleteByID(context.Context, string, time.Time) error
	MarkPublishFailed(context.Context, string) error
	UpdateProcessingResult(context.Context, string, string, string) error
}

type Storage interface {
	Put(context.Context, string, []byte, string) error
	Get(context.Context, string) ([]byte, string, error)
	Delete(context.Context, string) error
	Exists(context.Context, string) bool
}

type AvatarService struct {
	repo    Repository
	storage Storage
	broker  Broker
	now     func() time.Time
	nextID  func() string
}

func NewAvatarService(repo Repository, storage Storage, broker Broker) *AvatarService {
	var mu sync.Mutex
	seq := 0
	return &AvatarService{
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
}

func randomAvatarID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "avatar-" + hex.EncodeToString(b[:]), nil
}

func (s *AvatarService) Upload(ctx context.Context, in UploadInput) (AvatarDTO, error) {
	if err := domain.ValidateUserID(in.UserID); err != nil {
		return AvatarDTO{}, err
	}
	id := s.nextID()
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
		return AvatarDTO{}, err
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return AvatarDTO{}, err
	}
	if s.broker != nil {
		if err := s.broker.Publish(ctx, "avatar.uploaded", []byte(id), id); err != nil {
			_ = s.repo.MarkPublishFailed(ctx, id)
			a.Status = domain.StatusFailed
		}
	}
	return dto(*a), nil
}

func (s *AvatarService) ReadAvatar(ctx context.Context, id string, size domain.Size) ([]byte, string, error) {
	a, err := s.repo.GetActiveByID(ctx, id)
	if err != nil {
		return nil, "", err
	}
	key, ok := a.VariantKey(size)
	if !ok {
		return nil, "", ErrVariantNotReady
	}
	data, mime, err := s.storage.Get(ctx, key)
	if errors.Is(err, ErrObjectNotFound) {
		return nil, "", ErrObjectNotFound
	}
	return data, mime, err
}

func (s *AvatarService) Metadata(ctx context.Context, id string) (AvatarDTO, error) {
	a, err := s.repo.GetActiveByID(ctx, id)
	if err != nil {
		return AvatarDTO{}, err
	}
	out := dto(*a)
	if !s.storage.Exists(ctx, a.OriginalKey) {
		out.Status = domain.StatusFailed
	}
	if a.Thumb100Available && s.storage.Exists(ctx, a.Thumb100Key) {
		out.Thumbnails = append(out.Thumbnails, string(domain.Size100))
	}
	if a.Thumb300Available && s.storage.Exists(ctx, a.Thumb300Key) {
		out.Thumbnails = append(out.Thumbnails, string(domain.Size300))
	}
	return out, nil
}

func (s *AvatarService) ListByUser(ctx context.Context, userID string) ([]AvatarDTO, error) {
	if err := domain.ValidateUserID(userID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListActiveByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]AvatarDTO, 0, len(items))
	for _, a := range items {
		out = append(out, dto(a))
	}
	return out, nil
}

func (s *AvatarService) ReadUserAvatar(ctx context.Context, userID string, size domain.Size) ([]byte, AvatarDTO, error) {
	if err := domain.ValidateUserID(userID); err != nil {
		return nil, AvatarDTO{}, err
	}
	items, err := s.repo.ListActiveByUser(ctx, userID)
	if err != nil {
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
	return nil, AvatarDTO{}, ErrNotFound
}

func (s *AvatarService) DeleteByID(ctx context.Context, id, owner string) error {
	if err := domain.ValidateUserID(owner); err != nil {
		return err
	}
	a, err := s.repo.GetActiveByID(ctx, id)
	if err != nil {
		return err
	}
	if a.UserID != owner {
		return ErrForbidden
	}
	if err := s.repo.SoftDeleteByID(ctx, id, s.now().UTC()); err != nil {
		return err
	}
	if s.broker != nil {
		_ = s.broker.Publish(ctx, "avatar.delete_requested", []byte(id), id)
	}
	return nil
}

func (s *AvatarService) DeleteCurrentUserAvatar(ctx context.Context, pathUserID, owner string) error {
	if err := domain.ValidateUserID(pathUserID); err != nil {
		return err
	}
	if err := domain.ValidateUserID(owner); err != nil {
		return err
	}
	if pathUserID != owner {
		return ErrForbidden
	}
	items, err := s.repo.ListActiveByUser(ctx, pathUserID)
	if err != nil {
		return err
	}
	for _, a := range items {
		if a.OriginalAvailable && s.storage.Exists(ctx, a.OriginalKey) {
			return s.DeleteByID(ctx, a.ID, owner)
		}
	}
	return ErrNotFound
}

func (s *AvatarService) GalleryByUser(ctx context.Context, userID string) ([]AvatarDTO, bool, error) {
	items, err := s.repo.ListActiveByUser(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	out := make([]AvatarDTO, 0, len(items))
	for _, a := range items {
		if a.OriginalAvailable && s.storage.Exists(ctx, a.OriginalKey) {
			out = append(out, dto(a))
		}
	}
	return out, len(items) > 0, nil
}

func (s *AvatarService) Repo() Repository { return s.repo }
func (s *AvatarService) Storage() Storage { return s.storage }

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

type MemoryRepository struct {
	mu      sync.RWMutex
	avatars map[string]domain.Avatar
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{avatars: map[string]domain.Avatar{}}
}

func (r *MemoryRepository) Create(_ context.Context, a *domain.Avatar) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.avatars[a.ID] = *a
	return nil
}

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

func (r *MemoryRepository) GetByID(_ context.Context, id string) (*domain.Avatar, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.avatars[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &a, nil
}

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

type MemoryStorage struct {
	mu      sync.RWMutex
	objects map[string]object
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{objects: map[string]object{}}
}

func (s *MemoryStorage) Put(_ context.Context, key string, data []byte, mime string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := append([]byte(nil), data...)
	s.objects[key] = object{data: cp, mime: mime}
	return nil
}

func (s *MemoryStorage) Get(_ context.Context, key string) ([]byte, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	obj, ok := s.objects[key]
	if !ok {
		return nil, "", ErrObjectNotFound
	}
	return append([]byte(nil), obj.data...), obj.mime, nil
}

func (s *MemoryStorage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

func (s *MemoryStorage) Exists(_ context.Context, key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.objects[key]
	return ok
}
