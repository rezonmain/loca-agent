package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rezonmain/loca-agent/internal/config"
)

func sampleParams() Params {
	return Params{
		ProviderID:   "llama-local",
		ProviderName: "Local llama.cpp",
		NPMPackage:   "@ai-sdk/openai-compatible",
		BaseURL:      "http://10.50.0.1:8080/v1",
		ModelID:      "qwen3-coder-30b-a3b",
		ModelName:    "Qwen3-Coder 30B-A3B",
	}
}

// parse renders and unmarshals, failing on invalid JSON.
func parse(t *testing.T, p Params) map[string]any {
	t.Helper()
	out, err := Render(p)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	return m
}

func TestRenderWiresEndpointAndModel(t *testing.T) {
	m := parse(t, sampleParams())

	provider := m["provider"].(map[string]any)["llama-local"].(map[string]any)
	if got := provider["options"].(map[string]any)["baseURL"]; got != "http://10.50.0.1:8080/v1" {
		t.Errorf("baseURL = %v", got)
	}
	if got := provider["npm"]; got != "@ai-sdk/openai-compatible" {
		t.Errorf("npm = %v", got)
	}
	models := provider["models"].(map[string]any)
	if _, ok := models["qwen3-coder-30b-a3b"]; !ok {
		t.Errorf("expected model key present, got %v", models)
	}
}

func TestRenderEscapesSpecialCharacters(t *testing.T) {
	p := sampleParams()
	p.ModelName = `Weird "Quoted" Name` // must not break JSON
	m := parse(t, p)                    // parse asserts validity

	provider := m["provider"].(map[string]any)["llama-local"].(map[string]any)
	model := provider["models"].(map[string]any)["qwen3-coder-30b-a3b"].(map[string]any)
	if model["name"] != `Weird "Quoted" Name` {
		t.Errorf("escaped name round-trip failed: %v", model["name"])
	}
}

func TestFromConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Network.Inference.Scheme = "http"
	cfg.Network.Inference.Port = 8080
	cfg.Network.Inference.Path = "/v1"
	cfg.Network.WireGuard.ServerAddress = "10.50.0.1"
	m := config.Model{ID: "qwen3-coder-30b-a3b", Name: "Qwen3-Coder 30B-A3B"}

	p := FromConfig(cfg, m)
	if p.BaseURL != "http://10.50.0.1:8080/v1" {
		t.Errorf("BaseURL = %q", p.BaseURL)
	}
	if p.ModelID != m.ID || p.ModelName != m.Name {
		t.Errorf("model not wired: %+v", p)
	}
	if p.ProviderID != DefaultProviderID || p.NPMPackage != DefaultNPMPackage {
		t.Errorf("defaults not applied: %+v", p)
	}
}

func TestWriteCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "opencode.json")
	if err := Write(path, sampleParams()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Errorf("written file is not valid JSON: %v", err)
	}
}
