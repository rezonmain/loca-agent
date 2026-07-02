package model

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"

	"github.com/rezonmain/loca-agent/internal/config"
	"github.com/rezonmain/loca-agent/internal/errs"
)

// Result records the outcome of downloading one model file.
type Result struct {
	File     config.ModelFile
	Path     string
	Verified bool // true if the SHA-256 matched; false if no checksum was set
}

// Downloader fetches every file of a model and verifies its integrity.
type Downloader struct {
	Source     Source
	Log        *slog.Logger
	OnProgress func(Progress)
}

// Download fetches all files of m into destDir, resuming partial transfers and
// verifying checksums. It returns per-file results. A missing checksum is a
// logged warning, not a failure; a checksum mismatch is a hard error.
func (d *Downloader) Download(ctx context.Context, src config.ModelSource, m config.Model, destDir string) ([]Result, error) {
	results := make([]Result, 0, len(m.Files))
	for _, f := range m.Files {
		url := FileURL(src, m, f)
		dst := filepath.Join(destDir, f.Name)
		d.debug("downloading model file", "file", f.Name, "url", url, "dst", dst)

		if err := d.Source.Fetch(ctx, url, dst, d.OnProgress); err != nil {
			return results, errs.Wrap(err, "model_download_failed",
				"Model download failed for "+f.Name,
				"Check your network connection and the repo/file names in models.yaml, then re-run install to resume.")
		}

		verified := false
		switch err := VerifySHA256(dst, f.SHA256); {
		case err == nil:
			verified = true
		case errors.Is(err, ErrNoChecksum):
			d.warn("no checksum configured; skipping verification", "file", f.Name)
		default:
			return results, errs.Wrap(err, "model_checksum_mismatch",
				"Downloaded file failed its integrity check: "+f.Name,
				"Delete the file and re-run install to download it again.")
		}

		results = append(results, Result{File: f, Path: dst, Verified: verified})
	}
	return results, nil
}

func (d *Downloader) debug(msg string, args ...any) {
	if d.Log != nil {
		d.Log.Debug(msg, args...)
	}
}

func (d *Downloader) warn(msg string, args ...any) {
	if d.Log != nil {
		d.Log.Warn(msg, args...)
	}
}
