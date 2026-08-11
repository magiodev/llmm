package install

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Package-level variables so tests can inject failing paths.
var (
	httpClient = http.DefaultClient
	newRequest = http.NewRequestWithContext
	openFile   = os.OpenFile
	removeFile = os.Remove
	readFile   = os.Open
)

// Fetch downloads source into dest atomically and verifies the declared size
// and sha256 when present. A partial download is kept at dest+".part" so a
// retry resumes from the existing bytes via an HTTP Range request (resume).
// On success the complete file is fsynced and renamed over dest, so a failed
// download never leaves half-written state at the final path.
func Fetch(ctx context.Context, source, dest string, size int64, wantSHA string) error {
	parsed, err := url.ParseRequestURI(source)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("source must be an absolute http(s) URL without credentials")
	}
	partPath := dest + ".part"
	partialSize := int64(0)
	if info, err := lstat(partPath); err == nil {
		if info.Mode().IsRegular() {
			partialSize = info.Size()
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	req, err := newRequest(ctx, http.MethodGet, source, nil)
	if err != nil {
		return err
	}
	if partialSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", partialSize))
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 200 means the server ignored Range (or no partial existed): start fresh.
	fresh := resp.StatusCode == http.StatusOK
	if fresh {
		partialSize = 0
		if err := removeFile(partPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("fetch %s: %s", source, resp.Status)
	}
	part, err := openPart(partPath, fresh)
	if err != nil {
		return err
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(part, h), ctxReader{ctx: ctx, r: resp.Body})
	if copyErr != nil {
		closeFile(part) // keep the partial for a resume attempt
		return copyErr
	}
	total := partialSize + n
	if size > 0 && total != size {
		closeFile(part)
		return fmt.Errorf("fetch %s: size mismatch, got %d want %d", source, total, size)
	}
	if wantSHA != "" {
		var sum string
		if fresh {
			sum = fmt.Sprintf("%x", h.Sum(nil))
		} else {
			sum, err = sha256File(partPath)
			if err != nil {
				closeFile(part)
				return err
			}
		}
		if sum != strings.ToLower(wantSHA) {
			closeFile(part)
			return fmt.Errorf("fetch %s: sha256 mismatch, got %s", source, sum)
		}
	}
	if err := syncFile(part); err != nil {
		closeFile(part)
		return err
	}
	if err := closeFile(part); err != nil {
		return err
	}
	return renameFile(partPath, dest)
}

// openPart opens the partial file: truncate for a fresh download, append for a
// resumed download.
func openPart(partPath string, fresh bool) (*os.File, error) {
	if fresh {
		return openFile(partPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	}
	return openFile(partPath, os.O_WRONLY|os.O_APPEND, 0o600)
}

// sha256File hashes a whole file (used to re-verify a resumed download).
func sha256File(path string) (string, error) {
	f, err := readFile(path)
	if err != nil {
		return "", err
	}
	defer closeFile(f)
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
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
