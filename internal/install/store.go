// Package install manages node-local model artifact install state. The
// manifest declares what a node should serve; install records what is actually
// fetched and placed on disk. State is machine-managed and kept separate from
// the human-edited manifest (see docs/contract.md).
package install

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const Version = 1

// Package-level function variables let tests inject failing filesystem paths
// without changing behavior in production. The pattern mirrors config.Write.
var (
	lstat        = os.Lstat
	mkdirAll     = os.MkdirAll
	createTemp   = os.CreateTemp
	chmod        = func(f *os.File, mode os.FileMode) error { return f.Chmod(mode) }
	writeData    = func(f *os.File, data []byte) (int, error) { return f.Write(data) }
	syncFile     = func(f *os.File) error { return f.Sync() }
	closeFile    = func(f *os.File) error { return f.Close() }
	renameFile   = os.Rename
	marshalState = yaml.Marshal
)

// State is the machine-managed install record. It is additive and versioned,
// and old binaries ignore unknown fields.
type State struct {
	Version int                   `yaml:"version" json:"version"`
	Models  map[string]ModelState `yaml:"models" json:"models"`
}

// ModelState records the installed origin and integrity of one model.
type ModelState struct {
	Source      string    `yaml:"source,omitempty" json:"source,omitempty"`
	Path        string    `yaml:"path" json:"path"`
	Size        int64     `yaml:"size,omitempty" json:"size,omitempty"`
	SHA256      string    `yaml:"sha256,omitempty" json:"sha256,omitempty"`
	InstalledAt time.Time `yaml:"installed_at,omitempty" json:"installed_at,omitempty"`
}

// Path returns the install-state file that accompanies a config file. It
// lives in the config's directory so machine-managed state stays next to the
// human manifest but is a separate file.
func Path(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "installed.yaml")
}

// Load reads install state. A missing file is not an error: it means nothing
// is installed yet and the caller gets an empty state.
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{Version: Version, Models: map[string]ModelState{}}, nil
		}
		return nil, fmt.Errorf("read install state: %w", err)
	}
	var st State
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&st); err != nil {
		return nil, fmt.Errorf("parse install state: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("parse trailing install-state document: %w", err)
		}
		return nil, errors.New("multiple YAML documents are not supported")
	}
	st.normalize()
	if err := st.Validate(); err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *State) normalize() {
	if s.Models == nil {
		s.Models = map[string]ModelState{}
	}
}

func (s *State) Validate() error {
	s.normalize()
	var problems []string
	if s.Version != Version {
		problems = append(problems, fmt.Sprintf("version must be %d", Version))
	}
	for name, ms := range s.Models {
		if strings.TrimSpace(name) == "" {
			problems = append(problems, "model name must not be empty")
		}
		if ms.Path == "" {
			problems = append(problems, fmt.Sprintf("model %q requires path", name))
		}
		if ms.Size < 0 {
			problems = append(problems, fmt.Sprintf("model %q size must not be negative", name))
		}
		if ms.SHA256 != "" {
			digest, err := hex.DecodeString(ms.SHA256)
			if err != nil || len(digest) != 32 {
				problems = append(problems, fmt.Sprintf("model %q sha256 must be 64 hexadecimal characters", name))
			}
		}
		if ms.Source != "" {
			parsed, err := url.ParseRequestURI(ms.Source)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
				problems = append(problems, fmt.Sprintf("model %q source must be an absolute http(s) URL without credentials", name))
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// Record stores the installed state for one model, preserving other entries.
func Record(path string, name string, ms ModelState) error {
	st, err := Load(path)
	if err != nil {
		return err
	}
	st.Models[name] = ms
	return writeState(path, st)
}

// writeState writes state atomically with the same guarantees as config.Write:
// 0600 permissions, symlink rejection, temp-file + fsync + rename publish.
func writeState(path string, st *State) error {
	if err := st.Validate(); err != nil {
		return err
	}
	data, err := marshalState(st)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := mkdirAll(dir, 0o700); err != nil {
		return err
	}
	if info, err := lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("install state path is a symlink: %s", path)
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
	if err := chmod(temp, 0o600); err != nil {
		closeFile(temp)
		return err
	}
	if _, err := writeData(temp, data); err != nil {
		closeFile(temp)
		return err
	}
	if err := syncFile(temp); err != nil {
		closeFile(temp)
		return err
	}
	if err := closeFile(temp); err != nil {
		return err
	}
	return renameFile(tempPath, path)
}
