package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigInitValidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "config", "init"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	cmd = New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "config", "validate"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "config: ok") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestUnknownRuntime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "version: 1\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "status", "missing"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown runtime") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigShowJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "version: 1\nnode: dgx\nruntimes:\n  ds4:\n    type: systemd\n    service: ds4.service\nmodels: {}\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := New("test")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--config", path, "config", "show", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"node": "dgx"`) {
		t.Fatalf("output = %q", output.String())
	}
}
