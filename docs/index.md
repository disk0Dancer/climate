# climate docs

Human-first documentation for `climate`, a Go CLI that turns OpenAPI 3.x specs
into auth-aware command-line tools and agent-ready skills.

Canonical site: <https://disk0dancer.github.io/climate/>
Repository: <https://github.com/disk0Dancer/climate>

## Install

### Go

```bash
go install github.com/disk0Dancer/climate/cmd/climate@latest
```

### GitHub Releases

```bash
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

Tap repository:
<https://github.com/disk0Dancer/homebrew-tap>

## Quick start

Generate a CLI from an OpenAPI spec:

```bash
climate generate --name petstore https://petstore3.swagger.io/api/v3/openapi.json
```

Use the generated CLI:

```bash
petstore pet list --output=json
petstore pet get --pet-id 1 --output=json
petstore pet add --data-json '{"name":"Fido","status":"available"}' --output=json
```

Inspect locally generated CLIs:

```bash
climate list
```

Publish a generated CLI to GitHub:

```bash
export GITHUB_TOKEN=github_pat_your_token
climate publish petstore --owner disk0Dancer
```

## Generated CLI shape

```bash
<cli-name> <tag> <operation> [flags] --output=json|table|raw
```

- first-level subcommand: OpenAPI tag
- second-level subcommand: generated operation command
- path parameters: required flags
- query parameters: optional flags
- request body: `--data-json` or `--data-file`
- success and error output: structured JSON

## Agents and skills.sh

Print the built-in climate skill:

```bash
climate skill generator
```

Install the repo skill with skills.sh-compatible tooling:

```bash
npx skills add https://github.com/disk0Dancer/climate --skill climate-generator
```

Generate a skill for a CLI you just created:

```bash
climate skill generate petstore --mode=full
climate skill generate petstore --mode=compact
```

Publish the generated CLI into a GitHub repository with managed lifecycle files:

```bash
climate publish petstore --owner disk0Dancer
```

Relevant files:

- `skills/climate.md`
- `skills/climate-generator/SKILL.md`
- `docs/llms.txt`

## Distribution

Use these channels first:

- `go install`
- GitHub Releases
- Homebrew via `disk0Dancer/tap`

GitHub Packages are optional and not required for the current CLI distribution story.

## Publish lifecycle

`climate publish <cli-name>` creates or reuses a GitHub repository and bootstraps:

- `README.md`
- `.gitignore`
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`

Authentication is read from `--github-token`, `GITHUB_TOKEN`, or `GH_TOKEN`.

## Google Analytics

Analytics is opt-in.

Set the GA4 measurement ID in `docs/site-config.js`:

```js
window.CLIMATE_SITE_CONFIG = {
  googleAnalyticsMeasurementId: "G-XXXXXXXXXX",
};
```

If the value is empty, no Google Analytics script is loaded.

## License

Apache-2.0. See `LICENSE`.
