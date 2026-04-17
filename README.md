# climate — OpenAPI to Go CLI generation for humans and agents

[![CI](https://github.com/disk0Dancer/climate/actions/workflows/ci.yml/badge.svg)](https://github.com/disk0Dancer/climate/actions/workflows/ci.yml)
[![Release](https://github.com/disk0Dancer/climate/actions/workflows/release.yml/badge.svg)](https://github.com/disk0Dancer/climate/releases)
[![Docs](https://img.shields.io/badge/docs-GitHub%20Pages-0a7a5a)](https://disk0dancer.github.io/climate/)
[![Go](https://img.shields.io/badge/go-1.21%2B-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![skills.sh](https://img.shields.io/badge/skills.sh-compatible-111827)](https://skills.sh/docs)

**climate** turns an OpenAPI 3.0/3.1 spec into a compiled Go CLI with built-in
authentication support, structured JSON output, and agent-friendly skill prompts.

The repository now treats the website as a documentation landing first. The
HTML site is for people, while machine-readable companions live next to it.

Docs:

- Site: <https://disk0dancer.github.io/climate/>
- Markdown companion: <https://disk0dancer.github.io/climate/index.md>
- LLM index: <https://disk0dancer.github.io/climate/llms.txt>

## Why climate

- One command from OpenAPI spec to runnable Go CLI.
- Auth baked in for API key, bearer, basic, and OAuth2 client credentials.
- Stable command shape across generated CLIs.
- JSON output everywhere, including structured errors.
- Built-in skill generation for agent workflows.
- One-command publish flow for generated CLIs into GitHub repos with CI/release bootstrap.
- Repo-level skill packaging for `skills.sh`-compatible tooling.

## Installation

### Go

```bash
go install github.com/disk0Dancer/climate/cmd/climate@latest
```

Requires Go 1.21+.

### GitHub Releases

Download archives for Linux, macOS, and Windows from the
[Releases page](https://github.com/disk0Dancer/climate/releases).

```bash
# macOS arm64 example
curl -L https://github.com/disk0Dancer/climate/releases/latest/download/climate-darwin-arm64.tar.gz | tar xz
sudo mv climate-darwin-arm64 /usr/local/bin/climate
```

### Homebrew

```bash
brew tap disk0Dancer/tap
brew install climate

# or directly
brew install disk0Dancer/tap/climate
```

This uses the published tap repository:
<https://github.com/disk0Dancer/homebrew-tap>

## skills.sh

The repo now includes a `skills.sh`-compatible skill definition at
[`skills/climate-generator/SKILL.md`](skills/climate-generator/SKILL.md).

Install it with:

```bash
npx skills add https://github.com/disk0Dancer/climate --skill climate-generator
```

You can also print the built-in embedded skill directly from the binary:

```bash
climate skill generator
```

## Quick start

Generate a CLI from a remote OpenAPI spec:

```bash
climate generate --name petstore https://petstore3.swagger.io/api/v3/openapi.json
```

Use the generated CLI:

```bash
petstore pet list --output=json
petstore pet get --pet-id 1 --output=json
petstore pet add --data-json '{"name":"Fido","status":"available"}' --output=json
```

List locally registered CLIs:

```bash
climate list
```

Publish a generated CLI to GitHub and bootstrap lifecycle files:

```bash
export GITHUB_TOKEN=github_pat_your_token
climate publish petstore --owner disk0Dancer
```

## Generated CLI shape

Each generated CLI follows the same interface:

```bash
<cli-name> <tag> <operation> [flags] --output=json|table|raw
```

- level 1 subcommand: OpenAPI tag
- level 2 subcommand: generated operation command
- path parameters: required flags
- query parameters: optional flags
- request body: `--data-json` or `--data-file`

Example:

```bash
petstore pet get --pet-id 1 --output=json
```

## Agent workflow

climate has two agent-facing layers:

1. `climate skill generator` prints the built-in skill for climate itself.
2. `climate skill generate <cli-name>` prints a skill prompt for a generated CLI.
3. `climate publish <cli-name>` pushes a generated CLI into a GitHub repository with CI/release scaffolding.

Typical flow:

```bash
climate generate https://api.example.com/openapi.yaml
climate skill generate myapi --mode=full
climate publish myapi --owner disk0Dancer
```

Relevant files:

- [`skills/climate.md`](skills/climate.md)
- [`skills/climate-generator/SKILL.md`](skills/climate-generator/SKILL.md)

## Publish to GitHub

`climate publish` is the lifecycle handoff for generated CLIs.

It will:

- create or reuse a GitHub repository through the GitHub API
- write climate-managed repository files if they are missing or still managed by climate
- initialize git, add `origin`, create the default branch, and push the source tree
- persist repository metadata back into `~/.climate/manifest.json`

Command example:

```bash
export GITHUB_TOKEN=github_pat_your_token
climate publish petstore \
  --owner disk0Dancer \
  --repo petstore-cli \
  --visibility public
```

The generated repository bootstrap includes:

- `README.md`
- `.gitignore`
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`

## Documentation assets

The docs site is intentionally split into human and machine-friendly files:

- [`docs/index.html`](docs/index.html): primary human documentation landing
- [`docs/index.md`](docs/index.md): Markdown companion
- [`docs/llms.txt`](docs/llms.txt): short machine-readable index
- [`docs/robots.txt`](docs/robots.txt): crawler policy
- [`docs/site-config.js`](docs/site-config.js): opt-in Google Analytics config

## Google Analytics

The site supports optional GA4 tracking.

To enable it, set the measurement ID in `docs/site-config.js`:

```js
window.CLIMATE_SITE_CONFIG = {
  googleAnalyticsMeasurementId: "G-XXXXXXXXXX",
};
```

If the value is empty, the site will not load Google Analytics.

## Distribution notes

For this project shape, GitHub Packages are not required right now.

Recommended channels:

- `go install`
- GitHub Releases
- Homebrew via `disk0Dancer/tap`

Add GitHub Packages only if you decide to distribute bottles or another
package format there.

## Project layout

```text
climate/
├── cmd/climate/                    # cobra commands
├── docs/                           # GitHub Pages documentation landing
├── scripts/                        # release/publishing helpers
├── internal/                       # generator, spec, auth, manifest, skill logic
│   ├── githubutil/                 # GitHub REST API client for publishing
│   ├── publish/                    # repo bootstrap and git sync for generated CLIs
│   └── ...
├── skills/
│   ├── climate.md                  # embedded skill for climate itself
│   └── climate-generator/SKILL.md  # skills.sh-compatible skill definition
└── .github/workflows/              # CI, release, pages
```

## Development

```bash
go build ./...
go test ./...
```

## CI/CD

| Workflow | Trigger | Action |
| --- | --- | --- |
| `ci.yml` | push / PR to `main` | `go test -race ./...`, `go vet`, `go build ./...` |
| `release.yml` | push tag `v*` | build release archives, publish GitHub Releases, and sync `disk0Dancer/homebrew-tap` |
| `pages.yml` | push to `main` | deploy `docs/` to GitHub Pages |

To enable the Homebrew sync step, configure the repository secret
`HOMEBREW_TAP_TOKEN` with write access to `disk0Dancer/homebrew-tap`.

## License

[Apache License 2.0](LICENSE)
