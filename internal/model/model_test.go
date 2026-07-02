package model

import (
	"testing"

	"github.com/rezonmain/loca-agent/internal/config"
)

func testRegistry() config.ModelRegistry {
	return config.ModelRegistry{
		Default: "a",
		Source: config.ModelSource{
			BaseURL:         "https://example.com",
			FileURLTemplate: "{base}/{repo}/resolve/main/{file}",
		},
		Models: []config.Model{
			{ID: "a", Repo: "org/a", Files: []config.ModelFile{{Name: "a.gguf"}}},
			{ID: "b", Repo: "org/b", Files: []config.ModelFile{{Name: "b.gguf"}}},
		},
	}
}

func TestResolveDefault(t *testing.T) {
	m, err := Resolve(testRegistry(), "")
	if err != nil {
		t.Fatalf("Resolve default: %v", err)
	}
	if m.ID != "a" {
		t.Errorf("default resolved to %q, want a", m.ID)
	}
}

func TestResolveExplicit(t *testing.T) {
	m, err := Resolve(testRegistry(), "b")
	if err != nil {
		t.Fatalf("Resolve b: %v", err)
	}
	if m.ID != "b" {
		t.Errorf("resolved to %q, want b", m.ID)
	}
}

func TestResolveUnknown(t *testing.T) {
	_, err := Resolve(testRegistry(), "nope")
	if err == nil {
		t.Fatalf("expected error for unknown model")
	}
}

func TestFileURL(t *testing.T) {
	reg := testRegistry()
	m, _ := Resolve(reg, "a")
	got := FileURL(reg.Source, m, m.Files[0])
	want := "https://example.com/org/a/resolve/main/a.gguf"
	if got != want {
		t.Errorf("FileURL = %q, want %q", got, want)
	}
}

func TestFileURLTrimsTrailingSlash(t *testing.T) {
	reg := testRegistry()
	reg.Source.BaseURL = "https://example.com/"
	m, _ := Resolve(reg, "a")
	got := FileURL(reg.Source, m, m.Files[0])
	if got != "https://example.com/org/a/resolve/main/a.gguf" {
		t.Errorf("trailing slash not handled: %q", got)
	}
}
