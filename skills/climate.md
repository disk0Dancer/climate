# Skill: climate.generator

You have access to **`climate`** — a tool that generates production-ready CLI
binaries from OpenAPI 3.x specifications and produces plain-text skill prompts
so you can self-register those CLIs as new skills.

---

## What you can do

- Generate a typed Go CLI from any OpenAPI spec (URL or local file).
- Compose several OpenAPI specs into one facade CLI with per-spec path prefixes.
- Run a local OpenAPI-based mock HTTP server for simulator/sandbox workflows.
- Generate CLIs that can run spec-aware webhook/callback commands with optional cloudflared exposure and a local config store.
- Generate shell completion scripts for climate and manage local install/uninstall.
- List all CLIs you have already generated.
- Get a plain-text skill prompt for any generated CLI so you can self-register it.
- Publish a generated CLI into a GitHub repository with lifecycle bootstrap.
- Remove or upgrade a previously generated CLI.
- Uninstall the climate CLI itself, with optional full cleanup of climate-managed local artifacts.

---

## Commands

### Generate a CLI

```
climate generate [--name <cli-name>] [--out-dir <dir>] [--no-build] [--force] <openapi_spec>
```

| Flag | Description |
|------|-------------|
| `<openapi_spec>` | **Required.** URL or path to an OpenAPI 3.0/3.1 JSON or YAML spec. |
| `--name` | Override the binary name (default: normalised `info.title`). |
| `--out-dir` | Where to write generated Go source code. |
| `--no-build` | Skip compiling the binary (generate source only). |
| `--force` | Overwrite an existing output directory. |

**Output (JSON on success):**

```json
{
  "cli_name":    "<name>",
  "binary_path": "<absolute path to compiled binary>",
  "source_dir":  "<absolute path to generated source>",
  "version":     "<API version from spec>",
  "openapi_hash":"<sha256 of the spec>"
}
```

Generated CLIs also include:

```
<cli-name> events list
<cli-name> config list
<cli-name> config set <key> <value>
<cli-name> config get <key>
<cli-name> config unset <key>
<cli-name> config profiles list
<cli-name> config profiles create <name>
<cli-name> config profiles use <name>
<cli-name> config set --secret events.signing_secret <value>
<cli-name> auth login [--scheme <name>]
<cli-name> auth status
<cli-name> auth logout [--scheme <name>]
<cli-name> events listen [event-name] [--host 127.0.0.1] [--port 8081] [--path /] [--tunnel none|auto|cloudflared] [--signature-mode none|hmac]
<cli-name> events emit <event-name> --target-url <url> [--data-json <json>] [--data-file <path>] [--signature-mode none|hmac]
```

Use this to inspect generated callback/webhook names, receive them locally, and
emit synthetic payloads. Named profiles act like lightweight gcloud-style
profiles, and `auth login` can interactively store credentials or fetch/store
OAuth2 tokens when the API exposes compatible flows. `config set --secret
events.signing_secret ...` stores the signing secret for later use. `--tunnel
auto` exposes the listener through `cloudflared`. HMAC signing is configurable
via header name, algorithm, and optional timestamp signing.

---

### Compose multiple specs into one facade CLI

```
climate compose [--name <cli-name>] [--out-dir <dir>] [--no-build] [--force] [--title <title>] [--api-version <version>] [--description <text>] <spec1>:<prefix1> [<spec2>:<prefix2> ...]
```

Use this for multi-microservice setups where each service has its own spec.
Each input is `<spec>:<prefix>`, for example:

```
climate compose orders.yaml:/api/orders users.yaml:/api/users
climate compose https://orders.svc/openapi.json:/orders https://users.svc/openapi.json:/users
```

---

### Run a local mock server from an OpenAPI spec

```
climate mock [--port <port>] [--latency <ms>] [--emit-url <url> --event-path <path> [--event-method <method>]] <openapi_spec>
```

Starts a local simulator server that serves synthetic responses from response
schemas in the OpenAPI spec. Useful for local development and agent testing.
For webhook-style integrations, it can also emit one synthetic event payload to
a target endpoint and exit.

---

