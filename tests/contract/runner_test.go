package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewRunnerRequiresBaseURL(t *testing.T) {
	t.Setenv("BASE_URL", "")

	_, err := NewRunner(Config{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunnerURL(t *testing.T) {
	runner, err := NewRunner(Config{BaseURL: "http://example.test/root/", UserID: "u"})
	if err != nil {
		t.Fatal(err)
	}

	got := runner.URL("/api/v1/avatars")
	want := "http://example.test/root/api/v1/avatars"
	if got != want {
		t.Fatalf("URL()=%q, want %q", got, want)
	}
}

func TestValidateJSONError(t *testing.T) {
	valid := []byte(`{"error":{"code":"invalid_size","message":"Unsupported size"}}`)
	if err := validateJSONError(valid); err != nil {
		t.Fatalf("valid error rejected: %v", err)
	}

	invalid := []byte(`{"error":{"code":"invalid_size"}}`)
	if err := validateJSONError(invalid); err == nil {
		t.Fatal("expected missing message error")
	}
}

func TestMultipartBodyUsesRequestedField(t *testing.T) {
	body, contentType, err := multipartBody("file", "avatar.jpg", "image/jpeg", testJPEG())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", contentType)

	if err := req.ParseMultipartForm(1024 * 1024); err != nil {
		t.Fatal(err)
	}
	if req.MultipartForm.File["file"] == nil {
		t.Fatal("multipart field file is missing")
	}
	if req.MultipartForm.File["image"] != nil {
		t.Fatal("unexpected multipart field image")
	}
}

func TestRunnerFailsWrongErrorShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	runner, err := NewRunner(Config{BaseURL: server.URL, Out: &out})
	if err != nil {
		t.Fatal(err)
	}

	report := runner.Run(context.Background(), []Scenario{
		{Name: "bad error", Run: scenarioUploadMissingUser},
	})
	if report.Failed() != 1 {
		t.Fatalf("failed=%d, want 1; out=%s", report.Failed(), out.String())
	}
}

func TestRunnerRejectsImageFieldCompatibilityAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/avatars" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseMultipartForm(maxUploadBytes + 1024); err != nil {
			writeError(w, http.StatusBadRequest, "bad_multipart")
			return
		}
		if r.MultipartForm.File["image"] != nil {
			writeUpload(w, "1", r.Header.Get("X-User-ID"))
			return
		}
		writeError(w, http.StatusBadRequest, "missing_image")
	}))
	defer server.Close()

	var out bytes.Buffer
	runner, err := NewRunner(Config{BaseURL: server.URL, Out: &out})
	if err != nil {
		t.Fatal(err)
	}

	report := runner.Run(context.Background(), []Scenario{
		{Name: "wrong field", Run: scenarioUploadWrongField},
	})
	if report.Failed() != 1 {
		t.Fatalf("failed=%d, want 1; out=%s", report.Failed(), out.String())
	}
}

func TestRunnerHappyPathAgainstFakeAPI(t *testing.T) {
	api := newFakeAPI()
	server := httptest.NewServer(api)
	defer server.Close()

	var out bytes.Buffer
	runner, err := NewRunner(Config{
		BaseURL: server.URL,
		UserID:  "contract-" + fmt.Sprint(time.Now().UnixNano()),
		Out:     &out,
	})
	if err != nil {
		t.Fatal(err)
	}

	report := runner.Run(context.Background(), DefaultScenarios())
	if report.Failed() != 0 {
		t.Fatalf("failed=%d, want 0; out=%s", report.Failed(), out.String())
	}
}

type fakeAPI struct {
	avatars map[string]fakeAvatar
	nextID  int
}

