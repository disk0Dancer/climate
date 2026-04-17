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

## Demo

[disk0Dancer/github](https://github.com/disk0Dancer/github) — 1 100+ endpoint CLI from the GitHub REST API spec.

## Commands

| Command | Purpose |
|---|---|
| `generate` | Create CLI from OpenAPI spec |
| `list` | Show registered CLIs |
| `remove` | Delete a generated CLI |
| `upgrade` | Regenerate from updated spec |
| `publish` | Push CLI to GitHub with CI/release |
| `skill generate` | Emit agent skill prompt |

## Agent skills

```bash
climate skill generator                        # skill for climate itself
climate skill generate petstore --mode=compact # skill for a generated CLI
npx skills add https://github.com/disk0Dancer/climate --skill climate-generator
```

## Machine-readable

- [llms.txt](./llms.txt)
- [robots.txt](./robots.txt)

## License

Apache-2.0
