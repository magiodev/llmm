package install

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type stubTransport struct {
	resp *http.Response
	err  error
}

func (s *stubTransport) RoundTrip(_ *http.Request) (*http.Response, error) { return s.resp, s.err }

func stubClient(t *testing.T, resp *http.Response, err error) {
	t.Helper()
	old := httpClient
	httpClient = &http.Client{Transport: &stubTransport{resp: resp, err: err}}
	t.Cleanup(func() { httpClient = old })
}

func stubNewRequest(t *testing.T, err error) {
	t.Helper()
	old := newRequest
	newRequest = func(context.Context, string, string, io.Reader) (*http.Request, error) { return nil, err }
	t.Cleanup(func() { newRequest = old })
}

func stubOpenFile(t *testing.T, err error) {
	t.Helper()
	old := openFile
	openFile = func(string, int, os.FileMode) (*os.File, error) { return nil, err }
	t.Cleanup(func() { openFile = old })
}

func stubRemoveFile(t *testing.T, err error) {
	t.Helper()
	old := removeFile
	removeFile = func(string) error { return err }
	t.Cleanup(func() { removeFile = old })
}

func okResponse(content []byte) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(bytes.NewReader(content))}
}

// errReader returns some bytes then an error.
type errReader struct {
	done bool
}

func (r *errReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, errors.New("boom")
	}
	r.done = true
	return copy(p, []byte("x")), nil
}

// rangeServer serves the full body when no Range header, else the suffix
// starting at the requested byte with 206.
func rangeServer(t *testing.T, content []byte) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHdr := r.Header.Get("Range")
		if rangeHdr == "" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(content)
			return
		}
		var from int
		if _, err := fmt.Sscanf(rangeHdr, "bytes=%d-", &from); err != nil {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if from >= len(content) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(content[from:])
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestFetchBadSource(t *testing.T) {
	cases := []string{
		"://bad",
		"ftp://example.com/x",
		"http://",
		"http://user:pass@example.com/x",
	}
	for _, src := range cases {
		if err := Fetch(context.Background(), src, filepath.Join(t.TempDir(), "m"), 0, ""); err == nil {
			t.Fatalf("source %q: expected error", src)
		}
	}
}

func TestFetchNewRequestError(t *testing.T) {
	stubNewRequest(t, errors.New("req"))
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 0, ""); err == nil {
		t.Fatal("expected newRequest error")
	}
}

func TestFetchLstatError(t *testing.T) {
	restoreVars(t)
	lstat = func(string) (os.FileInfo, error) { return nil, errors.New("stat") }
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 0, ""); err == nil {
		t.Fatal("expected lstat error")
	}
}

func TestFetchDoError(t *testing.T) {
	stubClient(t, nil, errors.New("dial"))
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 0, ""); err == nil {
		t.Fatal("expected Do error")
	}
}

func TestFetchNonRangeableStatus(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Body: io.NopCloser(bytes.NewReader(nil))}
	stubClient(t, resp, nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 0, ""); err == nil {
		t.Fatal("expected non-200/206 error")
	}
}

