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

## How it works

One command turns an OpenAPI 3.x spec into a compiled Go binary with auth, JSON output, and structured errors.

```bash
climate generate --name myapi https://api.example.com/openapi.json
myapi <group> <operation> [flags] --output=json|table|raw
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

Push a generated CLI to GitHub with CI and release workflows:

```bash
climate publish myapi --owner disk0Dancer
```

Demo: [disk0Dancer/github](https://github.com/disk0Dancer/github) — 1 100+ endpoint CLI from the GitHub REST API spec.

## Commands

| Command | Purpose |
|---|---|
| `generate` | Create CLI from OpenAPI spec |
| `list` | Show registered CLIs |
| `remove` | Delete a generated CLI |
| `upgrade` | Regenerate from updated spec |
| `publish` | Push CLI to GitHub with CI/release |
| `skill generate` | Emit agent skill prompt |

## Docs

- [Site](https://disk0dancer.github.io/climate/)
- [LLM index](https://disk0dancer.github.io/climate/llms.txt)

## Development

```bash
go build ./...
go test ./...
```

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=disk0Dancer/climate&type=Date)](https://star-history.com/#disk0Dancer/climate&Date)

## License

[Apache 2.0](LICENSE)
