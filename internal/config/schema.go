// Package config defines the YAML configuration schema and loads it, merging
// the defaults embedded in the binary with optional user overrides.
//
// The four configuration domains — versions, models, network, installer — map
// one-to-one to the files under configs/. No version, URL, address, or path is
// hardcoded in Go; everything the operator might change lives here.
package config

import (
	"fmt"
	"strings"
)

// Config is the fully merged configuration.
type Config struct {
	Versions  Versions
	Models    ModelRegistry
	Network   Network
	Installer Installer
}

// Versions pins external dependency releases and download artifacts.
type Versions struct {
	LlamaCpp struct {
		ReleaseTag   string `yaml:"release_tag"`
		WindowsAsset string `yaml:"windows_asset"`
		DownloadBase string `yaml:"download_base"`
	} `yaml:"llama_cpp"`
	WireGuard struct {
		WindowsInstallerURL string `yaml:"windows_installer_url"`
	} `yaml:"wireguard"`
	OpenCode struct {
		InstallVia string `yaml:"install_via"`
		Package    string `yaml:"package"`
	} `yaml:"opencode"`
}

// WindowsAssetName resolves {tag} placeholders in the llama.cpp asset name.
func (v Versions) WindowsAssetName() string {
	return strings.ReplaceAll(v.LlamaCpp.WindowsAsset, "{tag}", v.LlamaCpp.ReleaseTag)
}

// ModelRegistry is the pluggable catalog of installable models.
type ModelRegistry struct {
	Default string      `yaml:"default"`
	Source  ModelSource `yaml:"source"`
	Models  []Model     `yaml:"models"`
}

// ModelSource describes where model files are downloaded from. FileURLTemplate
// is expanded with the {base}, {repo}, and {file} placeholders.
type ModelSource struct {
	BaseURL         string `yaml:"base_url"`
	FileURLTemplate string `yaml:"file_url_template"`
}

// Model describes a single GGUF model and its download files.
type Model struct {
	ID                string      `yaml:"id"`
	Name              string      `yaml:"name"`
	Repo              string      `yaml:"repo"`
	Quant             string      `yaml:"quant"`
	Files             []ModelFile `yaml:"files"`
	ContextLength     int         `yaml:"context_length"`
	MinVRAMGB         int         `yaml:"min_vram_gb"`
	RecommendedVRAMGB int         `yaml:"recommended_vram_gb"`
	Notes             string      `yaml:"notes"`
}

// ModelFile is one downloadable artifact with an optional integrity hash.
type ModelFile struct {
	Name   string `yaml:"name"`
	SHA256 string `yaml:"sha256"`
}

// Find returns the model with the given id.
func (r ModelRegistry) Find(id string) (Model, bool) {
	for _, m := range r.Models {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}

// DefaultModel returns the model referenced by the registry default.
func (r ModelRegistry) DefaultModel() (Model, bool) {
	return r.Find(r.Default)
}

// Network holds WireGuard addressing and the inference endpoint.
type Network struct {
	WireGuard WireGuardNet `yaml:"wireguard"`
	Inference InferenceNet `yaml:"inference"`
}

// WireGuardNet describes the tunnel subnet and peer addresses.
type WireGuardNet struct {
	Subnet           string   `yaml:"subnet"`
	ServerAddress    string   `yaml:"server_address"`
	ClientAddresses  []string `yaml:"client_addresses"`
	ListenPort       int      `yaml:"listen_port"`
	KeepaliveSeconds int      `yaml:"keepalive_seconds"`
}

// InferenceNet describes the OpenAI-compatible endpoint.
type InferenceNet struct {
	ListenAddress string `yaml:"listen_address"`
	Port          int    `yaml:"port"`
	Path          string `yaml:"path"`
	Scheme        string `yaml:"scheme"`
}

// EndpointURL returns the base URL OpenCode targets, e.g.
// "http://10.50.0.1:8080/v1". It uses the server's tunnel address so the URL
// is correct from the client's perspective.
func (n Network) EndpointURL() string {
	return fmt.Sprintf("%s://%s:%d%s",
		n.Inference.Scheme,
		n.WireGuard.ServerAddress,
		n.Inference.Port,
		n.Inference.Path,
	)
}

// Installer holds installation paths, service identity, and backend selection.
type Installer struct {
	Paths             Paths   `yaml:"paths"`
	Service           Service `yaml:"service"`
	Backend           string  `yaml:"backend"`
	ServiceSupervisor string  `yaml:"service_supervisor"`
}

// Paths are filesystem locations; empty values are resolved at runtime.
type Paths struct {
	InstallDir string `yaml:"install_dir"`
	ModelDir   string `yaml:"model_dir"`
	ConfigDir  string `yaml:"config_dir"`
	LogDir     string `yaml:"log_dir"`
}

// Service identifies the OS service running the inference server.
type Service struct {
	Name        string `yaml:"name"`
	DisplayName string `yaml:"display_name"`
}