func TestFetchRemoveError(t *testing.T) {
	// A non-empty directory as the .part makes fresh-start removeFile fail
	// (not ErrNotExist) and also exercises the non-regular lstat branch.
	dir := t.TempDir()
	part := filepath.Join(dir, "m.part")
	if err := os.Mkdir(part, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(part, "x"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	stubClient(t, okResponse([]byte("abc")), nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(dir, "m"), 0, ""); err == nil {
		t.Fatal("expected remove error")
	}
}

func TestFetchOpenError(t *testing.T) {
	stubOpenFile(t, errors.New("open"))
	stubClient(t, okResponse([]byte("abc")), nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 0, ""); err == nil {
		t.Fatal("expected open error")
	}
}

func TestFetchCopyError(t *testing.T) {
	restoreVars(t)
	stubClient(t, &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(&errReader{})}, nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 0, ""); err == nil {
		t.Fatal("expected copy error")
	}
}

func TestFetchCanceledContext(t *testing.T) {
	restoreVars(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stubClient(t, okResponse([]byte("abc")), nil)
	if err := Fetch(ctx, "http://example.com/m", filepath.Join(t.TempDir(), "m"), 0, ""); err == nil {
		t.Fatal("expected canceled-context error")
	}
}

func TestFetchSizeMismatch(t *testing.T) {
	restoreVars(t)
	stubClient(t, okResponse([]byte("abc")), nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 10, ""); err == nil {
		t.Fatal("expected size mismatch")
	}
}

func TestFetchShaMismatchFresh(t *testing.T) {
	restoreVars(t)
	stubClient(t, okResponse([]byte("abc")), nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 3, "ffff"); err == nil {
		t.Fatal("expected sha mismatch")
	}
}

func TestFetchSyncError(t *testing.T) {
	restoreVars(t)
	syncFile = func(*os.File) error { return errors.New("sync") }
	stubClient(t, okResponse([]byte("abc")), nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 3, ""); err == nil {
		t.Fatal("expected sync error")
	}
}

func TestFetchCloseError(t *testing.T) {
	restoreVars(t)
	closeFile = func(*os.File) error { return errors.New("close") }
	stubClient(t, okResponse([]byte("abc")), nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 3, ""); err == nil {
		t.Fatal("expected close error")
	}
}

func TestFetchRenameError(t *testing.T) {
	restoreVars(t)
	renameFile = func(string, string) error { return errors.New("rename") }
	stubClient(t, okResponse([]byte("abc")), nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 3, ""); err == nil {
		t.Fatal("expected rename error")
	}
}

func TestFetchSuccess(t *testing.T) {
	restoreVars(t)
	content := []byte("hello model")
	url := rangeServer(t, content)
	dest := filepath.Join(t.TempDir(), "model.gguf")
	sum := fmt.Sprintf("%x", sha256.Sum256(content))
	if err := Fetch(context.Background(), url, dest, int64(len(content)), sum); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %q want %q", got, content)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestFetchSuccessNoVerify(t *testing.T) {
	restoreVars(t)
	content := []byte("plain")
	url := rangeServer(t, content)
	dest := filepath.Join(t.TempDir(), "model.gguf")
	if err := Fetch(context.Background(), url, dest, 0, ""); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatal(err)
	}
}

func TestFetchResumeSuccess(t *testing.T) {
	restoreVars(t)
	content := []byte("abcdefghij")
	url := rangeServer(t, content)
	dir := t.TempDir()
	dest := filepath.Join(dir, "model.gguf")
	// Pre-seed a partial download of the first 3 bytes.
	if err := os.WriteFile(dest+".part", []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(content))
	if err := Fetch(context.Background(), url, dest, int64(len(content)), sum); err != nil {
		t.Fatalf("Fetch resume: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("resume content mismatch: got %q want %q", got, content)
	}
	// The .part must be gone after publish.
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Fatalf(".part should be gone after publish, err=%v", err)
	}
}

func TestFetchResumeServerIgnoresRange(t *testing.T) {
	restoreVars(t)
	content := []byte("abcdefghij")
	// Server ignores Range: always 200 with full body.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(dest+".part", []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(content))
	if err := Fetch(context.Background(), srv.URL, dest, int64(len(content)), sum); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	got, _ := os.ReadFile(dest)
	if !bytes.Equal(got, content) {
		t.Fatalf("restart content mismatch: got %q want %q", got, content)
	}
}

func TestFetchResumeShaMismatch(t *testing.T) {
	restoreVars(t)
	content := []byte("abcdefghij")
	url := rangeServer(t, content)
	dest := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(dest+".part", []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Fetch(context.Background(), url, dest, int64(len(content)), "ffff"); err == nil {
		t.Fatal("expected resume sha mismatch")
	}
}

func TestFetchResumeShaFileError(t *testing.T) {
	restoreVars(t)
	content := []byte("abcdefghij")
	url := rangeServer(t, content)
	dest := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(dest+".part", []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	readFile = func(string) (*os.File, error) { return nil, errors.New("read") }
	if err := Fetch(context.Background(), url, dest, int64(len(content)), "deadbeef"); err == nil {
		t.Fatal("expected resume sha-file error")
	}
}

func TestSha256FileOK(t *testing.T) {
	restoreVars(t)
	content := []byte("hello")
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum, err := sha256File(p)
	if err != nil {
		t.Fatalf("sha256File: %v", err)
	}
	if sum != fmt.Sprintf("%x", sha256.Sum256(content)) {
		t.Fatalf("sum = %s", sum)
	}
}

func TestSha256FileReadError(t *testing.T) {
	restoreVars(t)
	if _, err := sha256File(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected read error")
	}
}

func TestSha256FileCopyError(t *testing.T) {
	restoreVars(t)
	// os.Open on a directory returns a *os.File whose Read fails.
	if _, err := sha256File(t.TempDir()); err == nil {
		t.Fatal("expected copy error")
	}
}

func TestFetchHTTPSAccepted(t *testing.T) {
	restoreVars(t)
	content := []byte("y")
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()
	httpClient = srv.Client()
	dest := filepath.Join(t.TempDir(), "m")
	if err := Fetch(context.Background(), srv.URL, dest, 0, ""); err != nil {
		t.Fatalf("Fetch https: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatal(err)
	}
}
