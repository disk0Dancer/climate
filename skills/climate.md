# Skill: climate.generator

You have access to **`climate`** — a tool that generates production-ready CLI
binaries from OpenAPI 3.x specifications and produces plain-text skill prompts
so you can self-register those CLIs as new skills.

---

## What you can do

- Generate a typed Go CLI from any OpenAPI spec (URL or local file).
- Compose several OpenAPI specs into one facade CLI with per-spec path prefixes.
- Run a local OpenAPI-based mock HTTP server for simulator/sandbox workflows.
- List all CLIs you have already generated.
- Get a plain-text skill prompt for any generated CLI so you can self-register it.
- Publish a generated CLI into a GitHub repository with lifecycle bootstrap.
- Remove or upgrade a previously generated CLI.

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
climate mock [--port <port>] [--latency <ms>] <openapi_spec>
```

Starts a local simulator server that serves synthetic responses from response
schemas in the OpenAPI spec. Useful for local development and agent testing.

---

### List generated CLIs

```
climate list
```

Returns JSON with a `clis` array describing every CLI registered in the local
manifest (`~/.climate/manifest.json`).

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
bootstrap README plus CI/release workflows, initializes git, and pushes the
generated source tree.

Authentication is read from `--github-token`, `GITHUB_TOKEN`, or `GH_TOKEN`.

---

### Remove a generated CLI

```
climate remove [--purge-sources] <cli-name>
```

Removes the binary and manifest entry. `--purge-sources` also deletes the
generated source directory.

---

### Re-generate from an updated spec

```
climate upgrade [--openapi <spec>] <cli-name>
```

Re-generates and rebuilds a CLI. Pass `--openapi` to use a different spec.

---

## Output format

On success all commands exit 0 and print JSON to stdout.

On error commands exit non-zero and print to stderr:

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
6. Run `climate publish <cli_name>` if the user wants the generated CLI managed on GitHub.
7. Follow the self-registration instructions inside that prompt.
8. Use the new CLI skill for all subsequent tasks that involve that API.

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
