package probe

import (
	"os/exec"

	"github.com/rezonmain/loca-agent/internal/sys"
)

// OSLocator resolves executables using the process PATH via exec.LookPath.
type OSLocator struct{}

// LookPath implements Locator.
func (OSLocator) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// New returns the default probers backed by the real PATH and command runner.
func New(run sys.Runner) []Prober {
	return Default(run, OSLocator{})
}
