package sys

import (
	"os"
	"runtime"
	"testing"
)

func TestIsElevatedMatchesUID(t *testing.T) {
	// On non-Windows platforms elevation means effective UID 0. (On Windows the
	// check is a heuristic and environment-dependent, so we only assert it runs.)
	got := IsElevated()
	if runtime.GOOS != "windows" {
		if want := os.Geteuid() == 0; got != want {
			t.Errorf("IsElevated() = %v, want %v (euid=%d)", got, want, os.Geteuid())
		}
	}
}
