package llama

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Unzip extracts a zip archive into dest, returning the paths of extracted
// files. It guards against Zip Slip (entries that escape dest via "..").
func Unzip(src, dest string) ([]string, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return nil, fmt.Errorf("open zip %s: %w", src, err)
	}
	defer r.Close()

	cleanDest := filepath.Clean(dest)
	var extracted []string
	for _, f := range r.File {
		target := filepath.Join(cleanDest, f.Name)
		if target != cleanDest && !strings.HasPrefix(target, cleanDest+string(os.PathSeparator)) {
			return nil, fmt.Errorf("illegal path in zip (zip slip): %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return nil, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if err := extractOne(f, target); err != nil {
			return nil, err
		}
		extracted = append(extracted, target)
	}
	return extracted, nil
}

func extractOne(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	perm := f.Mode().Perm()
	if perm == 0 {
		perm = 0o644 // zips created on Windows often carry no unix mode
	}
	w, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer w.Close()

	if _, err := io.Copy(w, rc); err != nil {
		return fmt.Errorf("extract %s: %w", f.Name, err)
	}
	return nil
}
