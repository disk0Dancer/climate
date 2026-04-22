# climate

[![CI](https://github.com/disk0Dancer/climate/actions/workflows/ci.yml/badge.svg)](https://github.com/disk0Dancer/climate/actions/workflows/ci.yml)
[![Release](https://github.com/disk0Dancer/climate/actions/workflows/release.yml/badge.svg)](https://github.com/disk0Dancer/climate/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/disk0Dancer/climate.svg)](https://pkg.go.dev/github.com/disk0Dancer/climate)
[![Go Report Card](https://goreportcard.com/badge/github.com/disk0Dancer/climate)](https://goreportcard.com/report/github.com/disk0Dancer/climate)
[![Homebrew](https://img.shields.io/badge/brew-disk0Dancer/tap/climate-FBB040?logo=homebrew&logoColor=white)](https://github.com/disk0Dancer/homebrew-tap)
[![skills.sh](https://img.shields.io/badge/skills.sh-climate--generator-111827)](https://skills.sh)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

Any API becomes a tool. Give climate an OpenAPI spec — it builds a CLI and a skill.
Works for humans. Works for AI agents. One spec → unlimited new tools.

```bash
climate generate --name petstore https://petstore3.swagger.io/api/v3/openapi.json
petstore pet get --pet-id 1
```

## Install

```bash
brew tap disk0Dancer/tap && brew install climate
```

Or `go install github.com/disk0Dancer/climate/cmd/climate@latest`.

Enable local shell completion:

```bash
climate completion install --shell zsh
```

## How it works

One command turns an OpenAPI 3.x spec into a compiled Go binary with auth, JSON output, and structured errors.

```bash
climate generate --name myapi https://api.example.com/openapi.json
myapi <group> <operation> [flags] --output=json|table|raw
```

Generated CLIs also ship with spec-aware event commands:

```bash
myapi events list
myapi config profiles create work
myapi config profiles use work
myapi auth login
myapi config set --secret events.signing_secret supersecret
myapi events listen payment-succeeded --port 8081 --tunnel auto --signature-mode hmac
myapi events emit payment-succeeded --target-url http://localhost:8081/webhooks/payment-succeeded --signature-mode hmac
```

## Agent skill

An agent with climate can build its own tools. Point it at any OpenAPI spec —
it generates a CLI, creates a skill, and starts using the API. No human required.

```bash
climate skill generate myapi          # skill prompt for a generated CLI
climate skill generator               # skill for climate itself
npx skills add https://github.com/disk0Dancer/climate --skill climate-generator
```

## Publish

Push a generated CLI to GitHub with CI, CI auto-fix, and release workflows:

```bash
climate publish myapi --owner disk0Dancer
```

Demo: [disk0Dancer/github](https://github.com/disk0Dancer/github) — 1 100+ endpoint CLI from the GitHub REST API spec.

## Commands

| Command | Purpose |
|---|---|
| `generate` | Create CLI from OpenAPI spec |
| `compose` | Merge multiple specs (with prefixes) into one facade CLI |
| `mock` | Run local mock HTTP server from OpenAPI spec |
| `completion` | Print shell completions or install/uninstall them locally |
| `list` | Show registered CLIs |
| `remove` | Interactively delete a generated CLI |
| `uninstall` | Remove the climate CLI itself, optionally with full cleanup |
| `upgrade` | Regenerate from updated spec |
| `publish` | Push CLI to GitHub with CI/auto-fix/release |
| `skill generate` | Emit agent skill prompt |

## Shell completion

```bash
# print a completion script
climate completion zsh

# install it into your local shell setup
climate completion install --shell zsh

# remove climate-managed completion wiring later
climate completion uninstall --shell zsh

# remove one generated CLI with confirmation
climate remove petstore

# uninstall only the climate executable
climate uninstall

# uninstall climate plus generated CLIs, manifest, and completions
climate uninstall --full
```

## Docs

- [Site](https://disk0dancer.github.io/climate/)
- [LLM index](https://disk0dancer.github.io/climate/llms.txt)
- [Compose design](docs/design-compose.md)
- [CI auto-fix design](docs/design-ci-autofix.md)
- [Mock design](docs/design-mock.md)
- [Generated event listener design](docs/design-generated-events.md)
- [Shell completion design](docs/design-shell-completions.md)
- [Uninstall design](docs/design-uninstall.md)
- [OpenAPI 3.0 support matrix](docs/openapi-3-support-matrix.md)

## Development

```bash
go build ./...
go test ./...
```

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=disk0Dancer/climate&type=Date)](https://star-history.com/#disk0Dancer/climate&Date)

## License

[Apache 2.0](LICENSE)
