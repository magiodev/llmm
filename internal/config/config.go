package config

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const Version = 1

// Package-level function variables let tests inject failing filesystem and
// marshaling paths without changing behavior in production.
var (
	userConfigDir = os.UserConfigDir
	marshalYAML   = yaml.Marshal
	mkdirAll      = os.MkdirAll
	lstat         = os.Lstat
	createTemp    = os.CreateTemp
	chmod         = func(f *os.File, mode os.FileMode) error { return f.Chmod(mode) }
	writeData     = func(f *os.File, data []byte) (int, error) { return f.Write(data) }
	syncFile      = func(f *os.File) error { return f.Sync() }
	closeFile     = func(f *os.File) error { return f.Close() }
	linkFile      = os.Link
	renameFile    = os.Rename
)

type Config struct {
	Version  int                `yaml:"version" json:"version"`
	Node     string             `yaml:"node,omitempty" json:"node,omitempty"`
	Runtimes map[string]Runtime `yaml:"runtimes" json:"runtimes"`
	Models   map[string]Model   `yaml:"models" json:"models"`
}

type Runtime struct {
	Type       string `yaml:"type" json:"type"`
	Service    string `yaml:"service,omitempty" json:"service,omitempty"`
	Container  string `yaml:"container,omitempty" json:"container,omitempty"`
	Executable string `yaml:"executable,omitempty" json:"executable,omitempty"`
	Endpoint   string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
}

type Model struct {
	Runtime   string   `yaml:"runtime" json:"runtime"`
	Format    string   `yaml:"format" json:"format"`
	Path      string   `yaml:"path" json:"path"`
	Source    string   `yaml:"source,omitempty" json:"source,omitempty"`
	SHA256    string   `yaml:"sha256,omitempty" json:"sha256,omitempty"`
	Size      int64    `yaml:"size,omitempty" json:"size,omitempty"`
	Context   int      `yaml:"context,omitempty" json:"context,omitempty"`
	Output    int      `yaml:"output,omitempty" json:"output,omitempty"`
	Reasoning []string `yaml:"reasoning,omitempty" json:"reasoning,omitempty"`
}

func Marshal(c *Config, format string) ([]byte, error) {
	c.normalize()
	switch format {
	case "yaml":
		return marshalYAML(c)
	case "json":
		return json.MarshalIndent(c, "", "  ")
	default:
		return nil, fmt.Errorf("unsupported format %q (use yaml or json)", format)
	}
}

func DefaultPath() string {
	if value := os.Getenv("LLMM_CONFIG"); value != "" {
		return value
	}
	dir, err := userConfigDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(dir, "llmm", "config.yaml")
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("parse trailing document: %w", err)
		}
		return nil, errors.New("multiple YAML documents are not supported")
	}
	cfg.normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) normalize() {
	if c.Runtimes == nil {
		c.Runtimes = map[string]Runtime{}
	}
	if c.Models == nil {
		c.Models = map[string]Model{}
	}
}

func (c *Config) Validate() error {
	c.normalize()
	var problems []string
	if c.Version != Version {
		problems = append(problems, fmt.Sprintf("version must be %d", Version))
	}
	if len(c.Runtimes) == 0 {
		problems = append(problems, "at least one runtime is required")
	}
	for name, runtime := range c.Runtimes {
		if strings.TrimSpace(name) == "" {
			problems = append(problems, "runtime name must not be empty")
		}
		switch runtime.Type {
		case "systemd":
			if runtime.Service == "" {
				problems = append(problems, fmt.Sprintf("runtime %q requires service", name))
			} else if strings.HasPrefix(runtime.Service, "-") {
				problems = append(problems, fmt.Sprintf("runtime %q service must not start with '-'", name))
			}
			if runtime.Container != "" {
				problems = append(problems, fmt.Sprintf("runtime %q cannot set container for systemd", name))
			}
		case "docker":
			if runtime.Container == "" {
				problems = append(problems, fmt.Sprintf("runtime %q requires container", name))
			} else if strings.HasPrefix(runtime.Container, "-") {
				problems = append(problems, fmt.Sprintf("runtime %q container must not start with '-'", name))
			}
			if runtime.Service != "" {
				problems = append(problems, fmt.Sprintf("runtime %q cannot set service for docker", name))
			}
		default:
			problems = append(problems, fmt.Sprintf("runtime %q has unsupported type %q", name, runtime.Type))
		}
		if runtime.Endpoint != "" {
			parsed, err := url.ParseRequestURI(runtime.Endpoint)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				problems = append(problems, fmt.Sprintf("runtime %q endpoint must be an absolute URL", name))
			} else if parsed.User != nil {
				problems = append(problems, fmt.Sprintf("runtime %q endpoint must not contain credentials", name))
			}
		}
	}
	for name, model := range c.Models {
		if strings.TrimSpace(name) == "" {
			problems = append(problems, "model name must not be empty")
		}
		if _, ok := c.Runtimes[model.Runtime]; !ok {
			problems = append(problems, fmt.Sprintf("model %q references unknown runtime %q", name, model.Runtime))
		}
		if model.Path == "" {
			problems = append(problems, fmt.Sprintf("model %q requires path", name))
		}
		if model.Size < 0 {
			problems = append(problems, fmt.Sprintf("model %q size must not be negative", name))
		}
		if model.Context < 0 {
			problems = append(problems, fmt.Sprintf("model %q context must not be negative", name))
		}
		if model.Output < 0 {
			problems = append(problems, fmt.Sprintf("model %q output must not be negative", name))
		}
		if model.SHA256 != "" {
			digest, err := hex.DecodeString(model.SHA256)
			if err != nil || len(digest) != 32 {
				problems = append(problems, fmt.Sprintf("model %q sha256 must be 64 hexadecimal characters", name))
			}
		}
		for _, level := range model.Reasoning {
			if strings.TrimSpace(level) == "" {
				problems = append(problems, fmt.Sprintf("model %q has an empty reasoning level", name))
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func Write(path string, cfg *Config, force bool) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := marshalYAML(cfg)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := mkdirAll(dir, 0o700); err != nil {
		return err
	}
	if info, err := lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("config path is a symlink: %s", path)
		}
		if !force {
			return fmt.Errorf("config already exists: %s (use --force to replace)", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	temp, err := createTemp(dir, "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	closeWithError := func() error {
		if err := syncFile(temp); err != nil {
			closeFile(temp)
			return err
		}
		return closeFile(temp)
	}
	if err := chmod(temp, 0o600); err != nil {
		closeFile(temp)
		return err
	}
	if _, err := writeData(temp, data); err != nil {
		closeFile(temp)
		return err
	}
	if err := closeWithError(); err != nil {
		return err
	}

	if !force {
		if err := linkFile(tempPath, path); err != nil {
			if errors.Is(err, os.ErrExist) {
				return fmt.Errorf("config already exists: %s (use --force to replace)", path)
			}
			return err
		}
		return nil
	}
	if info, err := lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("config path is a symlink: %s", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return renameFile(tempPath, path)
}
