// Package llama acquires the llama.cpp server binary for the configured release
// and builds its launch arguments. It holds no versions or URLs of its own —
// the release comes from config.Versions and the endpoint from config.Network —
// so upgrading llama.cpp is a config change (see configs/versions.yaml).
package llama

import (
	"strconv"
	"strings"

	"github.com/rezonmain/loca-agent/internal/config"
)

// AllGPULayers offloads every model layer to the GPU (llama.cpp -ngl).
const AllGPULayers = 999

// AssetURL resolves the download URL for the platform release asset, expanding
// the {tag} placeholder. It returns "" when any required version field is unset.
func AssetURL(v config.Versions) string {
	base := strings.TrimRight(v.LlamaCpp.DownloadBase, "/")
	if base == "" || v.LlamaCpp.ReleaseTag == "" || v.LlamaCpp.WindowsAsset == "" {
		return ""
	}
	return base + "/" + v.LlamaCpp.ReleaseTag + "/" + v.WindowsAssetName()
}

// ServerOptions are the inputs to the llama-server launch arguments.
type ServerOptions struct {
	ModelPath   string
	Host        string // bind address — the tunnel IP, never 0.0.0.0
	Port        int
	ContextSize int
	GPULayers   int    // -ngl; use AllGPULayers to offload everything
	Alias       string // --alias, the model name reported to clients
}

// OptionsFromConfig derives ServerOptions from the network config and a resolved
// model, binding to the tunnel address and offloading all layers by default.
func OptionsFromConfig(net config.Network, m config.Model, modelPath string) ServerOptions {
	return ServerOptions{
		ModelPath:   modelPath,
		Host:        net.Inference.ListenAddress,
		Port:        net.Inference.Port,
		ContextSize: m.ContextLength,
		GPULayers:   AllGPULayers,
		Alias:       m.ID,
	}
}

// Args builds the llama-server command-line arguments from options. Zero-valued
// optional fields are omitted; -ngl is always emitted (0 means CPU-only).
func Args(opt ServerOptions) []string {
	args := []string{"-m", opt.ModelPath}
	if opt.Host != "" {
		args = append(args, "--host", opt.Host)
	}
	if opt.Port > 0 {
		args = append(args, "--port", strconv.Itoa(opt.Port))
	}
	if opt.ContextSize > 0 {
		args = append(args, "-c", strconv.Itoa(opt.ContextSize))
	}
	args = append(args, "-ngl", strconv.Itoa(opt.GPULayers))
	if opt.Alias != "" {
		args = append(args, "--alias", opt.Alias)
	}
	return args
}
