// Package opencode generates the OpenCode client configuration that points the
// coding agent at the WireGuard-tunneled, OpenAI-compatible llama.cpp endpoint.
//
// The endpoint and model come entirely from config; only OpenCode's provider
// plumbing (the npm adapter and a provider id) is defaulted here, and those
// remain overridable via Params.
package opencode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	assets "github.com/rezonmain/loca-agent"
	"github.com/rezonmain/loca-agent/internal/config"
)

// Defaults for OpenCode's custom OpenAI-compatible provider wiring.
const (
	DefaultProviderID   = "llama-local"
	DefaultProviderName = "Local llama.cpp (bootstrap-ai)"
	DefaultNPMPackage   = "@ai-sdk/openai-compatible"
)

// Params are the values rendered into the OpenCode config.
type Params struct {
	ProviderID   string
	ProviderName string
	NPMPackage   string
	BaseURL      string
	ModelID      string
	ModelName    string
}

// FromConfig builds Params from the resolved config and model, targeting the
// tunnel endpoint (e.g. http://10.50.0.1:8080/v1).
func FromConfig(cfg *config.Config, m config.Model) Params {
	return Params{
		ProviderID:   DefaultProviderID,
		ProviderName: DefaultProviderName,
		NPMPackage:   DefaultNPMPackage,
		BaseURL:      cfg.Network.EndpointURL(),
		ModelID:      m.ID,
		ModelName:    m.Name,
	}
}

const templatePath = "templates/opencode/opencode.json.tmpl"

// Render produces the OpenCode config JSON for the given params. Every value is
// JSON-encoded via the template's `json` function, so the output is always
// valid JSON even if a name contains quotes or other special characters.
func Render(p Params) (string, error) {
	tmpl, err := template.New("opencode.json.tmpl").
		Funcs(template.FuncMap{"json": jsonString}).
		ParseFS(assets.Templates, templatePath)
	if err != nil {
		return "", fmt.Errorf("parse opencode template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, p); err != nil {
		return "", fmt.Errorf("render opencode config: %w", err)
	}
	return buf.String(), nil
}

// Write renders the config and writes it to path, creating parent directories.
func Write(path string, p Params) error {
	out, err := Render(p)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create opencode config directory: %w", err)
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

func jsonString(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
