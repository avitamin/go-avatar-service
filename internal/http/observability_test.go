package httpapi

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"go-avatar-service/internal/observability"
	"go-avatar-service/internal/service"
)

func TestMetricsEndpointAndRouteLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	svc := service.NewAvatarService(service.NewMemoryRepository(), service.NewMemoryStorage(), noopBroker{})
	router := NewRouter(
		svc,
		staticHealthService{snapshot: service.HealthSnapshot{Status: service.HealthStatusOK}},
		WithObservability(observability.RouterOptions{Metrics: metrics}),
	)
	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/health status = %d, want 200", resp.StatusCode)
	}

	resp, err = http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "http_requests_total") {
		t.Fatalf("metrics missing http_requests_total:\n%s", text)
	}
	if !strings.Contains(text, `route="/health"`) {
		t.Fatalf("metrics missing route template label:\n%s", text)
	}
	if strings.Contains(text, `route="unknown"`) {
		t.Fatalf("metrics must not use unknown route labels:\n%s", text)
	}
}

func TestMetricsUseRouteTemplateForDynamicRoutes(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	svc := service.NewAvatarService(service.NewMemoryRepository(), service.NewMemoryStorage(), noopBroker{})
	server := httptest.NewServer(NewRouter(
		svc,
		staticHealthService{snapshot: service.HealthSnapshot{Status: service.HealthStatusOK}},
		WithObservability(observability.RouterOptions{Metrics: metrics}),
	))
	defer server.Close()

	body, contentType := multipartPayload(t, "file", "avatar.jpg", "image/jpeg", testJPEG(t))
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/avatars", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-User-ID", "route-template-user")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d, want 201", resp.StatusCode)
	}
	createdID := uploadHTTP(t, server.URL, "route-template-user")
	resp, err = http.Get(server.URL + "/api/v1/avatars/" + createdID + "/metadata")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metadata status = %d, want 200", resp.StatusCode)
	}

	resp, err = http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	metricsBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(metricsBody)
	if !strings.Contains(text, `route="/api/v1/avatars/{avatar_id}/metadata"`) {
		t.Fatalf("metrics missing route template label:\n%s", text)
	}
	if strings.Contains(text, createdID+`/metadata`) {
		t.Fatalf("metrics leaked raw avatar id in route label:\n%s", text)
	}
}

func TestRejectedUploadRecordsUploadErrorMetric(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)
	svc := service.NewAvatarService(service.NewMemoryRepository(), service.NewMemoryStorage(), noopBroker{}, service.WithObservability(metrics))
	server := httptest.NewServer(NewRouter(
		svc,
		staticHealthService{snapshot: service.HealthSnapshot{Status: service.HealthStatusOK}},
		WithObservability(observability.RouterOptions{Metrics: metrics}),
	))
	defer server.Close()

	body, contentType := multipartPayload(t, "file", "avatar.jpg", "image/jpeg", []byte("not an image"))
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/avatars", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-User-ID", "metrics-user")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("upload status = %d, want 400", resp.StatusCode)
	}

	resp, err = http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	metricsBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(metricsBody, []byte(`avatars_uploads_total{mime_type="unknown",status="error"} 1`)) {
		t.Fatalf("rejected upload metric missing:\n%s", string(metricsBody))
	}
}
