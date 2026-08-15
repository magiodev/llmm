package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstalledMixed(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  a:\n    runtime: example\n    path: /a\n  b:\n    runtime: example\n    path: /b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "installed.yaml"), []byte("version: 1\nmodels:\n  a:\n    path: /a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := New("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config", cfgPath, "installed"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("installed: %v", err)
	}
	if !strings.Contains(out.String(), "a\tinstalled\t/a\n") || !strings.Contains(out.String(), "b\tmissing\n") {
		t.Fatalf("output = %q", out.String())
	}
	if !strings.Contains(out.String(), "total 2 (1 installed, 1 missing)\n") {
		t.Fatalf("summary = %q", out.String())
	}
}

func TestInstalledAllMissing(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  a:\n    runtime: example\n    path: /a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// No installed.yaml exists -> empty state.
	cmd := New("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config", cfgPath, "installed"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("installed: %v", err)
	}
	if !strings.Contains(out.String(), "a\tmissing\n") || !strings.Contains(out.String(), "total 1 (0 installed, 1 missing)\n") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestInstalledConfigError(t *testing.T) {
	cmd := New("test")
	cmd.SetArgs([]string{"--config", "/nonexistent.yaml", "installed"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected config error")
	}
}

func TestInstalledStateError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  a:\n    runtime: example\n    path: /a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "installed.yaml"), []byte("version: [broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := New("test")
	cmd.SetArgs([]string{"--config", cfgPath, "installed"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected install-state error")
	}
}
