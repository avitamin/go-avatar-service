package avatarservice_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestWorkerDeploymentProbesUseHealthEndpoint(t *testing.T) {
	rendered := helmTemplate(t, "-f", "values-local.yaml", "--show-only", "templates/deployment-worker.yaml")

	if !strings.Contains(rendered, "path: /health") {
		t.Fatalf("worker deployment probes do not use /health:\n%s", rendered)
	}
	if strings.Contains(rendered, "path: /metrics") {
		t.Fatalf("worker deployment probes still use /metrics:\n%s", rendered)
	}
}

func TestWorkerServiceMonitorStillScrapesMetrics(t *testing.T) {
	rendered := helmTemplate(t,
		"-f", "values-local.yaml",
		"--show-only", "templates/servicemonitor.yaml",
		"--set", "serviceMonitor.enabled=true",
	)

	if !strings.Contains(rendered, "app.kubernetes.io/component: worker") {
		t.Fatalf("rendered ServiceMonitor missing worker selector:\n%s", rendered)
	}
	if !strings.Contains(rendered, "port: metrics") || !strings.Contains(rendered, "path: /metrics") {
		t.Fatalf("worker ServiceMonitor no longer scrapes /metrics on metrics port:\n%s", rendered)
	}
}

func TestDefaultRenderRequiresExplicitCredentials(t *testing.T) {
	out, err := helmTemplateOutput(t)
	if err == nil {
		t.Fatalf("default helm template succeeded, want missing credentials error:\n%s", out)
	}
	for _, want := range []string{
		"postgresql.password is required",
		"rabbitmq.username is required",
		"rabbitmq.password is required",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("default helm template error missing %q:\n%s", want, out)
		}
	}
}

func TestLocalValuesRenderWithDemoCredentials(t *testing.T) {
	rendered := helmTemplate(t, "-f", "values-local.yaml")

	for _, want := range []string{
		"POSTGRES_DSN: \"postgres://avatars:avatars@avatar-service-postgresql:5432/avatars?sslmode=disable\"",
		"RABBITMQ_URL: \"amqp://guest:guest@avatar-service-rabbitmq:5672/\"",
		"value: \"guest\"",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("local render missing %q:\n%s", want, rendered)
		}
	}
}

func TestExplicitCredentialsRender(t *testing.T) {
	rendered := helmTemplate(t,
		"--set", "postgresql.password=strong-postgres",
		"--set", "rabbitmq.username=avatar-user",
		"--set", "rabbitmq.password=strong-rabbit",
	)

	for _, want := range []string{
		"POSTGRES_DSN: \"postgres://avatars:strong-postgres@avatar-service-postgresql:5432/avatars?sslmode=require\"",
		"RABBITMQ_URL: \"amqp://avatar-user:strong-rabbit@avatar-service-rabbitmq:5672/\"",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("explicit render missing %q:\n%s", want, rendered)
		}
	}
}

func TestDefaultValuesDoNotContainWeakPostgresOrRabbitMQCredentials(t *testing.T) {
	values, err := os.ReadFile("values.yaml")
	if err != nil {
		t.Fatalf("read values.yaml: %v", err)
	}
	text := string(values)
	for _, weakDefault := range []string{
		"password: avatars",
		"username: guest",
		"password: guest",
	} {
		if strings.Contains(text, weakDefault) {
			t.Fatalf("values.yaml still contains weak default %q:\n%s", weakDefault, text)
		}
	}
}

func helmTemplate(t *testing.T, args ...string) string {
	t.Helper()
	out, err := helmTemplateOutput(t, args...)
	if err != nil {
		t.Fatalf("helm template avatar-service . %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func helmTemplateOutput(t *testing.T, args ...string) (string, error) {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not installed")
	}
	cmdArgs := append([]string{"template", "avatar-service", "."}, args...)
	cmd := exec.Command("helm", cmdArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
