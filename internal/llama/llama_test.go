package llama

import (
	"reflect"
	"testing"

	"github.com/rezonmain/loca-agent/internal/config"
)

func testVersions() config.Versions {
	var v config.Versions
	v.LlamaCpp.DownloadBase = "https://github.com/ggml-org/llama.cpp/releases/download"
	v.LlamaCpp.ReleaseTag = "b4585"
	v.LlamaCpp.WindowsAsset = "llama-{tag}-bin-win-vulkan-x64.zip"
	return v
}

func TestAssetURL(t *testing.T) {
	got := AssetURL(testVersions())
	want := "https://github.com/ggml-org/llama.cpp/releases/download/b4585/llama-b4585-bin-win-vulkan-x64.zip"
	if got != want {
		t.Errorf("AssetURL = %q, want %q", got, want)
	}
}

func TestAssetURLIncomplete(t *testing.T) {
	var v config.Versions // all empty
	if AssetURL(v) != "" {
		t.Errorf("expected empty URL when versions are unset")
	}
}

func TestArgs(t *testing.T) {
	got := Args(ServerOptions{
		ModelPath:   "/models/m.gguf",
		Host:        "10.50.0.1",
		Port:        8080,
		ContextSize: 32768,
		GPULayers:   AllGPULayers,
		Alias:       "qwen",
	})
	want := []string{
		"-m", "/models/m.gguf",
		"--host", "10.50.0.1",
		"--port", "8080",
		"-c", "32768",
		"-ngl", "999",
		"--alias", "qwen",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Args = %v\nwant %v", got, want)
	}
}

func TestArgsOmitsUnsetOptionals(t *testing.T) {
	got := Args(ServerOptions{ModelPath: "/m.gguf", GPULayers: 0})
	want := []string{"-m", "/m.gguf", "-ngl", "0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Args = %v\nwant %v", got, want)
	}
}

func TestOptionsFromConfig(t *testing.T) {
	var net config.Network
	net.Inference.ListenAddress = "10.50.0.1"
	net.Inference.Port = 8080
	m := config.Model{ID: "qwen3-coder-30b-a3b", ContextLength: 32768}

	opt := OptionsFromConfig(net, m, "/models/x.gguf")
	if opt.Host != "10.50.0.1" || opt.Port != 8080 || opt.ContextSize != 32768 ||
		opt.Alias != "qwen3-coder-30b-a3b" || opt.GPULayers != AllGPULayers || opt.ModelPath != "/models/x.gguf" {
		t.Errorf("OptionsFromConfig = %+v", opt)
	}
}
