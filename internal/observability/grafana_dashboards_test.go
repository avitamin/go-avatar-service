package observability_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestObservabilityStackProvisionsGrafanaDashboards(t *testing.T) {
	root := repoRoot(t)

	dashboardDir := filepath.Join(root, "configs/observability/grafana/dashboards")
	wantDashboards := []string{
		"avatar-service-overview.json",
		"avatar-business-kpis.json",
		"avatar-infrastructure.json",
	}
	for _, name := range wantDashboards {
		t.Run(name, func(t *testing.T) {
			body := readFile(t, filepath.Join(dashboardDir, name))
			var dashboard map[string]any
			if err := json.Unmarshal(body, &dashboard); err != nil {
				t.Fatalf("dashboard must be valid JSON: %v", err)
			}
			if dashboard["title"] == "" {
				t.Fatalf("dashboard must have a title")
			}
			if !strings.Contains(string(body), `"uid": "prometheus"`) {
				t.Fatalf("dashboard must use provisioned Prometheus datasource uid")
			}
			for _, want := range []string{`"name": "service"`, `"name": "interval"`} {
				if !strings.Contains(string(body), want) {
					t.Fatalf("dashboard must define variable %s", want)
				}
			}
		})
	}

	datasources := readFile(t, filepath.Join(root, "configs/observability/grafana/provisioning/datasources/datasources.yml"))
	for _, want := range []string{"uid: prometheus", "uid: jaeger", "uid: loki"} {
		if !strings.Contains(string(datasources), want) {
			t.Fatalf("datasource provisioning missing %q", want)
		}
	}

	provider := readFile(t, filepath.Join(root, "configs/observability/grafana/provisioning/dashboards/dashboards.yml"))
	if !strings.Contains(string(provider), "path: /var/lib/grafana/dashboards") {
		t.Fatalf("dashboard provider must use the dashboard mount path")
	}
}

func TestObservabilityStackScrapesInfrastructureMetrics(t *testing.T) {
	root := repoRoot(t)

	prometheus := readFile(t, filepath.Join(root, "configs/observability/prometheus/prometheus.yml"))
	for _, want := range []string{"job_name: postgres", "postgres-exporter:9187", "job_name: rabbitmq", "rabbitmq:15692"} {
		if !strings.Contains(string(prometheus), want) {
			t.Fatalf("prometheus config missing %q", want)
		}
	}

	var compose map[string]any
	if err := yaml.Unmarshal(readFile(t, filepath.Join(root, "docker-compose.observability.yml")), &compose); err != nil {
		t.Fatalf("observability compose must be valid YAML: %v", err)
	}
	services, ok := compose["services"].(map[string]any)
	if !ok {
		t.Fatalf("observability compose must define services")
	}
	if _, ok := services["postgres-exporter"]; !ok {
		t.Fatalf("observability compose must define postgres-exporter service")
	}
	if _, ok := services["rabbitmq"].(map[string]any); !ok {
		t.Fatalf("observability compose must override rabbitmq service")
	}
	rabbitmqText := string(readFile(t, filepath.Join(root, "docker-compose.observability.yml")))
	for _, want := range []string{"rabbitmq_prometheus", "${COMPOSE_RABBITMQ_METRICS_PORT:-15692}:15692"} {
		if !strings.Contains(rabbitmqText, want) {
			t.Fatalf("rabbitmq observability override missing %q", want)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatal("go.mod not found")
		}
		dir = next
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