### List generated CLIs

```
climate list
```

Returns JSON with a `clis` array describing every CLI registered in the local
manifest (`~/.climate/manifest.json`).

---

### Generate or manage shell completions for climate

```
climate completion bash|zsh|fish|powershell
climate completion install [--shell bash|zsh|fish|powershell]
climate completion uninstall [--shell bash|zsh|fish|powershell]
```

`climate completion <shell>` prints the raw completion script to stdout.
`install` writes a climate-managed script file and updates the relevant local
shell config when needed. `uninstall` removes only climate-managed completion
files and config blocks.

---

### Get a skill prompt for a generated CLI

```
climate skill generate [--mode=full|compact] [--out <file>] <cli-name>
```

| Flag | Description |
|------|-------------|
| `--mode=full` | One documented command per API operation (default). |
| `--mode=compact` | Shorter summary grouped by tag. |
| `--out <file>` | Write the prompt to a file instead of stdout. |

Prints a **plain-text Markdown prompt** you should read and follow to
self-register the CLI as a new skill.

---

### Publish a generated CLI to GitHub

```
climate publish [--owner <owner>] [--repo <repo>] [--visibility public|private] [--github-token <token>] <cli-name>
```

Creates or reuses a GitHub repository through the GitHub API, writes a
bootstrap README plus CI/auto-fix/release workflows, initializes git, and pushes the
generated source tree.

Authentication is read from `--github-token`, `GITHUB_TOKEN`, or `GH_TOKEN`.

---

### Remove a generated CLI

```
climate remove [--purge-sources] [--yes] <cli-name>
```

Prompts before deletion by default. Removes the binary and manifest entry.
`--purge-sources` also deletes the generated source directory. `--yes` skips
the prompt.

---

### Uninstall the climate CLI itself

```
climate uninstall [--full] [--yes]
```

Detects whether climate was installed via Homebrew, `go install`, or a
standalone binary and removes it the right way for that installation method.

- Default mode removes only the climate executable.
- `--full` also removes generated CLIs recorded in the manifest, their source
  directories, the manifest file, and climate-managed shell completion wiring.
- `--yes` skips the prompt.

---

### Re-generate from an updated spec

```
climate upgrade [--openapi <spec>] <cli-name>
```

Re-generates and rebuilds a CLI. Pass `--openapi` to use a different spec.

---

## Output format

Most climate management commands exit 0 and print JSON to stdout.
Text-printing commands include:

- `climate skill generate`
- `climate skill generator`
- `climate completion <shell>`
- `climate mock` runtime output

On error commands exit non-zero and print structured JSON to stderr:

```json
{
  "error": {
    "status":  0,
    "code":    "CliError",
    "message": "<human-readable description>"
  }
}
```

---

## Typical agent workflow

1. User provides an OpenAPI spec URL or file path.
2. If it is one spec, run `climate generate <url>`; if many microservices, run `climate compose <spec:prefix>...`.
3. Note the `cli_name` in the JSON response.
4. Run `climate skill generate <cli_name>` → read the plain-text prompt it prints.
5. Optional: run `climate mock <openapi_spec>` for local simulator/sandbox testing.
6. Optional: run `climate completion install --shell zsh` if the user wants tab completion for climate itself.
7. Run `climate publish <cli_name>` if the user wants the generated CLI managed on GitHub.
8. Follow the self-registration instructions inside that prompt.
9. Use the new CLI skill for all subsequent tasks that involve that API.
10. If the user wants to remove a generated CLI later, prefer `climate remove` and let the confirmation prompt guard accidental deletion.

---

## Installation

```bash
# via Go
go install github.com/disk0Dancer/climate/cmd/climate@latest

# via Homebrew
brew tap disk0Dancer/tap
brew install climate

# via GitHub Releases
curl -L https://github.com/disk0Dancer/climate/releases/latest/download/climate-darwin-arm64.tar.gz | tar xz
sudo mv climate-darwin-arm64 /usr/local/bin/climate
```

Binaries for Linux, macOS, and Windows are also available on the
[GitHub Releases](https://github.com/disk0Dancer/climate/releases) page.
