package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	assets "github.com/rezonmain/loca-agent"
	"github.com/rezonmain/loca-agent/internal/errs"
	"gopkg.in/yaml.v3"
)

// domain maps a config section to its file name (shared by embedded defaults
// and user overrides).
type domain struct {
	file string
	dst  any
}

// Load builds a Config by unmarshaling the embedded defaults and then applying
// any override files found in userDir on top. Passing an empty userDir loads
// defaults only. After merging, runtime path defaults are resolved and the
// result is validated.
func Load(userDir string) (*Config, error) {
	c := &Config{}
	domains := []domain{
		{"versions.yaml", &c.Versions},
		{"models.yaml", &c.Models},
		{"network.yaml", &c.Network},
		{"installer.yaml", &c.Installer},
	}

	for _, d := range domains {
		if err := unmarshalFS(assets.Defaults, filepath.ToSlash(filepath.Join("configs", d.file)), d.dst); err != nil {
			return nil, errs.Wrap(err, "config_defaults",
				fmt.Sprintf("failed to read built-in defaults for %s", d.file),
				"This indicates a corrupt build. Reinstall bootstrap-ai.")
		}
	}

	if userDir != "" {
		for _, d := range domains {
			path := filepath.Join(userDir, d.file)
			if _, err := os.Stat(path); err != nil {
				continue // no override for this domain
			}
			if err := unmarshalFS(os.DirFS(userDir), d.file, d.dst); err != nil {
				return nil, errs.Wrap(err, "config_override",
					fmt.Sprintf("failed to parse override %s", path),
					"Check the file for YAML syntax errors.")
			}
		}
	}

	c.resolveDefaults()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func unmarshalFS(fsys fs.FS, name string, dst any) error {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, dst)
}

// resolveDefaults fills empty path fields with per-OS conventions.
func (c *Config) resolveDefaults() {
	p := &c.Installer.Paths
	if p.InstallDir == "" {
		p.InstallDir = defaultInstallDir()
	}
	if p.ModelDir == "" {
		p.ModelDir = filepath.Join(p.InstallDir, "models")
	}
	if p.ConfigDir == "" {
		p.ConfigDir = DefaultUserDir()
	}
	if p.LogDir == "" {
		p.LogDir = filepath.Join(p.InstallDir, "logs")
	}

	src := &c.Models.Source
	if src.BaseURL == "" {
		src.BaseURL = "https://huggingface.co"
	}
	if src.FileURLTemplate == "" {
		src.FileURLTemplate = "{base}/{repo}/resolve/main/{file}"
	}
}

// Validate checks that required fields are present and internally consistent.
func (c *Config) Validate() error {
	if c.Models.Default == "" {
		return errs.New("config_invalid", "models.yaml has no default model",
			"Set `default:` to one of the model ids in models.yaml.")
	}
	if _, ok := c.Models.DefaultModel(); !ok {
		return errs.New("config_invalid",
			fmt.Sprintf("default model %q is not defined in models.yaml", c.Models.Default),
			"Add a matching model entry or fix the `default:` value.")
	}
	if c.Network.WireGuard.ServerAddress == "" {
		return errs.New("config_invalid", "network.yaml is missing wireguard.server_address",
			"Set a tunnel address such as 10.50.0.1.")
	}
	if c.Network.Inference.Port == 0 {
		return errs.New("config_invalid", "network.yaml is missing inference.port",
			"Set the llama.cpp server port, e.g. 8080.")
	}
	return nil
}

// DefaultUserDir returns the per-user config directory for bootstrap-ai.
func DefaultUserDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "bootstrap-ai")
}

// defaultInstallDir returns the OS-appropriate installation root.
func defaultInstallDir() string {
	switch runtime.GOOS {
	case "windows":
		if pd := os.Getenv("ProgramData"); pd != "" {
			return filepath.Join(pd, "bootstrap-ai")
		}
		return `C:\ProgramData\bootstrap-ai`
	default:
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".bootstrap-ai")
		}
		return "/opt/bootstrap-ai"
	}
}
