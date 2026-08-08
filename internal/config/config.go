package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const Version = 1

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
	Runtime string `yaml:"runtime" json:"runtime"`
	Format  string `yaml:"format" json:"format"`
	Path    string `yaml:"path" json:"path"`
	Source  string `yaml:"source,omitempty" json:"source,omitempty"`
	SHA256  string `yaml:"sha256,omitempty" json:"sha256,omitempty"`
	Size    int64  `yaml:"size,omitempty" json:"size,omitempty"`
	Context int    `yaml:"context,omitempty" json:"context,omitempty"`
	Output  int    `yaml:"output,omitempty" json:"output,omitempty"`
}

func Marshal(c *Config, format string) ([]byte, error) {
	switch format {
	case "yaml":
		return yaml.Marshal(c)
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
	dir, err := os.UserConfigDir()
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
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	var problems []string
	if c.Version != Version {
		problems = append(problems, fmt.Sprintf("version must be %d", Version))
	}
	if len(c.Runtimes) == 0 {
		problems = append(problems, "at least one runtime is required")
	}
	for name, runtime := range c.Runtimes {
		switch runtime.Type {
		case "systemd":
			if runtime.Service == "" {
				problems = append(problems, fmt.Sprintf("runtime %q requires service", name))
			}
		case "docker":
			if runtime.Container == "" {
				problems = append(problems, fmt.Sprintf("runtime %q requires container", name))
			}
		default:
			problems = append(problems, fmt.Sprintf("runtime %q has unsupported type %q", name, runtime.Type))
		}
	}
	for name, model := range c.Models {
		if _, ok := c.Runtimes[model.Runtime]; !ok {
			problems = append(problems, fmt.Sprintf("model %q references unknown runtime %q", name, model.Runtime))
		}
		if model.Path == "" {
			problems = append(problems, fmt.Sprintf("model %q requires path", name))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func Write(path string, cfg *Config, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config already exists: %s (use --force to replace)", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
