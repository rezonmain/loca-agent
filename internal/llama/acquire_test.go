package llama

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rezonmain/loca-agent/internal/model"
)

// makeZip builds an in-memory zip from name->content entries.
func makeZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range entries {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestUnzipExtractsFiles(t *testing.T) {
	data := makeZip(t, map[string]string{
		"dir/":         "",
		"dir/file.txt": "hello",
	})
	src := filepath.Join(t.TempDir(), "a.zip")
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()

	files, err := Unzip(src, dest)
	if err != nil {
		t.Fatalf("Unzip: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 extracted file, got %v", files)
	}
	got, err := os.ReadFile(filepath.Join(dest, "dir", "file.txt"))
	if err != nil || string(got) != "hello" {
		t.Errorf("content = %q err = %v", got, err)
	}
}

func TestUnzipRejectsZipSlip(t *testing.T) {
	data := makeZip(t, map[string]string{"../escape.txt": "evil"})
	src := filepath.Join(t.TempDir(), "evil.zip")
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Unzip(src, t.TempDir()); err == nil {
		t.Errorf("expected zip slip to be rejected")
	}
}

func TestFindServerBinaryPrefersLlamaServer(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "llama-b4585")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	// Both a legacy "server.exe" and the modern "llama-server.exe" are present.
	for _, name := range []string{"server.exe", "llama-server.exe"} {
		if err := os.WriteFile(filepath.Join(nested, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := FindServerBinary(dir)
	if err != nil {
		t.Fatalf("FindServerBinary: %v", err)
	}
	if filepath.Base(got) != "llama-server.exe" {
		t.Errorf("preferred binary = %q, want llama-server.exe", filepath.Base(got))
	}
}

func TestFindServerBinaryMissing(t *testing.T) {
	if _, err := FindServerBinary(t.TempDir()); err == nil {
		t.Errorf("expected error when no server binary present")
	}
}

// fakeSource writes a fixed zip payload to the destination path.
type fakeSource struct{ zip []byte }

func (f fakeSource) Fetch(_ context.Context, _, dst string, _ func(model.Progress)) error {
	return os.WriteFile(dst, f.zip, 0o644)
}

func TestAcquireEndToEnd(t *testing.T) {
	data := makeZip(t, map[string]string{
		"llama-b4585-bin-win-vulkan-x64/llama-server.exe": "binary",
		"llama-b4585-bin-win-vulkan-x64/ggml.dll":         "lib",
	})
	a := NewAcquirer(fakeSource{zip: data}, nil)

	dest := t.TempDir()
	bin, err := a.Acquire(context.Background(), testVersions(), dest)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if filepath.Base(bin) != "llama-server.exe" {
		t.Errorf("returned binary = %q", bin)
	}
	if !strings.HasPrefix(bin, dest) {
		t.Errorf("binary path %q not under dest %q", bin, dest)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Errorf("expected binary to exist on disk: %v", err)
	}
}
