package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// FileSHA256 computes the hex-encoded SHA-256 of a file.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifySHA256 checks a file against an expected hex digest. An empty expected
// value returns ErrNoChecksum so callers can warn rather than fail.
func VerifySHA256(path, expected string) error {
	if strings.TrimSpace(expected) == "" {
		return ErrNoChecksum
	}
	got, err := FileSHA256(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, strings.TrimSpace(expected)) {
		return fmt.Errorf("checksum mismatch for %s: got %s, expected %s", path, got, expected)
	}
	return nil
}

// ErrNoChecksum indicates no expected checksum was configured for a file.
var ErrNoChecksum = fmt.Errorf("no checksum configured")
