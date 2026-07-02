// Package assets embeds the default configuration files that ship inside the
// bootstrap-ai binary. These built-in defaults are always available and are
// merged with any user-provided overrides at runtime (see internal/config).
//
// Keeping the canonical YAML in configs/ at the repository root lets both
// humans (editing files) and the binary (embedding them) share one source of
// truth. Because //go:embed cannot reference paths above the embedding source
// file, this file — and the module's go.mod — live at the repository root.
package assets

import "embed"

// Defaults contains the shipped configuration files, addressable as
// "configs/<name>.yaml" through the embedded fs.FS.
//
//go:embed configs/*.yaml
var Defaults embed.FS

// Templates contains the text/template files rendered at install time
// (WireGuard configs, OpenCode config, llama.cpp launch args), addressable as
// "templates/<area>/<name>.tmpl".
//
//go:embed templates/wireguard/*.tmpl
var Templates embed.FS
