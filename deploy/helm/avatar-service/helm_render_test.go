package avatarservice_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestWorkerDeploymentProbesUseHealthEndpoint(t *testing.T) {
	rendered := helmTemplate(t, "--show-only", "templates/deployment-worker.yaml")

	if !strings.Contains(rendered, "path: /health") {
		t.Fatalf("worker deployment probes do not use /health:\n%s", rendered)
	}
	if strings.Contains(rendered, "path: /metrics") {
		t.Fatalf("worker deployment probes still use /metrics:\n%s", rendered)
	}
}

func TestWorkerServiceMonitorStillScrapesMetrics(t *testing.T) {
	rendered := helmTemplate(t,
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

func helmTemplate(t *testing.T, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not installed")
	}
	cmdArgs := append([]string{"template", "avatar-service", "."}, args...)
	cmd := exec.Command("helm", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helm %s failed: %v\n%s", strings.Join(cmdArgs, " "), err, string(out))
	}
	return string(out)
}
