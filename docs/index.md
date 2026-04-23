# climate

Any API becomes a tool. Give climate an OpenAPI spec — it builds a CLI and an agent skill.
One spec, one command, unlimited new tools.

Site: <https://disk0dancer.github.io/climate/>
Repo: <https://github.com/disk0Dancer/climate>

## Install

```bash
brew tap disk0Dancer/tap && brew install climate
```

Or `go install github.com/disk0Dancer/climate/cmd/climate@latest`.

Optional local shell completion:

```bash
climate completion install --shell zsh
```

## Quick start

```bash
climate generate --name petstore https://petstore3.swagger.io/api/v3/openapi.json
petstore pet get --pet-id 1
```

## Generated CLI shape

```
<cli> <group> <operation> [flags] --output=json|table|raw
```

- Groups = OpenAPI tags
- Operations = endpoints
- Path/query/header params → flags
- Body → `--data-json` / `--data-file`
- Auth via env vars (API key, bearer, basic, OAuth2)
- Config + auth + event commands → `<cli> config profiles ...`, `<cli> config set/get`, `<cli> auth ...`, `<cli> events ...`

Example generated-CLI workflow:

```bash
myapi config profiles create work
myapi config profiles use work
myapi auth login
myapi config set --secret events.signing_secret supersecret
myapi events listen payment-succeeded --port 8081 --tunnel auto --signature-mode hmac
```

## Demo

[disk0Dancer/github](https://github.com/disk0Dancer/github) — 1 100+ endpoint CLI from the GitHub REST API spec.

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
climate completion zsh
climate completion install --shell zsh
climate completion uninstall --shell zsh
climate remove petstore
climate uninstall
climate uninstall --full
```

## Agent skills

```bash
climate skill generator                        # skill for climate itself
climate skill generate petstore --mode=compact # skill for a generated CLI
npx skills add https://github.com/disk0Dancer/climate --skill climate-generator
```

## Machine-readable

- [llms.txt](./llms.txt)
- [robots.txt](./robots.txt)

## Design docs

- [Compose design](./design-compose.md)
- [CI auto-fix design](./design-ci-autofix.md)
- [Mock design](./design-mock.md)
- [Generated event listener design](./design-generated-events.md)
- [Shell completion design](./design-shell-completions.md)
- [Uninstall design](./design-uninstall.md)
- [OpenAPI 3.0 support matrix](./openapi-3-support-matrix.md)

## License

Apache-2.0
