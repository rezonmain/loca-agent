package llama

import (
	"context"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/rezonmain/loca-agent/internal/config"
	"github.com/rezonmain/loca-agent/internal/errs"
	"github.com/rezonmain/loca-agent/internal/model"
)

// Acquirer downloads and unpacks the llama.cpp release. It reuses a model.Source
// so it inherits resumable downloads instead of duplicating that logic.
type Acquirer struct {
	Source model.Source
	Log    *slog.Logger
}

// NewAcquirer constructs an Acquirer.
func NewAcquirer(src model.Source, log *slog.Logger) *Acquirer {
	return &Acquirer{Source: src, Log: log}
}

// Acquire downloads the release asset into destDir, extracts it, and returns the
// path to the llama-server binary.
func (a *Acquirer) Acquire(ctx context.Context, v config.Versions, destDir string) (string, error) {
	url := AssetURL(v)
	if url == "" {
		return "", errs.New("llama_asset_unresolved",
			"Could not resolve the llama.cpp download URL",
			"Set llama_cpp.download_base, release_tag, and windows_asset in versions.yaml.")
	}

	zipPath := filepath.Join(destDir, v.WindowsAssetName())
	if a.Log != nil {
		a.Log.Debug("acquiring llama.cpp", "url", url, "dst", zipPath)
	}
	if err := a.Source.Fetch(ctx, url, zipPath, nil); err != nil {
		return "", errs.Wrap(err, "llama_download_failed",
			"Failed to download the llama.cpp release",
			"Check your network connection and the release_tag/windows_asset in versions.yaml.")
	}

	extractDir := filepath.Join(destDir, "llama.cpp")
	if _, err := Unzip(zipPath, extractDir); err != nil {
		return "", errs.Wrap(err, "llama_extract_failed",
			"Failed to extract the llama.cpp release archive",
			"Delete the downloaded archive and re-run install to fetch it again.")
	}

	bin, err := FindServerBinary(extractDir)
	if err != nil {
		return "", err
	}
	return bin, nil
}

// serverBinaryRank ranks candidate binary names, lowest preferred.
var serverBinaryRank = map[string]int{
	"llama-server.exe": 0,
	"llama-server":     1,
	"server.exe":       2,
	"server":           3,
}

// FindServerBinary locates the llama-server executable under dir, preferring the
// modern "llama-server" name over the legacy "server" name.
func FindServerBinary(dir string) (string, error) {
	best := ""
	bestRank := len(serverBinaryRank) + 1

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if rank, ok := serverBinaryRank[strings.ToLower(d.Name())]; ok && rank < bestRank {
			best, bestRank = path, rank
		}
		return nil
	})
	if err != nil {
		return "", errs.Wrap(err, "llama_scan_failed",
			"Failed to scan the extracted llama.cpp directory", "")
	}
	if best == "" {
		return "", errs.New("llama_binary_not_found",
			"Could not find the llama-server binary in the release archive",
			"The release asset may have an unexpected layout; check windows_asset in versions.yaml.")
	}
	return best, nil
}
