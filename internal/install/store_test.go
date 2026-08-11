package install

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// restoreVars snapshots package-level injection variables and restores them.
func restoreVars(t *testing.T) {
	t.Helper()
	l, mk, ct, rn := lstat, mkdirAll, createTemp, renameFile
	ch, wd, sy, cl := chmod, writeData, syncFile, closeFile
	ms := marshalState
	t.Cleanup(func() {
		lstat, mkdirAll, createTemp, renameFile = l, mk, ct, rn
		chmod, writeData, syncFile, closeFile = ch, wd, sy, cl
		marshalState = ms
	})
}

func TestPath(t *testing.T) {
	if got := Path("/a/b/config.yaml"); got != filepath.Join("/a/b", "installed.yaml") {
		t.Fatalf("Path(%q) = %q", "/a/b/config.yaml", got)
	}
}

func TestLoadMissing(t *testing.T) {
	restoreVars(t)
	st, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if st.Version != Version || len(st.Models) != 0 {
		t.Fatalf("empty state expected, got %+v", st)
	}
}

func TestLoadReadError(t *testing.T) {
	restoreVars(t)
	// A directory cannot be ReadFile'd.
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("expected read error")
	}
}

func TestLoadParseError(t *testing.T) {
	restoreVars(t)
	p := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(p, []byte("version: [unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoadTrailingError(t *testing.T) {
	restoreVars(t)
	p := filepath.Join(t.TempDir(), "trailing.yaml")
	content := "version: 1\nmodels: {}\n---\nbad: [unclosed\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("expected trailing parse error")
	}
}

func TestLoadRejectsMultipleDocuments(t *testing.T) {
	restoreVars(t)
	p := filepath.Join(t.TempDir(), "multi.yaml")
	content := "version: 1\nmodels: {}\n---\nversion: 1\nmodels: {}\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil || err.Error() != "multiple YAML documents are not supported" {
		t.Fatalf("expected multiple-documents error, got %v", err)
	}
}

func TestLoadValidationError(t *testing.T) {
	restoreVars(t)
	p := filepath.Join(t.TempDir(), "v.yaml")
	content := "version: 2\nmodels:\n  m:\n    path: /m\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadOK(t *testing.T) {
	restoreVars(t)
	p := filepath.Join(t.TempDir(), "ok.yaml")
	content := "version: 1\nmodels:\n  m:\n    source: https://example.com/m\n    path: /m\n    size: 10\n    sha256: " + repeat('a', 64) + "\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(st.Models) != 1 || st.Models["m"].Path != "/m" {
		t.Fatalf("bad state %+v", st.Models)
	}
}

func repeat(ch byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ch
	}
	return string(b)
}

func TestValidateProblems(t *testing.T) {
	restoreVars(t)
	st := &State{Version: Version, Models: map[string]ModelState{
		"":      {Path: "/p"},
		"empty": {Path: ""},
		"neg":   {Path: "/p", Size: -1},
		"sha":   {Path: "/p", SHA256: "zz"},
		"src1":  {Path: "/p", Source: "ftp://example.com/x"},
		"src2":  {Path: "/p", Source: "http://"},
		"src3":  {Path: "/p", Source: "http://user@example.com/x"},
		"badv":  {Path: "/p"},
	}}
	st.Version = 99
	if err := st.Validate(); err == nil {
		t.Fatal("expected validation problems")
	}
}

