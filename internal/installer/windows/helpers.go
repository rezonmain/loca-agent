package windows

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/rezonmain/loca-agent/internal/wgtunnel"
)

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// wireguardExePath returns the default install location of wireguard.exe.
func wireguardExePath() string {
	pf := os.Getenv("ProgramFiles")
	if pf == "" {
		pf = `C:\Program Files`
	}
	return filepath.Join(pf, "WireGuard", "wireguard.exe")
}

// writeConfig writes a WireGuard config with owner-only permissions.
func writeConfig(path, contents string) error {
	return wgtunnel.WriteConfig(path, contents)
}

func itoa(n int) string { return strconv.Itoa(n) }
