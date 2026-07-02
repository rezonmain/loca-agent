package model

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Progress reports download progress for a single file.
type Progress struct {
	File       string
	Downloaded int64 // bytes on disk so far (including any resumed prefix)
	Total      int64 // total size in bytes, or 0 if unknown
}

// Source fetches a file from a URL to a local path. It is an interface so the
// downloader can be tested against fakes and, later, extended with other
// transports (e.g. torrent, S3) without changing callers.
type Source interface {
	Fetch(ctx context.Context, url, dst string, progress func(Progress)) error
}

// HTTPSource downloads over HTTP(S) with resume support via Range requests.
type HTTPSource struct {
	Client *http.Client
}

// NewHTTPSource returns an HTTPSource using the provided client (or the default
// client when nil).
func NewHTTPSource(client *http.Client) *HTTPSource {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPSource{Client: client}
}

// Fetch downloads url to dst, resuming from any bytes already present. It emits
// progress via the callback (which may be nil).
func (s *HTTPSource) Fetch(ctx context.Context, url, dst string, progress func(Progress)) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create download directory: %w", err)
	}

	var offset int64
	if fi, err := os.Stat(dst); err == nil {
		offset = fi.Size()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	f, total, err := s.openTarget(dst, offset, resp)
	if err != nil {
		return err
	}
	if f == nil {
		return nil // already complete (HTTP 416)
	}
	defer f.Close()

	name := filepath.Base(dst)
	pw := &progressWriter{
		file:    f,
		written: offset,
		total:   total,
		name:    name,
		report:  progress,
	}
	if _, err := io.Copy(pw, resp.Body); err != nil {
		return fmt.Errorf("download %s: %w", name, err)
	}
	return nil
}

// openTarget opens dst for writing based on the server's response and returns
// the file and known total size. A nil file with nil error means the content is
// already complete.
func (s *HTTPSource) openTarget(dst string, offset int64, resp *http.Response) (*os.File, int64, error) {
	switch resp.StatusCode {
	case http.StatusOK:
		// Server ignored the Range (or none was sent): restart from scratch.
		f, err := os.Create(dst)
		if err != nil {
			return nil, 0, err
		}
		return f, resp.ContentLength, nil

	case http.StatusPartialContent:
		f, err := os.OpenFile(dst, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, 0, err
		}
		total := totalFromContentRange(resp.Header.Get("Content-Range"))
		if total <= 0 {
			total = offset + resp.ContentLength
		}
		return f, total, nil

	case http.StatusRequestedRangeNotSatisfiable:
		// The file on disk is already at or beyond the full length.
		return nil, 0, nil

	default:
		return nil, 0, fmt.Errorf("unexpected status %s fetching %s", resp.Status, resp.Request.URL)
	}
}

// totalFromContentRange parses the total length from a "bytes X-Y/Z" header.
func totalFromContentRange(h string) int64 {
	i := strings.LastIndex(h, "/")
	if i < 0 {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(h[i+1:]), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// progressWriter writes to the target file while reporting cumulative progress.
type progressWriter struct {
	file    io.Writer
	written int64
	total   int64
	name    string
	report  func(Progress)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n, err := w.file.Write(p)
	w.written += int64(n)
	if w.report != nil {
		w.report(Progress{File: w.name, Downloaded: w.written, Total: w.total})
	}
	return n, err
}
