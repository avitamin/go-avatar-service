package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"go-avatar-service/internal/domain"
	"go-avatar-service/internal/imageproc"
	"go-avatar-service/internal/service"
)

const MaxUploadBytes = 10 * 1024 * 1024

func NewRouter(svc *service.AvatarService, health service.RuntimeHealthChecker) http.Handler {
	r := chi.NewRouter()
	h := &handler{svc: svc, healthSvc: health}
	r.Get("/health", h.health)
	r.Post("/api/v1/avatars", h.upload)
	r.Get("/api/v1/avatars/{avatar_id}", h.readAvatar)
	r.Get("/api/v1/avatars/{avatar_id}/metadata", h.metadata)
	r.Delete("/api/v1/avatars/{avatar_id}", h.deleteAvatar)
	r.Get("/api/v1/users/{user_id}/avatars", h.listUserAvatars)
	r.Get("/api/v1/users/{user_id}/avatar", h.readUserAvatar)
	r.Delete("/api/v1/users/{user_id}/avatar", h.deleteUserAvatar)
	r.Get("/web/upload", h.webUpload)
	r.Get("/web/gallery/{user_id}", h.webGallery)
	return accessLog(r)
}

type handler struct {
	svc       *service.AvatarService
	healthSvc service.RuntimeHealthChecker
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	snapshot := h.healthSvc.Check(r.Context())
	writeJSON(w, http.StatusOK, snapshot)
}

func (h *handler) upload(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if err := domain.ValidateUserID(userID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_user_id", "Invalid X-User-ID", nil)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes+1024*1024)
	if err := r.ParseMultipartForm(MaxUploadBytes + 1); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart", "Invalid multipart form", nil)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file_required", "Multipart field file is required", nil)
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, MaxUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_file_failed", "Could not read uploaded file", nil)
		return
	}
	if len(data) > MaxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", "File is larger than 10 MB", nil)
		return
	}
	mime, err := imageproc.Sniff(data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "unsupported_image", "Unsupported image bytes", nil)
		return
	}
	if mime != "image/webp" {
		if _, err := imageproc.Decode(data, mime); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_image", "Image cannot be decoded", nil)
			return
		}
	}
	created, err := h.svc.Upload(r.Context(), service.UploadInput{UserID: userID, FileName: header.Filename, Content: data, ContentType: mime})
	if err != nil {
		h.writeMappedError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *handler) readAvatar(w http.ResponseWriter, r *http.Request) {
	size, ok := parseReadQuery(w, r)
	if !ok {
		return
	}
	data, mime, err := h.svc.ReadAvatar(r.Context(), chi.URLParam(r, "avatar_id"), size)
	if err != nil {
		h.writeMappedError(w, err)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *handler) metadata(w http.ResponseWriter, r *http.Request) {
	meta, err := h.svc.Metadata(r.Context(), chi.URLParam(r, "avatar_id"))
	if err != nil {
		h.writeMappedError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (h *handler) listUserAvatars(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListByUser(r.Context(), chi.URLParam(r, "user_id"))
	if err != nil {
		h.writeMappedError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *handler) readUserAvatar(w http.ResponseWriter, r *http.Request) {
	size, ok := parseReadQuery(w, r)
	if !ok {
		return
	}
	data, _, err := h.svc.ReadUserAvatar(r.Context(), chi.URLParam(r, "user_id"), size)
	if err != nil {
		h.writeMappedError(w, err)
		return
	}
	mime, _ := imageproc.Sniff(data)
	w.Header().Set("Content-Type", mime)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *handler) deleteAvatar(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteByID(r.Context(), chi.URLParam(r, "avatar_id"), r.Header.Get("X-User-ID")); err != nil {
		h.writeMappedError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) deleteUserAvatar(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteCurrentUserAvatar(r.Context(), chi.URLParam(r, "user_id"), r.Header.Get("X-User-ID")); err != nil {
		h.writeMappedError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) webUpload(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, UploadPageHTML())
}

func (h *handler) webGallery(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	if err := domain.ValidateUserID(userID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_user_id", "Invalid user_id", nil)
		return
	}
	items, hadRecords, err := h.svc.GalleryByUser(r.Context(), userID)
	if err != nil {
		h.writeMappedError(w, err)
		return
	}
	if !hadRecords {
		writeError(w, http.StatusNotFound, "not_found", "User avatars not found", nil)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, "<!doctype html><html><body><h1>Gallery %s</h1><ul>", userID)
	for _, item := range items {
		_, _ = fmt.Fprintf(w, `<li><img src="%s" alt="%s"></li>`, item.URL, item.ID)
	}
	_, _ = io.WriteString(w, "</ul></body></html>")
}

func (h *handler) writeMappedError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidUserID):
		writeError(w, http.StatusBadRequest, "invalid_user_id", "Invalid user id", nil)
	case errors.Is(err, service.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "Forbidden", nil)
	case errors.Is(err, service.ErrNotFound), errors.Is(err, service.ErrObjectNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Not found", nil)
	case errors.Is(err, service.ErrVariantNotReady):
		writeError(w, http.StatusConflict, "variant_not_ready", "Variant is not ready", nil)
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal error", nil)
	}
}

func parseReadQuery(w http.ResponseWriter, r *http.Request) (domain.Size, bool) {
	if r.URL.Query().Has("format") {
		writeError(w, http.StatusBadRequest, "unsupported_format", "format query is not supported", nil)
		return "", false
	}
	size, err := domain.ParseSize(r.URL.Query().Get("size"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_size", "Unsupported size", map[string]any{"allowed": []string{"original", "100x100", "300x300"}})
		return "", false
	}
	return size, true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string, details any) {
	payload := map[string]any{"error": map[string]any{"code": code, "message": message}}
	if details != nil {
		payload["error"].(map[string]any)["details"] = details
	}
	writeJSON(w, status, payload)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = r.Header.Get("X-Correlation-ID")
		}
		slog.Info("http access", "method", r.Method, "path", r.URL.Path, "status", sw.status, "duration", time.Since(start).String(), "request_id", requestID)
	})
}

func UploadPageHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"><title>Avatar Upload</title></head>
<body>
<h1>Avatar Upload</h1>
<form id="uploadForm">
<label>User ID <input type="text" id="userId" name="userId" required></label>
<label>Choose Image <input type="file" id="file" name="file" accept="image/*" required></label>
<button type="submit">Upload Avatar</button>
</form>
<pre id="response"></pre>
<script>
document.getElementById('uploadForm').addEventListener('submit', async function (event) {
  event.preventDefault();
  const formData = new FormData();
  formData.append('file', document.getElementById('file').files[0]);
  const response = await fetch('/api/v1/avatars', { method: 'POST', headers: { 'X-User-ID': document.getElementById('userId').value }, body: formData });
  document.getElementById('response').textContent = await response.text();
});
</script>
</body>
</html>`
}
