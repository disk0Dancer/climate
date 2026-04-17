# climate — OpenAPI → production-ready CLI on Go

[![CI](https://github.com/disk0Dancer/climate/actions/workflows/ci.yml/badge.svg)](https://github.com/disk0Dancer/climate/actions/workflows/ci.yml)
[![Release](https://github.com/disk0Dancer/climate/actions/workflows/release.yml/badge.svg)](https://github.com/disk0Dancer/climate/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/disk0Dancer/climate.svg)](https://pkg.go.dev/github.com/disk0Dancer/climate)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**climate** is a single binary that turns any OpenAPI 3.0/3.1 spec into a
compiled Go CLI with full authentication support, structured JSON output, and
built-in LLM-agent skill integration.

> **Landing page:** <https://disk0Dancer.github.io/climate>

---

## Features

- 🚀 **One command** to go from spec URL/file to a working binary.
- 🔐 **Auth built-in** — API key (header / query / cookie), HTTP bearer & basic, OAuth2 client credentials.
- 📦 **Structured output** — all responses are JSON; errors follow a consistent schema.
- 🤖 **LLM-agent ready** — `climate skill generate` emits a Markdown prompt so an agent can self-register the CLI as a skill.
- 🧩 **skills.sh** — `skills/climate.md` is the ready-to-use skill for climate itself.

---

## Installation

### Via Go

```bash
go install github.com/disk0Dancer/climate/cmd/climate@latest
```

Requires **Go 1.21+**.

### Via Homebrew

```bash
brew tap disk0dancer/tap
brew install climate
```

### Pre-built binaries (GitHub Releases)

Download archives for Linux, macOS, and Windows (amd64/arm64) from the
[Releases page](https://github.com/disk0Dancer/climate/releases).

```bash
# Linux amd64 example
curl -L https://github.com/disk0Dancer/climate/releases/latest/download/climate_linux_amd64.tar.gz | tar xz
sudo mv climate /usr/local/bin/
```

---

## Quick Start

```bash
# Generate a CLI from a remote spec
climate generate --name petstore https://petstore3.swagger.io/api/v3/openapi.json

# Use the generated CLI
petstore pet list --output=json
petstore pet get --pet-id 1 --output=json
petstore pet add --data-json '{"name":"Fido","status":"available"}' --output=json
```

### Authentication

Credentials are resolved in order: **CLI flag → ENV variable**.

```bash
# API key
export PETSTORE_APIKEY_API_KEY=secret
petstore pet list

# Bearer token
export PETSTORE_TOKEN=my-token
petstore pet list

# HTTP Basic
export PETSTORE_USERNAME=user
export PETSTORE_PASSWORD=pass
petstore pet list

# OAuth2 — pass an existing token directly (highest priority)
petstore pet list --token my-access-token

# OAuth2 — token from ENV (second priority)
export PETSTORE_TOKEN=my-access-token
petstore pet list

# OAuth2 — client credentials flow (token fetched automatically when no token is set)
export PETSTORE_CLIENT_ID=id
export PETSTORE_CLIENT_SECRET=secret
petstore pet list
```

---

## climate Commands

| Command | Description |
|---------|-------------|
| `climate generate [flags] <spec>` | Generate & build a CLI from an OpenAPI spec (URL or file path). |
| `climate list` | List all generated CLIs in the local manifest (`~/.climate/manifest.json`). |
| `climate remove <name>` | Remove a CLI binary and manifest entry. Add `--purge-sources` to also remove generated source. |
| `climate upgrade <name>` | Re-generate and rebuild a CLI (optionally from a new spec with `--openapi`). |
| `climate skill generate <name>` | Print a plain-text Markdown prompt an LLM agent reads to self-register the CLI as a skill. |
| `climate skill generator` | Print the `climate.generator` skill file (`skills/climate.md`). |

### `generate` flags

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | normalized `info.title` | Override the binary name. |
| `--out-dir` | `~/.climate/src/<name>` | Where to write generated source code. |
| `--no-build` | false | Skip `go build` (generate source only). |
| `--force` | false | Overwrite existing output directory. |

### Output (JSON)

```json
{
  "cli_name":    "petstore",
  "binary_path": "/home/user/.climate/bin/petstore",
  "source_dir":  "/home/user/.climate/src/petstore",
  "version":     "1.0.0",
  "openapi_hash": "sha256:..."
}
```

---

## Generated CLI Interface

Each generated CLI follows the same conventions:

```
<cli-name> <tag> <operation> [flags] --output=json|table|raw
```

- **Level 1 subcommand** — OpenAPI tag (e.g. `pet`, `store`, `user`).
- **Level 2 subcommand** — operation (`list`, `get`, `create`, `update`, `delete`, or `operationId`).
- **Path params** — required flags (`--pet-id 42`).
- **Query params** — optional flags (`--status available`).
- **Request body** — `--data-json '{"key":"value"}'` or `--data-file body.json`.

### Error format

```json
{
  "error": {
    "status":  404,
    "code":    "HTTPError",
    "message": "Not Found",
    "raw":     { }
  }
}
```

CLI errors (missing params, bad config) use `"status": 0` and `"code": "CliError"`.

---

## LLM Agent Integration

climate ships a static Markdown skill for climate itself:

```bash
# Print the climate.generator skill — paste into your agent's system prompt
climate skill generator
```

Once an agent has the climate.generator skill, it can:

1. Generate a CLI for any API: `climate generate <spec>`.
2. Get a skill prompt for the new CLI: `climate skill generate <cli-name>`.
3. Self-register the CLI and use it for all subsequent API calls.

```bash
# Generate skill prompt for an existing CLI
climate skill generate petstore --mode=full   # one entry per operation
climate skill generate petstore --mode=compact # grouped by tag
```

---

## Project Layout

```
climate/
├── cmd/climate/            # cobra commands (generate, list, remove, upgrade, skill)
├── internal/
│   ├── auth/              # auth scheme types & ENV helpers
│   ├── generator/         # Go code generator (root.go, commands.go, client.go, go.mod)
│   ├── manifest/          # ~/.climate/manifest.json CRUD
│   ├── skill/             # plain-text Markdown prompt generator
│   └── spec/              # OpenAPI 3.x loader, validator, types
├── skills/
│   └── climate.md          # static climate.generator skill (embedded into binary)
├── docs/                  # GitHub Pages landing page
└── .github/workflows/     # CI, release, pages
```

---

## Development

```bash
# Build
go build ./...

# Test
go test ./...

# Run locally
go run ./cmd/climate generate --name petstore https://petstore3.swagger.io/api/v3/openapi.json
```

---

## CI/CD

| Workflow | Trigger | Action |
|----------|---------|--------|
| `ci.yml` | push / PR to `main` | `go test -race ./...` + `go vet` |
| `release.yml` | push tag `v*` | multi-platform build, GitHub Release, Homebrew tap update |
| `pages.yml` | push to `main` | deploy `docs/` to GitHub Pages |

---

## License

[MIT](LICENSE)
