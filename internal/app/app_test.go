package app

import (
	"bytes"
	"testing"
)

func TestCLIContract(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "server", args: []string{"avatars-service", "server", "--check"}},
		{name: "worker", args: []string{"avatars-service", "worker", "--check"}},
		{name: "migrate up", args: []string{"avatars-service", "migrate", "up", "--check"}},
		{name: "migrate down", args: []string{"avatars-service", "migrate", "down", "--check"}},
		{name: "migrate status", args: []string{"avatars-service", "migrate", "status", "--check"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Run(tt.args, &bytes.Buffer{}); err != nil {
				t.Fatalf("Run(%v) error = %v", tt.args, err)
			}
		})
	}
}

func TestCLIDoesNotRunMigrationsOnServerOrWorker(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"avatars-service", "server", "--check"}, &out); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out.Bytes(), []byte("migrate")) {
		t.Fatalf("server output mentions migrations: %s", out.String())
	}
	out.Reset()
	if err := Run([]string{"avatars-service", "worker", "--check"}, &out); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out.Bytes(), []byte("migrate")) {
		t.Fatalf("worker output mentions migrations: %s", out.String())
	}
}
