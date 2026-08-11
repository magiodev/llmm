package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/magiodev/llmm/internal/config"
)

func loadConfig(t *testing.T, path string) *config.Config {
	t.Helper()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestPreflightArtifactsPresent(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "m.gguf")
	if err := os.WriteFile(model, []byte("m"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n", model))
	cfg := loadConfig(t, path)
	if missing := preflightArtifacts(cfg, "web"); len(missing) != 0 {
		t.Fatalf("missing = %v", missing)
	}
}

func TestPreflightArtifactsMissing(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n", filepath.Join(dir, "nope.gguf")))
	cfg := loadConfig(t, path)
	if missing := preflightArtifacts(cfg, "web"); len(missing) != 1 || missing[0] != "m" {
		t.Fatalf("missing = %v", missing)
	}
}

func TestPreflightArtifactsNotRegular(t *testing.T) {
	dir := t.TempDir()
	modelDir := filepath.Join(dir, "m")
	if err := os.Mkdir(modelDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n", modelDir))
	cfg := loadConfig(t, path)
	if missing := preflightArtifacts(cfg, "web"); len(missing) != 1 {
		t.Fatalf("missing = %v", missing)
	}
}

func TestPreflightArtifactsOtherRuntime(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\n  db:\n    type: systemd\n    service: db.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n", filepath.Join(dir, "nope.gguf")))
	cfg := loadConfig(t, path)
	// m is bound to web; db has no models -> empty missing.
	if missing := preflightArtifacts(cfg, "db"); len(missing) != 0 {
		t.Fatalf("missing = %v", missing)
	}
}

func TestPreflightStatError(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n", filepath.Join(dir, "m.gguf")))
	cfg := loadConfig(t, path)
	old := osStat
	osStat = func(string) (os.FileInfo, error) { return nil, errors.New("stat") }
	defer func() { osStat = old }()
	if missing := preflightArtifacts(cfg, "web"); len(missing) != 1 {
		t.Fatalf("missing = %v", missing)
	}
}

func TestActionStartMissingArtifact(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n", filepath.Join(dir, "nope.gguf")))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "start", "web"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "model artifacts missing") {
		t.Fatalf("error = %v", err)
	}
}

func TestActionStartArtifactPresent(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'active\n'`)
	dir := t.TempDir()
	model := filepath.Join(dir, "m.gguf")
	if err := os.WriteFile(model, []byte("m"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n", model))
	cmd := New("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config", path, "start", "web"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !strings.Contains(out.String(), "web: active") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestActionRestartMissingArtifact(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n", filepath.Join(dir, "nope.gguf")))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "restart", "web"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "model artifacts missing") {
		t.Fatalf("error = %v", err)
	}
}

func TestActionStopIgnoresPreflight(t *testing.T) {
	installFakeCommand(t, "systemctl", `printf 'inactive\n'`)
	dir := t.TempDir()
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n", filepath.Join(dir, "nope.gguf")))
	cmd := New("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config", path, "stop", "web"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if !strings.Contains(out.String(), "web: inactive") {
		t.Fatalf("output = %q", out.String())
	}
}
