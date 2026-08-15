package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validConfig() *Config {
	return &Config{Version: Version, Runtimes: map[string]Runtime{"example": {Type: "systemd", Service: "example.service"}}, Models: map[string]Model{"flash": {Runtime: "example", Format: "gguf", Path: "/models/flash.gguf"}}}
}
func TestValidate(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"unknown runtime", func(c *Config) { c.Models["flash"] = Model{Runtime: "missing", Path: "/model"} }, "unknown runtime"},
		{"negative size", func(c *Config) { m := c.Models["flash"]; m.Size = -1; c.Models["flash"] = m }, "size must not be negative"},
		{"negative context", func(c *Config) { m := c.Models["flash"]; m.Context = -1; c.Models["flash"] = m }, "context must not be negative"},
		{"negative output", func(c *Config) { m := c.Models["flash"]; m.Output = -1; c.Models["flash"] = m }, "output must not be negative"},
		{"invalid checksum", func(c *Config) { m := c.Models["flash"]; m.SHA256 = "nope"; c.Models["flash"] = m }, "sha256 must be 64 hexadecimal"},
		{"empty runtime name", func(c *Config) { c.Runtimes[""] = Runtime{Type: "docker", Container: "one"} }, "runtime name must not be empty"},
		{"empty model name", func(c *Config) { c.Models[""] = Model{Runtime: "example", Path: "/model"} }, "model name must not be empty"},
		{"systemd container", func(c *Config) {
			c.Runtimes["example"] = Runtime{Type: "systemd", Service: "example.service", Container: "wrong"}
		}, "cannot set container"},
		{"docker service", func(c *Config) {
			c.Runtimes["example"] = Runtime{Type: "docker", Container: "example", Service: "wrong"}
		}, "cannot set service"},
		{"leading dash", func(c *Config) { c.Runtimes["example"] = Runtime{Type: "systemd", Service: "--system"} }, "must not start"},
		{"invalid endpoint", func(c *Config) {
			c.Runtimes["example"] = Runtime{Type: "systemd", Service: "example.service", Endpoint: "not-a-url"}
		}, "absolute URL"},
		{"credential endpoint", func(c *Config) {
			c.Runtimes["example"] = Runtime{Type: "systemd", Service: "example.service", Endpoint: "https://user:secret@example.test/v1"}
		}, "must not contain credentials"},
		{"empty reasoning level", func(c *Config) {
			m := c.Models["flash"]
			m.Reasoning = []string{""}
			c.Models["flash"] = m
		}, "empty reasoning level"},
		{"artifact missing path", func(c *Config) {
			m := c.Models["flash"]
			m.Artifacts = []Artifact{{Path: ""}}
			c.Models["flash"] = m
		}, "artifact 0 requires path"},
		{"artifact negative size", func(c *Config) {
			m := c.Models["flash"]
			m.Artifacts = []Artifact{{Path: "/a", Size: -1}}
			c.Models["flash"] = m
		}, "artifact 0 size must not be negative"},
		{"artifact invalid checksum", func(c *Config) {
			m := c.Models["flash"]
			m.Artifacts = []Artifact{{Path: "/a", SHA256: "nope"}}
			c.Models["flash"] = m
		}, "artifact 0 sha256 must be 64 hexadecimal"},
		{"source whitespace", func(c *Config) {
			m := c.Models["flash"]
			m.Source = "owner/my model"
			c.Models["flash"] = m
		}, "source must not contain whitespace"},
		{"source leading dash", func(c *Config) {
			m := c.Models["flash"]
			m.Source = "-owner/model"
			c.Models["flash"] = m
		}, "source must not start with '-'"},
		{"source credentials", func(c *Config) {
			m := c.Models["flash"]
			m.Source = "https://user:secret@example.test/v1"
			c.Models["flash"] = m
		}, "source must not contain credentials"},
		{"default model unknown", func(c *Config) {
			c.DefaultModel = "missing"
		}, "default_model \"missing\" references unknown model"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			test.mutate(cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
func TestValidateSourceOK(t *testing.T) {
	cfg := validConfig()
	m := cfg.Models["flash"]
	m.Source = "owner/example-model-gguf"
	cfg.Models["flash"] = m
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid source rejected: %v", err)
	}
}
func TestWriteLoadAndKnownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, validConfig(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, validConfig(), false); err == nil {
		t.Fatal("expected overwrite refusal")
	}
	if err := os.WriteFile(path, []byte("version: 1\nruntimes: {}\nmodels: {}\nunknown: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}
func TestLoadRejectsTrailingDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\n---\nunknown: true\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "multiple YAML documents") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestForceWriteRestoresPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, validConfig(), true); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want 600", got)
	}
}
func TestForceWriteRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(victim, []byte("untouched"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, path); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, validConfig(), true); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(victim)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "untouched" {
		t.Fatalf("victim = %q", data)
	}
}
func TestForceWritePreservesDestinationOnRenameFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, validConfig(), true); err == nil {
		t.Fatal("expected rename failure")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("destination directory was replaced")
	}
}
func TestDefaultPathEnv(t *testing.T) {
	t.Setenv("LLMM_CONFIG", "/tmp/custom.yaml")
	if got := DefaultPath(); got != "/tmp/custom.yaml" {
		t.Fatalf("DefaultPath = %q", got)
	}
}
func TestDefaultPathUserConfigDirError(t *testing.T) {
	os.Unsetenv("LLMM_CONFIG")
	old := userConfigDir
	userConfigDir = func() (string, error) { return "", errors.New("boom") }
	defer func() { userConfigDir = old }()
	if got := DefaultPath(); got != "config.yaml" {
		t.Fatalf("DefaultPath = %q", got)
	}
}
func TestDefaultPathUserConfigDir(t *testing.T) {
	os.Unsetenv("LLMM_CONFIG")
	old := userConfigDir
	userConfigDir = func() (string, error) { return "/tmp/conf", nil }
	defer func() { userConfigDir = old }()
	if got := DefaultPath(); got != filepath.Join("/tmp/conf", "llmm", "config.yaml") {
		t.Fatalf("DefaultPath = %q", got)
	}
}
func TestValidateWrongVersion(t *testing.T) {
	cfg := validConfig()
	cfg.Version = 2
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "version must be") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidateNoRuntimes(t *testing.T) {
	cfg := &Config{Version: Version}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "at least one runtime") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidateSystemdRequiresService(t *testing.T) {
	cfg := validConfig()
	cfg.Runtimes["example"] = Runtime{Type: "systemd"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "requires service") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidateDockerRequiresContainer(t *testing.T) {
	cfg := validConfig()
	cfg.Runtimes["web"] = Runtime{Type: "docker"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "requires container") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidateDockerContainerDash(t *testing.T) {
	cfg := validConfig()
	cfg.Runtimes["web"] = Runtime{Type: "docker", Container: "--bad"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "must not start") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidateUnsupportedType(t *testing.T) {
	cfg := validConfig()
	cfg.Runtimes["p"] = Runtime{Type: "process"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "unsupported type") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidateModelRequiresPath(t *testing.T) {
	cfg := validConfig()
	cfg.Models["flash"] = Model{Runtime: "example"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "requires path") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestLoadReadError(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil || !strings.Contains(err.Error(), "read config") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestLoadValidationError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "version: 99\nruntimes:\n  example:\n    type: systemd\n    service: example.service\nmodels: {}\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "version must be") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestWriteMarshalError(t *testing.T) {
	old := marshalYAML
	marshalYAML = func(any) ([]byte, error) { return nil, errors.New("boom") }
	defer func() { marshalYAML = old }()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, validConfig(), false); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestLoadTrailingMalformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "version: 1\nruntimes:\n  example:\n    type: systemd\n    service: example.service\n---\n[unclosed,"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "parse trailing document") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestWriteRejectsInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, &Config{Version: 2}, false); err == nil {
		t.Fatal("expected invalid config error")
	}
}
func TestWriteMkdirAllError(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "parent")
	if err := os.WriteFile(parent, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "config.yaml")
	if err := Write(path, validConfig(), false); err == nil {
		t.Fatal("expected mkdir error")
	}
}
func TestWriteLstatError(t *testing.T) {
	old := lstat
	lstat = func(string) (os.FileInfo, error) { return nil, errors.New("boom") }
	defer func() { lstat = old }()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, validConfig(), false); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestWriteCreateTempError(t *testing.T) {
	old := createTemp
	createTemp = func(dir, pattern string) (*os.File, error) { return nil, errors.New("boom") }
	defer func() { createTemp = old }()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, validConfig(), false); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestWriteChmodError(t *testing.T) {
	old := chmod
	chmod = func(*os.File, os.FileMode) error { return errors.New("boom") }
	defer func() { chmod = old }()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, validConfig(), false); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestWriteDataError(t *testing.T) {
	old := writeData
	writeData = func(*os.File, []byte) (int, error) { return 0, errors.New("boom") }
	defer func() { writeData = old }()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, validConfig(), false); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestWriteSyncError(t *testing.T) {
	old := syncFile
	syncFile = func(*os.File) error { return errors.New("boom") }
	defer func() { syncFile = old }()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, validConfig(), false); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestWriteCloseError(t *testing.T) {
	old := closeFile
	closeFile = func(*os.File) error { return errors.New("boom") }
	defer func() { closeFile = old }()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, validConfig(), false); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestWriteLinkAlreadyExists(t *testing.T) {
	old := linkFile
	linkFile = func(a, b string) error { return os.ErrExist }
	defer func() { linkFile = old }()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, validConfig(), false); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestWriteLinkError(t *testing.T) {
	old := linkFile
	linkFile = func(a, b string) error { return errors.New("boom") }
	defer func() { linkFile = old }()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, validConfig(), false); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestForceWriteLstatError(t *testing.T) {
	old := lstat
	lstat = func(string) (os.FileInfo, error) { return nil, errors.New("boom") }
	defer func() { lstat = old }()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, validConfig(), true); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestForceWriteSecondLstatError(t *testing.T) {
	calls := 0
	old := lstat
	lstat = func(string) (os.FileInfo, error) {
		calls++
		if calls == 1 {
			return nil, os.ErrNotExist
		}
		return nil, errors.New("boom")
	}
	defer func() { lstat = old }()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, validConfig(), true); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type symlinkInfo struct{}

func (symlinkInfo) Name() string       { return "" }
func (symlinkInfo) Size() int64        { return 0 }
func (symlinkInfo) Mode() os.FileMode  { return os.ModeSymlink }
func (symlinkInfo) ModTime() time.Time { return time.Time{} }
func (symlinkInfo) IsDir() bool        { return false }
func (symlinkInfo) Sys() any           { return nil }
func TestForceWriteSecondSymlink(t *testing.T) {
	calls := 0
	old := lstat
	lstat = func(string) (os.FileInfo, error) {
		calls++
		if calls == 1 {
			return nil, os.ErrNotExist
		}
		return symlinkInfo{}, nil
	}
	defer func() { lstat = old }()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Write(path, validConfig(), true); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("unexpected error: %v", err)
	}
}
