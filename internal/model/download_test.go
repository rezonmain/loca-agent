package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/rezonmain/loca-agent/internal/config"
)

// fakeSource writes fixed bytes for each requested URL, simulating a download.
type fakeSource struct {
	content map[string][]byte
}

func (f fakeSource) Fetch(_ context.Context, url, dst string, progress func(Progress)) error {
	b := f.content[url]
	if progress != nil {
		progress(Progress{File: filepath.Base(dst), Downloaded: int64(len(b)), Total: int64(len(b))})
	}
	return os.WriteFile(dst, b, 0o644)
}

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestDownloadVerifiesChecksum(t *testing.T) {
	content := []byte("model-weights")
	src := config.ModelSource{BaseURL: "https://h", FileURLTemplate: "{base}/{repo}/{file}"}
	m := config.Model{
		ID: "m", Repo: "org/m",
		Files: []config.ModelFile{{Name: "m.gguf", SHA256: sha256hex(content)}},
	}
	url := FileURL(src, m, m.Files[0])

	d := &Downloader{Source: fakeSource{content: map[string][]byte{url: content}}}
	res, err := d.Download(context.Background(), src, m, t.TempDir())
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if len(res) != 1 || !res[0].Verified {
		t.Errorf("expected 1 verified result, got %+v", res)
	}
}

func TestDownloadChecksumMismatch(t *testing.T) {
	content := []byte("model-weights")
	src := config.ModelSource{BaseURL: "https://h", FileURLTemplate: "{base}/{repo}/{file}"}
	m := config.Model{
		ID: "m", Repo: "org/m",
		Files: []config.ModelFile{{Name: "m.gguf", SHA256: sha256hex([]byte("something else"))}},
	}
	url := FileURL(src, m, m.Files[0])

	d := &Downloader{Source: fakeSource{content: map[string][]byte{url: content}}}
	_, err := d.Download(context.Background(), src, m, t.TempDir())
	if err == nil {
		t.Fatalf("expected checksum mismatch error")
	}
}

func TestDownloadMissingChecksumWarnsNotFails(t *testing.T) {
	content := []byte("model-weights")
	src := config.ModelSource{BaseURL: "https://h", FileURLTemplate: "{base}/{repo}/{file}"}
	m := config.Model{
		ID: "m", Repo: "org/m",
		Files: []config.ModelFile{{Name: "m.gguf", SHA256: ""}},
	}
	url := FileURL(src, m, m.Files[0])

	d := &Downloader{Source: fakeSource{content: map[string][]byte{url: content}}}
	res, err := d.Download(context.Background(), src, m, t.TempDir())
	if err != nil {
		t.Fatalf("missing checksum should not fail: %v", err)
	}
	if res[0].Verified {
		t.Errorf("expected Verified=false when no checksum configured")
	}
}
