package app

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func serveContent(t *testing.T, content []byte) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestInstallCompletion(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  a:\n    runtime: example\n    path: /a\n    source: https://example.com/a\n  b:\n    runtime: example\n    path: /b\n")
	cmd := New("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config", path, "__complete", "install", "inst"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "a\n") || strings.Contains(out.String(), "b\n") {
		t.Fatalf("completion output = %q", out.String())
	}
}

func TestInstallCompletionConfigError(t *testing.T) {
	cmd := New("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config", "/nonexistent.yaml", "__complete", "install", "x"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), ":1") {
		t.Fatalf("expected error directive, got %q", out.String())
	}
}

func TestInstallSuccess(t *testing.T) {
	content := []byte("hello model")
	sum := fmt.Sprintf("%x", sha256.Sum256(content))
	url := serveContent(t, content)
	dir := t.TempDir()
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  m:\n    runtime: example\n    path: "+filepath.Join(dir, "model.gguf")+"\n    source: "+url+"\n    size: "+fmt.Sprint(len(content))+"\n    sha256: "+sum+"\n")
	cmd := New("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config", path, "install", "m"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install: %v", err)
	}
	if out.String() != "installed m\n" {
		t.Fatalf("output = %q", out.String())
	}
	st := filepath.Join(filepath.Dir(path), "installed.yaml")
	if _, err := os.Stat(st); err != nil {
		t.Fatalf("install state not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "model.gguf")); err != nil {
		t.Fatalf("model not installed: %v", err)
	}
}

func TestInstallQuiet(t *testing.T) {
	content := []byte("q")
	url := serveContent(t, content)
	dir := t.TempDir()
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  m:\n    runtime: example\n    path: "+filepath.Join(dir, "m")+"\n    source: "+url+"\n")
	cmd := New("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--quiet", "--config", path, "install", "m"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("quiet output = %q", out.String())
	}
}

func TestInstallConfigError(t *testing.T) {
	cmd := New("test")
	cmd.SetArgs([]string{"--config", "/nonexistent.yaml", "install", "m"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected config error")
	}
}

func TestInstallUnknownModel(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  a:\n    runtime: example\n    path: /a\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "install", "nope"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown model") {
		t.Fatalf("error = %v", err)
	}
}

func TestInstallNoSource(t *testing.T) {
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  a:\n    runtime: example\n    path: /a\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "install", "a"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "no source") {
		t.Fatalf("error = %v", err)
	}
}

func TestInstallFetchError(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  m:\n    runtime: example\n    path: "+filepath.Join(dir, "m")+"\n    source: http://127.0.0.1:1/m\n")
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "install", "m"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected fetch error")
	}
}

func TestInstallRecordError(t *testing.T) {
	content := []byte("r")
	url := serveContent(t, content)
	dir := t.TempDir()
	cfgDir := t.TempDir()
	path := filepath.Join(cfgDir, "config.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels:\n  m:\n    runtime: example\n    path: "+filepath.Join(dir, "m")+"\n    source: "+url+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Make the install-state path a symlink so Record fails with a symlink error.
	real := filepath.Join(cfgDir, "real.yaml")
	if err := os.WriteFile(real, []byte("version: 1\nmodels: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(cfgDir, "installed.yaml")); err != nil {
		t.Fatal(err)
	}
	cmd := New("test")
	cmd.SetArgs([]string{"--config", path, "install", "m"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error = %v", err)
	}
}
