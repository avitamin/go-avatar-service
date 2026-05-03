package contract

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type uploadResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	URL       string `json:"url"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func scenarioHealth(ctx context.Context, r *Runner) error {
	resp, err := r.do(ctx, http.MethodGet, "/health", nil, nil)
	if err != nil {
		return err
	}
	if err := expectStatus(resp, http.StatusOK); err != nil {
		return err
	}

	var payload map[string]any
	if err := decodeJSON(resp.Body, &payload); err != nil {
		return err
	}
	status, ok := payload["status"].(string)
	if !ok || status == "" {
		return fmt.Errorf("health status is required")
	}
	components := payload
	if nested, ok := payload["components"].(map[string]any); ok {
		components = nested
	}
	for _, component := range []string{"postgres", "minio", "rabbitmq"} {
		if _, ok := components[component]; !ok {
			return fmt.Errorf("health component %q is required", component)
		}
	}
	return nil
}

func scenarioWebUpload(ctx context.Context, r *Runner) error {
	resp, err := r.do(ctx, http.MethodGet, "/web/upload", nil, nil)
	if err != nil {
		return err
	}
	if err := expectStatus(resp, http.StatusOK); err != nil {
		return err
	}
	if !strings.Contains(strings.ToLower(resp.ContentType), "text/html") {
		return fmt.Errorf("web upload content type = %q, want html", resp.ContentType)
	}
	return nil
}

func scenarioWebGalleryInvalidUser(ctx context.Context, r *Runner) error {
	resp, err := r.do(ctx, http.MethodGet, "/web/gallery/bad user", nil, nil)
	if err != nil {
		return err
	}
	return expectJSONError(resp, http.StatusBadRequest)
}

func scenarioUploadMissingUser(ctx context.Context, r *Runner) error {
	body, contentType, err := multipartBody("file", "avatar.jpg", "image/jpeg", testJPEG())
	if err != nil {
		return err
	}
	resp, err := r.do(ctx, http.MethodPost, "/api/v1/avatars", body, map[string]string{
		"Content-Type": contentType,
	})
	if err != nil {
		return err
	}
	return expectJSONError(resp, http.StatusBadRequest)
}

func scenarioUploadWrongField(ctx context.Context, r *Runner) error {
	body, contentType, err := multipartBody("image", "avatar.jpg", "image/jpeg", testJPEG())
	if err != nil {
		return err
	}
	resp, err := r.do(ctx, http.MethodPost, "/api/v1/avatars", body, map[string]string{
		"Content-Type": contentType,
		"X-User-ID":    r.UserID("wrong-field"),
	})
	if err != nil {
		return err
	}
	return expectJSONError(resp, http.StatusBadRequest)
}

func scenarioUploadInvalidImage(ctx context.Context, r *Runner) error {
	body, contentType, err := multipartBody("file", "avatar.jpg", "image/jpeg", []byte("not an image"))
	if err != nil {
		return err
	}
	resp, err := r.do(ctx, http.MethodPost, "/api/v1/avatars", body, map[string]string{
		"Content-Type": contentType,
		"X-User-ID":    r.UserID("invalid-image"),
	})
	if err != nil {
		return err
	}
	return expectJSONError(resp, http.StatusBadRequest)
}

func scenarioUploadTooLarge(ctx context.Context, r *Runner) error {
	body, contentType, err := multipartBody("file", "avatar.jpg", "image/jpeg", oversizedPayload())
	if err != nil {
		return err
	}
	resp, err := r.do(ctx, http.MethodPost, "/api/v1/avatars", body, map[string]string{
		"Content-Type": contentType,
		"X-User-ID":    r.UserID("too-large"),
	})
	if err != nil {
		return err
	}
	return expectJSONError(resp, http.StatusRequestEntityTooLarge)
}

func scenarioUploadValid(ctx context.Context, r *Runner) error {
	created, err := r.upload(ctx, r.UserID("main"), "avatar.jpg", testJPEG())
	if err != nil {
		return err
	}
	r.state.AvatarID = created.ID
	return nil
}

func scenarioMetadata(ctx context.Context, r *Runner) error {
	if r.state.AvatarID == "" {
		return fmt.Errorf("avatar id is empty")
	}
	resp, err := r.do(ctx, http.MethodGet, "/api/v1/avatars/"+r.state.AvatarID+"/metadata", nil, nil)
	if err != nil {
		return err
	}
	if err := expectStatus(resp, http.StatusOK); err != nil {
		return err
	}

	var payload map[string]any
	if err := decodeJSON(resp.Body, &payload); err != nil {
		return err
	}
	return requireFields(payload, "id", "user_id", "url", "status", "created_at")
}

func scenarioList(ctx context.Context, r *Runner) error {
	resp, err := r.do(ctx, http.MethodGet, "/api/v1/users/"+r.UserID("main")+"/avatars", nil, nil)
	if err != nil {
		return err
	}
	if err := expectStatus(resp, http.StatusOK); err != nil {
		return err
	}
	if !json.Valid(resp.Body) {
		return fmt.Errorf("list response is not valid JSON: %s", trimBody(resp.Body))
	}
	if !strings.Contains(string(resp.Body), r.state.AvatarID) {
		return fmt.Errorf("list response does not include uploaded avatar %q", r.state.AvatarID)
	}
	return nil
}

func scenarioReadOriginal(ctx context.Context, r *Runner) error {
	resp, err := r.do(ctx, http.MethodGet, "/api/v1/avatars/"+r.state.AvatarID, nil, nil)
	if err != nil {
		return err
	}
	if err := expectStatus(resp, http.StatusOK); err != nil {
		return err
	}
	return expectImage(resp, "image/")
}

func scenarioUnsupportedSize(ctx context.Context, r *Runner) error {
	resp, err := r.do(ctx, http.MethodGet, "/api/v1/avatars/"+r.state.AvatarID+"?size=42x42", nil, nil)
	if err != nil {
		return err
	}
	return expectJSONError(resp, http.StatusBadRequest)
}

func scenarioUnsupportedFormat(ctx context.Context, r *Runner) error {
	resp, err := r.do(ctx, http.MethodGet, "/api/v1/avatars/"+r.state.AvatarID+"?format=webp", nil, nil)
	if err != nil {
		return err
	}
	return expectJSONError(resp, http.StatusBadRequest)
}

func scenarioReadThumbnail(ctx context.Context, r *Runner) error {
	deadline := time.Now().Add(2 * time.Second)
	var last *httpResponse
	for {
		resp, err := r.do(ctx, http.MethodGet, "/api/v1/avatars/"+r.state.AvatarID+"?size=100x100", nil, nil)
		if err != nil {
			return err
		}
		last = resp
		if resp.StatusCode == http.StatusOK || time.Now().After(deadline) {
			break
		}
		if resp.StatusCode != http.StatusConflict {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if last.StatusCode == http.StatusConflict {
		return validateJSONError(last.Body)
	}
	if err := expectStatus(last, http.StatusOK); err != nil {
		return err
	}
	return expectImage(last, "image/jpeg")
}

func scenarioReadUserAvatar(ctx context.Context, r *Runner) error {
	resp, err := r.do(ctx, http.MethodGet, "/api/v1/users/"+r.UserID("main")+"/avatar", nil, nil)
	if err != nil {
		return err
	}
	if err := expectStatus(resp, http.StatusOK); err != nil {
		return err
	}
	return expectImage(resp, "image/")
}

func scenarioDeleteRequiresOwner(ctx context.Context, r *Runner) error {
	resp, err := r.do(ctx, http.MethodDelete, "/api/v1/avatars/"+r.state.AvatarID, nil, nil)
	if err != nil {
		return err
	}
	return expectJSONError(resp, http.StatusBadRequest)
}

func scenarioDeleteWrongOwner(ctx context.Context, r *Runner) error {
	resp, err := r.do(ctx, http.MethodDelete, "/api/v1/avatars/"+r.state.AvatarID, nil, map[string]string{
		"X-User-ID": r.UserID("intruder"),
	})
	if err != nil {
		return err
	}
	return expectJSONError(resp, http.StatusForbidden)
}

func scenarioDeleteOwner(ctx context.Context, r *Runner) error {
	resp, err := r.do(ctx, http.MethodDelete, "/api/v1/avatars/"+r.state.AvatarID, nil, map[string]string{
		"X-User-ID": r.UserID("main"),
	})
	if err != nil {
		return err
	}
	if err := expectStatus(resp, http.StatusNoContent); err != nil {
		return err
	}

	resp, err = r.do(ctx, http.MethodDelete, "/api/v1/avatars/"+r.state.AvatarID, nil, map[string]string{
		"X-User-ID": r.UserID("main"),
	})
	if err != nil {
		return err
	}
	if err := expectJSONError(resp, http.StatusNotFound); err != nil {
		return fmt.Errorf("repeat delete: %w", err)
	}

	resp, err = r.do(ctx, http.MethodGet, "/api/v1/avatars/"+r.state.AvatarID+"/metadata", nil, nil)
	if err != nil {
		return err
	}
	if err := expectJSONError(resp, http.StatusNotFound); err != nil {
		return fmt.Errorf("metadata after delete: %w", err)
	}
	return nil
}

func scenarioDeleteCurrentUserAvatar(ctx context.Context, r *Runner) error {
	userID := r.UserID("delete-current")
	created, err := r.upload(ctx, userID, "avatar.png", testPNG())
	if err != nil {
		return err
	}
	resp, err := r.do(ctx, http.MethodDelete, "/api/v1/users/"+userID+"/avatar", nil, map[string]string{
		"X-User-ID": r.UserID("another"),
	})
	if err != nil {
		return err
	}
	if err := expectJSONError(resp, http.StatusForbidden); err != nil {
		return fmt.Errorf("wrong user delete current: %w", err)
	}

	resp, err = r.do(ctx, http.MethodDelete, "/api/v1/users/"+userID+"/avatar", nil, map[string]string{
		"X-User-ID": userID,
	})
	if err != nil {
		return err
	}
	if err := expectStatus(resp, http.StatusNoContent); err != nil {
		return err
	}

	resp, err = r.do(ctx, http.MethodGet, "/api/v1/avatars/"+created.ID+"/metadata", nil, nil)
	if err != nil {
		return err
	}
	return expectJSONError(resp, http.StatusNotFound)
}

func (r *Runner) upload(ctx context.Context, userID, fileName string, image []byte) (uploadResponse, error) {
	body, contentType, err := multipartBody("file", fileName, "image/jpeg", image)
	if err != nil {
		return uploadResponse{}, err
	}
	resp, err := r.do(ctx, http.MethodPost, "/api/v1/avatars", body, map[string]string{
		"Content-Type": contentType,
		"X-User-ID":    userID,
	})
	if err != nil {
		return uploadResponse{}, err
	}
	if err := expectStatus(resp, http.StatusCreated); err != nil {
		return uploadResponse{}, err
	}

	var payload uploadResponse
	if err := decodeJSON(resp.Body, &payload); err != nil {
		return uploadResponse{}, err
	}
	if payload.ID == "" || payload.UserID == "" || payload.URL == "" || payload.Status == "" || payload.CreatedAt == "" {
		return uploadResponse{}, fmt.Errorf("upload response misses required fields: %s", trimBody(resp.Body))
	}
	if payload.UserID != userID {
		return uploadResponse{}, fmt.Errorf("upload user_id=%q, want %q", payload.UserID, userID)
	}
	if !validExternalStatus(payload.Status) {
		return uploadResponse{}, fmt.Errorf("upload status=%q, want processing/completed/failed", payload.Status)
	}
	if !strings.HasPrefix(payload.URL, "/") {
		return uploadResponse{}, fmt.Errorf("upload url=%q, want relative API path", payload.URL)
	}
	return payload, nil
}

func validExternalStatus(status string) bool {
	switch status {
	case "processing", "completed", "failed":
		return true
	default:
		return false
	}
}

func requireFields(payload map[string]any, fields ...string) error {
	for _, field := range fields {
		value, ok := payload[field]
		if !ok {
			return fmt.Errorf("field %q is required", field)
		}
		if text, ok := value.(string); ok && text == "" {
			return fmt.Errorf("field %q must not be empty", field)
		}
	}
	if status, ok := payload["status"].(string); ok && !validExternalStatus(status) {
		return fmt.Errorf("status=%q, want processing/completed/failed", status)
	}
	return nil
}

func expectImage(resp *httpResponse, wantPrefix string) error {
	if len(resp.Body) == 0 {
		return fmt.Errorf("image response body is empty")
	}
	contentType := strings.ToLower(resp.ContentType)
	if !strings.HasPrefix(contentType, strings.ToLower(wantPrefix)) {
		return fmt.Errorf("content type = %q, want prefix %q", resp.ContentType, wantPrefix)
	}
	return nil
}
