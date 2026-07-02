# loca-agent

Self-hosted AI coding infrastructure: run a coding agent (OpenCode) on your
macOS machine while a Windows box with an AMD GPU serves a local coding model
through an OpenAI-compatible llama.cpp endpoint. All traffic flows exclusively
over a WireGuard tunnel.

```
macOS client (OpenCode) ──WireGuard──▶ Windows server (llama.cpp + GPU)
```

The `bootstrap-ai` CLI detects your hardware, installs the right components for
each machine role, wires up WireGuard, downloads the selected model, and
verifies the whole path with a `doctor` health check.

> The model is **not** stored in this repository. It is downloaded during
> installation based on `configs/models.yaml`.

## Quick start

```sh
make build
./bin/bootstrap-ai --help
./bin/bootstrap-ai doctor
./bin/bootstrap-ai models
```

## Repository layout

| Path                | Purpose                                                     |
| ------------------- | ----------------------------------------------------------- |
| `cmd/bootstrap-ai/` | CLI entrypoint                                              |
| `internal/`         | Application packages (config, platform, cli commands, …)    |
| `configs/`          | Shipped default configuration (embedded into the binary)    |
| `templates/`        | Rendered config templates (WireGuard, OpenCode, llama args) |
| `scripts/install/`  | One-line bootstrap scripts for Windows / macOS              |
| `docs/`             | Architecture, installation, networking, troubleshooting, …  |

## Status

Under active, phased development. See `docs/roadmap.md` (coming in a later
phase) for what is implemented and what is planned.

## License

MIT — see [LICENSE](LICENSE).
