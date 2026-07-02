// Command bootstrap-ai installs, configures, and manages self-hosted AI coding
// infrastructure: a WireGuard-linked macOS OpenCode client and a Windows
// llama.cpp inference server.
//
// This file is intentionally thin — all behavior lives in internal/cli and the
// per-command packages under internal/command.
package main

import (
	"os"

	"github.com/rezonmain/loca-agent/internal/cli"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(cli.Execute(version))
}
