package app

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifySuccess(t *testing.T) {
	data := []byte("model")
	model := filepath.Join(t.TempDir(), "m.gguf")
	if err := os.WriteFile(model, data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n    size: %d\n    sha256: %s\n", model, len(data), sum))
	cmd := New("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config", path, "verify"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.Contains(out.String(), "sha256 model") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestVerifyMissing(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n    size: 5\n", filepath.Join(dir, "nope.gguf")))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "verify"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "verify found") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifySizeMismatch(t *testing.T) {
	data := []byte("model")
	model := filepath.Join(t.TempDir(), "m.gguf")
	if err := os.WriteFile(model, data, 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n    size: 999\n", model))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "verify"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "verify found") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyShaMismatch(t *testing.T) {
	data := []byte("model")
	model := filepath.Join(t.TempDir(), "m.gguf")
	if err := os.WriteFile(model, data, 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n    size: %d\n    sha256: %s\n", model, len(data), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "verify"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyNotRegular(t *testing.T) {
	modelDir := filepath.Join(t.TempDir(), "m")
	if err := os.Mkdir(modelDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n    size: 5\n", modelDir))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "verify"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "verify found") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyNoModels(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels: {}\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "verify"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerifySizeZero(t *testing.T) {
	data := []byte("model")
	model := filepath.Join(t.TempDir(), "m.gguf")
	if err := os.WriteFile(model, data, 0o600); err != nil {
		t.Fatal(err)
	}
	// No size, no sha: only path presence is verified.
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n", model))
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "verify"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerifyArtifactList(t *testing.T) {
	data := []byte("artifact")
	primary := filepath.Join(t.TempDir(), "m.gguf")
	artifact := filepath.Join(t.TempDir(), "a.bin")
	if err := os.WriteFile(primary, []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, data, 0o600); err != nil {
		t.Fatal(err)
	}
	good := fmt.Sprintf("%x", sha256.Sum256(data))
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n    artifacts:\n      - path: %s\n        size: %d\n        sha256: %s\n", primary, artifact, len(data), good))
	cmd := New("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config", path, "verify"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.Contains(out.String(), "sha256 model m artifact 0") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestVerifyConfigError(t *testing.T) {
	cmd := New("test")
	cmd.SetArgs([]string{"--config", "/nonexistent.yaml", "verify"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error")
	}
}

func TestVerifyJSON(t *testing.T) {
	data := []byte("model")
	model := filepath.Join(t.TempDir(), "m.gguf")
	if err := os.WriteFile(model, data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n    size: %d\n    sha256: %s\n", model, len(data), sum))
	cmd := New("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config", path, "verify", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.Contains(out.String(), "\"success\": true") || !strings.Contains(out.String(), "\"ok\": true") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestVerifyJSONWriteError(t *testing.T) {
	data := []byte("model")
	model := filepath.Join(t.TempDir(), "m.gguf")
	if err := os.WriteFile(model, data, 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n    size: %d\n", model, len(data)))
	cmd := New("test")
	cmd.SetOut(errWriter{})
	cmd.SetArgs([]string{"--config", path, "verify", "--format", "json"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyHashError(t *testing.T) {
	data := []byte("model")
	model := filepath.Join(t.TempDir(), "m.gguf")
	if err := os.WriteFile(model, data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	path := writeManifest(t, fmt.Sprintf("version: 1\nruntimes:\n  web:\n    type: systemd\n    service: web.service\nmodels:\n  m:\n    runtime: web\n    path: %s\n    size: %d\n    sha256: %s\n", model, len(data), sum))
	old := openFile
	openFile = func(string) (*os.File, error) { return nil, errors.New("boom") }
	defer func() { openFile = old }()
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "verify"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("error = %v", err)
	}
}
