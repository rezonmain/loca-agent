package model

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// serveContent serves a fixed payload with full Range support via
// http.ServeContent (which handles 200, 206, and 416 for us).
func serveContent(payload []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "model.gguf", time.Unix(0, 0), bytes.NewReader(payload))
	}))
}

func TestFetchFull(t *testing.T) {
	payload := []byte(strings.Repeat("abcdef", 1000))
	srv := serveContent(payload)
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "sub", "model.gguf") // sub dir must be created
	src := NewHTTPSource(srv.Client())
	if err := src.Fetch(context.Background(), srv.URL, dst, nil); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("downloaded content mismatch: got %d bytes, want %d", len(got), len(payload))
	}
}

func TestFetchResume(t *testing.T) {
	payload := []byte(strings.Repeat("0123456789", 500))
	srv := serveContent(payload)
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "model.gguf")
	// Pre-seed a partial download (the first 1234 bytes).
	prefix := 1234
	if err := os.WriteFile(dst, payload[:prefix], 0o644); err != nil {
		t.Fatalf("seed partial: %v", err)
	}

	var lastTotal int64
	src := NewHTTPSource(srv.Client())
	err := src.Fetch(context.Background(), srv.URL, dst, func(p Progress) { lastTotal = p.Total })
	if err != nil {
		t.Fatalf("Fetch resume: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("resumed content mismatch: got %d bytes, want %d", len(got), len(payload))
	}
	if lastTotal != int64(len(payload)) {
		t.Errorf("progress total = %d, want %d (resume should report full size)", lastTotal, len(payload))
	}
}

func TestFetchAlreadyComplete(t *testing.T) {
	payload := []byte("complete")
	srv := serveContent(payload)
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(dst, payload, 0o644); err != nil {
		t.Fatalf("seed complete: %v", err)
	}

	// A Range at EOF yields 416; Fetch should treat it as done, not an error.
	src := NewHTTPSource(srv.Client())
	if err := src.Fetch(context.Background(), srv.URL, dst, nil); err != nil {
		t.Fatalf("Fetch complete: %v", err)
	}
	got, _ := os.ReadFile(dst)
	if !bytes.Equal(got, payload) {
		t.Errorf("content changed for already-complete file")
	}
}