func TestRecordAddsAndPreserves(t *testing.T) {
	restoreVars(t)
	p := filepath.Join(t.TempDir(), "installed.yaml")
	if err := Record(p, "a", ModelState{Path: "/a", InstalledAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Record a: %v", err)
	}
	if err := Record(p, "b", ModelState{Path: "/b"}); err != nil {
		t.Fatalf("Record b: %v", err)
	}
	st, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(st.Models) != 2 || st.Models["a"].Path != "/a" || st.Models["b"].Path != "/b" {
		t.Fatalf("expected both entries, got %+v", st.Models)
	}
}

func TestRecordLoadError(t *testing.T) {
	restoreVars(t)
	// Read error path: point at a directory.
	if err := Record(t.TempDir(), "a", ModelState{Path: "/a"}); err == nil {
		t.Fatal("expected load error")
	}
}

func TestWriteStateSymlink(t *testing.T) {
	restoreVars(t)
	dir := t.TempDir()
	real := filepath.Join(dir, "real.yaml")
	if err := os.WriteFile(real, []byte("version: 1\nmodels: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.yaml")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	st := &State{Version: Version, Models: map[string]ModelState{}}
	if err := writeState(link, st); err == nil || err.Error() != "install state path is a symlink: "+link {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}

func TestWriteStateMkdirError(t *testing.T) {
	restoreVars(t)
	mkdirAll = func(string, os.FileMode) error { return errors.New("mkdir") }
	p := filepath.Join(t.TempDir(), "sub", "installed.yaml")
	st := &State{Version: Version, Models: map[string]ModelState{}}
	if err := writeState(p, st); err == nil {
		t.Fatal("expected mkdir error")
	}
}

func TestWriteStateCreateTempError(t *testing.T) {
	restoreVars(t)
	createTemp = func(string, string) (*os.File, error) { return nil, errors.New("temp") }
	st := &State{Version: Version, Models: map[string]ModelState{}}
	if err := writeState(filepath.Join(t.TempDir(), "x.yaml"), st); err == nil {
		t.Fatal("expected createTemp error")
	}
}

func TestWriteStateChmodError(t *testing.T) {
	restoreVars(t)
	chmod = func(*os.File, os.FileMode) error { return errors.New("chmod") }
	st := &State{Version: Version, Models: map[string]ModelState{}}
	if err := writeState(filepath.Join(t.TempDir(), "x.yaml"), st); err == nil {
		t.Fatal("expected chmod error")
	}
}

func TestWriteStateWriteDataError(t *testing.T) {
	restoreVars(t)
	writeData = func(*os.File, []byte) (int, error) { return 0, errors.New("write") }
	st := &State{Version: Version, Models: map[string]ModelState{}}
	if err := writeState(filepath.Join(t.TempDir(), "x.yaml"), st); err == nil {
		t.Fatal("expected write error")
	}
}

func TestWriteStateSyncError(t *testing.T) {
	restoreVars(t)
	syncFile = func(*os.File) error { return errors.New("sync") }
	st := &State{Version: Version, Models: map[string]ModelState{}}
	if err := writeState(filepath.Join(t.TempDir(), "x.yaml"), st); err == nil {
		t.Fatal("expected sync error")
	}
}

func TestWriteStateCloseError(t *testing.T) {
	restoreVars(t)
	closeFile = func(*os.File) error { return errors.New("close") }
	st := &State{Version: Version, Models: map[string]ModelState{}}
	if err := writeState(filepath.Join(t.TempDir(), "x.yaml"), st); err == nil {
		t.Fatal("expected close error")
	}
}

func TestWriteStateRenameError(t *testing.T) {
	restoreVars(t)
	renameFile = func(string, string) error { return errors.New("rename") }
	st := &State{Version: Version, Models: map[string]ModelState{}}
	if err := writeState(filepath.Join(t.TempDir(), "x.yaml"), st); err == nil {
		t.Fatal("expected rename error")
	}
}

func TestWriteStateMarshalError(t *testing.T) {
	restoreVars(t)
	marshalState = func(any) ([]byte, error) { return nil, errors.New("marshal") }
	st := &State{Version: Version, Models: map[string]ModelState{}}
	if err := writeState(filepath.Join(t.TempDir(), "x.yaml"), st); err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestWriteStateValidateError(t *testing.T) {
	restoreVars(t)
	st := &State{Version: 99, Models: map[string]ModelState{}}
	if err := writeState(filepath.Join(t.TempDir(), "x.yaml"), st); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateNilModelsNormalize(t *testing.T) {
	restoreVars(t)
	st := &State{Version: Version} // Models is nil
	if err := st.Validate(); err != nil {
		t.Fatalf("Validate nil models: %v", err)
	}
	if st.Models == nil || len(st.Models) != 0 {
		t.Fatalf("expected normalized empty map, got %v", st.Models)
	}
}

func TestWriteStateLstatError(t *testing.T) {
	restoreVars(t)
	lstat = func(string) (os.FileInfo, error) { return nil, errors.New("stat") }
	st := &State{Version: Version, Models: map[string]ModelState{}}
	if err := writeState(filepath.Join(t.TempDir(), "x.yaml"), st); err == nil {
		t.Fatal("expected lstat error")
	}
}

func TestWriteStateMode0600(t *testing.T) {
	restoreVars(t)
	p := filepath.Join(t.TempDir(), "installed.yaml")
	st := &State{Version: Version, Models: map[string]ModelState{}}
	if err := writeState(p, st); err != nil {
		t.Fatalf("writeState: %v", err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 0600", info.Mode().Perm())
	}
}
