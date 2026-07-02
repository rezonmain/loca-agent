//go:build !windows

package sys

import "os"

// IsElevated reports whether the process is running as root (effective UID 0).
func IsElevated() bool {
	return os.Geteuid() == 0
}
