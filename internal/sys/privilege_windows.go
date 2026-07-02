//go:build windows

package sys

import "os"

// IsElevated reports whether the process is running with Administrator rights.
//
// It uses a well-known heuristic: only an elevated process can open the raw
// physical-drive device. This avoids pulling in a Win32 token-inspection
// dependency; it can be upgraded to a proper token check later if needed.
func IsElevated() bool {
	f, err := os.Open(`\\.\PHYSICALDRIVE0`)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