type fakeAvatar struct {
	ID      string
	UserID  string
	Deleted bool
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{avatars: map[string]fakeAvatar{}}
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/health":
		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "healthy",
			"postgres": map[string]string{"status": "ok"},
			"minio":    map[string]string{"status": "ok"},
			"rabbitmq": map[string]string{"status": "ok"},
		})
	case r.Method == http.MethodGet && r.URL.Path == "/web/upload":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html></html>"))
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/web/gallery/"):
		userID := strings.TrimPrefix(r.URL.Path, "/web/gallery/")
		if strings.Contains(userID, " ") {
			writeError(w, http.StatusBadRequest, "invalid_user_id")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html></html>"))
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/avatars":
		f.handleUpload(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/avatars/") && strings.HasSuffix(r.URL.Path, "/metadata"):
		f.handleMetadata(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/users/") && strings.HasSuffix(r.URL.Path, "/avatars"):
		f.handleList(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/users/") && strings.HasSuffix(r.URL.Path, "/avatar"):
		f.handleUserAvatar(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/users/") && strings.HasSuffix(r.URL.Path, "/avatar"):
		f.handleDeleteCurrent(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/avatars/"):
		f.handleReadAvatar(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/avatars/"):
		f.handleDeleteAvatar(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeAPI) handleUpload(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id")
		return
	}
	if err := r.ParseMultipartForm(maxUploadBytes + 1024); err != nil {
		writeError(w, http.StatusBadRequest, "bad_multipart")
		return
	}
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "missing_file")
		return
	}
	file, err := files[0].Open()
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_file")
		return
	}
	defer file.Close()
	data, _ := io.ReadAll(file)
	if int64(len(data)) > maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "file_too_large")
		return
	}
	if !bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}) && !bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G'}) {
		writeError(w, http.StatusBadRequest, "invalid_file_format")
		return
	}

	f.nextID++
	id := fmt.Sprintf("avatar-%d", f.nextID)
	f.avatars[id] = fakeAvatar{ID: id, UserID: userID}
	writeUpload(w, id, userID)
}

func (f *fakeAPI) handleMetadata(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/avatars/"), "/metadata")
	avatar, ok := f.avatars[id]
	if !ok || avatar.Deleted {
		writeError(w, http.StatusNotFound, "avatar_not_found")
		return
	}
	writeJSON(w, http.StatusOK, avatarPayload(avatar))
}

func (f *fakeAPI) handleList(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/users/"), "/avatars")
	items := make([]map[string]any, 0)
	for _, avatar := range f.avatars {
		if avatar.UserID == userID && !avatar.Deleted {
			items = append(items, avatarPayload(avatar))
		}
	}
	writeJSON(w, http.StatusOK, items)
}

func (f *fakeAPI) handleUserAvatar(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/users/"), "/avatar")
	for _, avatar := range f.avatars {
		if avatar.UserID == userID && !avatar.Deleted {
			writeImage(w)
			return
		}
	}
	writeError(w, http.StatusNotFound, "avatar_not_found")
}

func (f *fakeAPI) handleReadAvatar(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("size") == "42x42" {
		writeError(w, http.StatusBadRequest, "invalid_size")
		return
	}
	if r.URL.Query().Has("format") {
		writeError(w, http.StatusBadRequest, "unsupported_format")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/avatars/")
	avatar, ok := f.avatars[id]
	if !ok || avatar.Deleted {
		writeError(w, http.StatusNotFound, "avatar_not_found")
		return
	}
	if r.URL.Query().Get("size") == "100x100" {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(testJPEG())
		return
	}
	writeImage(w)
}

func (f *fakeAPI) handleDeleteAvatar(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/avatars/")
	avatar, ok := f.avatars[id]
	if !ok || avatar.Deleted {
		writeError(w, http.StatusNotFound, "avatar_not_found")
		return
	}
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id")
		return
	}
	if userID != avatar.UserID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	avatar.Deleted = true
	f.avatars[id] = avatar
	w.WriteHeader(http.StatusNoContent)
}

func (f *fakeAPI) handleDeleteCurrent(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/users/"), "/avatar")
	headerUserID := r.Header.Get("X-User-ID")
	if headerUserID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id")
		return
	}
	if headerUserID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	for id, avatar := range f.avatars {
		if avatar.UserID == userID && !avatar.Deleted {
			avatar.Deleted = true
			f.avatars[id] = avatar
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	writeError(w, http.StatusNotFound, "avatar_not_found")
}

func writeUpload(w http.ResponseWriter, id, userID string) {
	writeJSON(w, http.StatusCreated, avatarPayload(fakeAvatar{ID: id, UserID: userID}))
}

func avatarPayload(avatar fakeAvatar) map[string]any {
	return map[string]any{
		"id":         avatar.ID,
		"user_id":    avatar.UserID,
		"url":        "/api/v1/avatars/" + avatar.ID,
		"status":     "completed",
		"created_at": "2026-04-14T00:00:00Z",
	}
}

func writeImage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "image/jpeg")
	_, _ = w.Write(testJPEG())
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": code,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
