package install

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// httpClient and newRequest are package-level variables so tests can inject
// failing paths.
var (
	httpClient = http.DefaultClient
	newRequest = http.NewRequestWithContext
)

// Fetch downloads source into dest atomically and verifies the declared size
// and sha256 when present. dest is published via temp-file + fsync + rename,
// so a partial or failed download never leaves half-written state at the final
// path (atomic publish).
func Fetch(ctx context.Context, source, dest string, size int64, wantSHA string) error {
	parsed, err := url.ParseRequestURI(source)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("source must be an absolute http(s) URL without credentials")
	}
	req, err := newRequest(ctx, http.MethodGet, source, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: %s", source, resp.Status)
	}
	dir := filepath.Dir(dest)
	temp, err := createTemp(dir, "."+filepath.Base(dest)+"-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := chmod(temp, 0o600); err != nil {
		closeFile(temp)
		return err
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(temp, h), ctxReader{ctx: ctx, r: resp.Body})
	if copyErr != nil {
		closeFile(temp)
		return copyErr
	}
	if size > 0 && n != size {
		closeFile(temp)
		return fmt.Errorf("fetch %s: size mismatch, got %d want %d", source, n, size)
	}
	sum := fmt.Sprintf("%x", h.Sum(nil))
	if wantSHA != "" && sum != strings.ToLower(wantSHA) {
		closeFile(temp)
		return fmt.Errorf("fetch %s: sha256 mismatch, got %s", source, sum)
	}
	if err := syncFile(temp); err != nil {
		closeFile(temp)
		return err
	}
	if err := closeFile(temp); err != nil {
		return err
	}
	return renameFile(tempPath, dest)
}

// ctxReader aborts the copy when ctx is done.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
