package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"go-avatar-service/internal/service"
)

func TestHTTPUploadValidationAndSuccess(t *testing.T) {
	svc := service.NewAvatarService(service.NewMemoryRepository(), service.NewMemoryStorage(), noopBroker{})
	server := httptest.NewServer(NewRouter(svc, staticHealthService{snapshot: service.HealthSnapshot{
		Status: service.HealthStatusOK,
		Components: map[string]string{
			"postgres": service.HealthStatusOK,
			"minio":    service.HealthStatusOK,
			"rabbitmq": service.HealthStatusOK,
		},
	}}))
	defer server.Close()

	tests := []struct {
		name   string
		userID string
		field  string
		data   []byte
		status int
	}{
		{name: "missing user", field: "file", data: testJPEG(t), status: http.StatusBadRequest},
		{name: "invalid user", userID: "bad user", field: "file", data: testJPEG(t), status: http.StatusBadRequest},
		{name: "wrong field", userID: "user-1", field: "image", data: testJPEG(t), status: http.StatusBadRequest},
		{name: "invalid image", userID: "user-1", field: "file", data: []byte("nope"), status: http.StatusBadRequest},
		{name: "too large", userID: "user-1", field: "file", data: append(testJPEG(t), make([]byte, MaxUploadBytes+1)...), status: http.StatusRequestEntityTooLarge},
		{name: "success", userID: "user-1", field: "file", data: testJPEG(t), status: http.StatusCreated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, contentType := multipartPayload(t, tt.field, "avatar.jpg", "image/jpeg", tt.data)
			req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/avatars", body)
			req.Header.Set("Content-Type", contentType)
			if tt.userID != "" {
				req.Header.Set("X-User-ID", tt.userID)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.status {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.status)
			}
			if tt.status >= 400 {
				assertErrorModel(t, resp)
			}
		})
	}
}

func TestHTTPReadListDeleteWebAndHealth(t *testing.T) {
	svc := service.NewAvatarService(service.NewMemoryRepository(), service.NewMemoryStorage(), noopBroker{})
	router := NewRouter(svc, staticHealthService{snapshot: service.HealthSnapshot{
		Status: service.HealthStatusDegraded,
		Components: map[string]string{
			"postgres": service.HealthStatusOK,
			"minio":    service.HealthStatusDegraded,
			"rabbitmq": service.HealthStatusOK,
		},
	}})
	server := httptest.NewServer(router)
	defer server.Close()

	createdID := uploadHTTP(t, server.URL, "user-1")

	assertHealth(t, server.URL+"/health", service.HealthStatusDegraded, map[string]string{
		"postgres": service.HealthStatusOK,
		"minio":    service.HealthStatusDegraded,
		"rabbitmq": service.HealthStatusOK,
	})
	checkStatus(t, http.MethodGet, server.URL+"/api/v1/avatars/"+createdID, nil, http.StatusOK)
	checkStatus(t, http.MethodGet, server.URL+"/api/v1/avatars/"+createdID+"?size=42x42", nil, http.StatusBadRequest)
	checkStatus(t, http.MethodGet, server.URL+"/api/v1/avatars/"+createdID+"?format=webp", nil, http.StatusBadRequest)
	checkStatus(t, http.MethodGet, server.URL+"/api/v1/avatars/"+createdID+"/metadata", nil, http.StatusOK)
	checkStatus(t, http.MethodGet, server.URL+"/api/v1/users/user-1/avatars", nil, http.StatusOK)
	checkStatus(t, http.MethodGet, server.URL+"/api/v1/users/user-1/avatar", nil, http.StatusOK)
	checkStatus(t, http.MethodGet, server.URL+"/web/upload", nil, http.StatusOK)
	checkStatus(t, http.MethodGet, server.URL+"/web/gallery/bad user", nil, http.StatusBadRequest)
	checkStatus(t, http.MethodGet, server.URL+"/web/gallery/missing", nil, http.StatusNotFound)
	checkStatus(t, http.MethodGet, server.URL+"/web/gallery/user-1", nil, http.StatusOK)

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/avatars/"+createdID, nil)
	req.Header.Set("X-User-ID", "intruder")
	checkReqStatus(t, req, http.StatusForbidden)
	req, _ = http.NewRequest(http.MethodDelete, server.URL+"/api/v1/avatars/"+createdID, nil)
	req.Header.Set("X-User-ID", "user-1")
	checkReqStatus(t, req, http.StatusNoContent)
	checkStatus(t, http.MethodGet, server.URL+"/api/v1/avatars/"+createdID+"/metadata", nil, http.StatusNotFound)
}

func uploadHTTP(t *testing.T, baseURL, userID string) string {
	t.Helper()
	body, contentType := multipartPayload(t, "file", "avatar.jpg", "image/jpeg", testJPEG(t))
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/avatars", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-User-ID", userID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d", resp.StatusCode)
	}
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	return payload.ID
}

func checkStatus(t *testing.T, method, url string, body io.Reader, want int) {
	t.Helper()
	req, _ := http.NewRequest(method, url, body)
	checkReqStatus(t, req, want)
}

func checkReqStatus(t *testing.T, req *http.Request, want int) {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		t.Fatalf("%s %s status = %d, want %d", req.Method, req.URL.Path, resp.StatusCode, want)
	}
	if want >= 400 {
		assertErrorModel(t, resp)
	}
}

func assertErrorModel(t *testing.T, resp *http.Response) {
	t.Helper()
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error model: %v", err)
	}
	if payload.Error.Code == "" || payload.Error.Message == "" {
		t.Fatalf("bad error model: %+v", payload)
	}
}

func multipartPayload(t *testing.T, field, fileName, contentType string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="`+field+`"; filename="`+fileName+`"`)
	header.Set("Content-Type", contentType)
	part, err := w.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, w.FormDataContentType()
}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 30, G: uint8(x * 40), B: uint8(y * 40), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type noopBroker struct{}

func (noopBroker) Publish(context.Context, string, []byte, string) error { return nil }

type staticHealthService struct {
	snapshot service.HealthSnapshot
}

func (s staticHealthService) Check(context.Context) service.HealthSnapshot {
	return s.snapshot
}

func assertHealth(t *testing.T, url, wantStatus string, wantComponents map[string]string) {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status code = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var payload struct {
		Status     string            `json:"status"`
		Components map[string]string `json:"components"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if payload.Status != wantStatus {
		t.Fatalf("health status = %q, want %q", payload.Status, wantStatus)
	}
	for component, want := range wantComponents {
		if payload.Components[component] != want {
			t.Fatalf("%s = %q, want %q", component, payload.Components[component], want)
		}
	}
}

func TestUploadPageUsesFileField(t *testing.T) {
	page := UploadPageHTML()
	if !strings.Contains(page, `name="file"`) {
		t.Fatalf("upload page must use multipart field file")
	}
	if strings.Contains(page, `name="image"`) || strings.Contains(page, `append('image'`) {
		t.Fatalf("upload page must not use image field")
	}
}
