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

func TestFetchBadSource(t *testing.T) {
	cases := []string{
		"://bad",               // parse error
		"ftp://example.com/x",  // scheme
		"http://",              // host empty
		"http://u@example.com", // credentials
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

func TestFetchDoError(t *testing.T) {
	stubClient(t, nil, errors.New("dial"))
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 0, ""); err == nil {
		t.Fatal("expected Do error")
	}
}

func TestFetchNonOK(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Body: io.NopCloser(bytes.NewReader(nil))}
	stubClient(t, resp, nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 0, ""); err == nil {
		t.Fatal("expected non-OK error")
	}
}

func TestFetchCreateTempError(t *testing.T) {
	restoreVars(t)
	createTemp = func(string, string) (*os.File, error) { return nil, errors.New("temp") }
	body := io.NopCloser(bytes.NewReader([]byte("abc")))
	stubClient(t, &http.Response{StatusCode: 200, Status: "200 OK", Body: body}, nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 0, ""); err == nil {
		t.Fatal("expected createTemp error")
	}
}

func TestFetchChmodError(t *testing.T) {
	restoreVars(t)
	chmod = func(*os.File, os.FileMode) error { return errors.New("chmod") }
	body := io.NopCloser(bytes.NewReader([]byte("abc")))
	stubClient(t, &http.Response{StatusCode: 200, Status: "200 OK", Body: body}, nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 0, ""); err == nil {
		t.Fatal("expected chmod error")
	}
}

func TestFetchCopyError(t *testing.T) {
	restoreVars(t)
	body := io.NopCloser(&errReader{})
	stubClient(t, &http.Response{StatusCode: 200, Status: "200 OK", Body: body}, nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 0, ""); err == nil {
		t.Fatal("expected copy error")
	}
}

func TestFetchCanceledContext(t *testing.T) {
	restoreVars(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	body := io.NopCloser(bytes.NewReader([]byte("abc")))
	stubClient(t, &http.Response{StatusCode: 200, Status: "200 OK", Body: body}, nil)
	if err := Fetch(ctx, "http://example.com/m", filepath.Join(t.TempDir(), "m"), 0, ""); err == nil {
		t.Fatal("expected canceled-context error")
	}
}

func TestFetchSizeMismatch(t *testing.T) {
	restoreVars(t)
	body := io.NopCloser(bytes.NewReader([]byte("abc")))
	stubClient(t, &http.Response{StatusCode: 200, Status: "200 OK", Body: body}, nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 10, ""); err == nil {
		t.Fatal("expected size mismatch")
	}
}

func TestFetchShaMismatch(t *testing.T) {
	restoreVars(t)
	body := io.NopCloser(bytes.NewReader([]byte("abc")))
	stubClient(t, &http.Response{StatusCode: 200, Status: "200 OK", Body: body}, nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 3, "ffff"); err == nil {
		t.Fatal("expected sha mismatch")
	}
}

func TestFetchSyncError(t *testing.T) {
	restoreVars(t)
	syncFile = func(*os.File) error { return errors.New("sync") }
	body := io.NopCloser(bytes.NewReader([]byte("abc")))
	stubClient(t, &http.Response{StatusCode: 200, Status: "200 OK", Body: body}, nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 3, ""); err == nil {
		t.Fatal("expected sync error")
	}
}

func TestFetchCloseError(t *testing.T) {
	restoreVars(t)
	closeFile = func(*os.File) error { return errors.New("close") }
	body := io.NopCloser(bytes.NewReader([]byte("abc")))
	stubClient(t, &http.Response{StatusCode: 200, Status: "200 OK", Body: body}, nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 3, ""); err == nil {
		t.Fatal("expected close error")
	}
}

func TestFetchRenameError(t *testing.T) {
	restoreVars(t)
	renameFile = func(string, string) error { return errors.New("rename") }
	body := io.NopCloser(bytes.NewReader([]byte("abc")))
	stubClient(t, &http.Response{StatusCode: 200, Status: "200 OK", Body: body}, nil)
	if err := Fetch(context.Background(), "http://example.com/m", filepath.Join(t.TempDir(), "m"), 3, ""); err == nil {
		t.Fatal("expected rename error")
	}
}

func TestFetchSuccess(t *testing.T) {
	restoreVars(t)
	content := []byte("hello model")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer server.Close()
	dest := filepath.Join(t.TempDir(), "model.gguf")
	sum := fmt.Sprintf("%x", sha256.Sum256(content))
	if err := Fetch(context.Background(), server.URL, dest, int64(len(content)), sum); err != nil {
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer server.Close()
	dest := filepath.Join(t.TempDir(), "model.gguf")
	if err := Fetch(context.Background(), server.URL, dest, 0, ""); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatal(err)
	}
}

func TestFetchRejectsCredentials(t *testing.T) {
	if err := Fetch(context.Background(), "http://user:pass@example.com/m", filepath.Join(t.TempDir(), "m"), 0, ""); err == nil {
		t.Fatal("expected credential rejection")
	}
}

func TestFetchHTTPSNoVerify(t *testing.T) {
	// Covers the https-scheme accepted path with a real TLS server.
	restoreVars(t)
	content := []byte("y")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer server.Close()
	httpClient = server.Client() // trusts the test server's self-signed cert
	dest := filepath.Join(t.TempDir(), "m")
	if err := Fetch(context.Background(), server.URL, dest, 0, ""); err != nil {
		t.Fatalf("Fetch https: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatal(err)
	}
}
