# Skill: climate.generator

You have access to **`climate`** — a tool that generates production-ready CLI
binaries from OpenAPI 3.x specifications and produces plain-text skill prompts
so you can self-register those CLIs as new skills.

---

## What you can do

- Generate a typed Go CLI from any OpenAPI spec (URL or local file).
- List all CLIs you have already generated.
- Get a plain-text skill prompt for any generated CLI so you can self-register it.
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
2. Run `climate generate <url>` → note the `cli_name` in the JSON response.
3. Run `climate skill generate <cli_name>` → read the plain-text prompt it prints.
4. Follow the self-registration instructions inside that prompt.
5. Use the new CLI skill for all subsequent tasks that involve that API.

---

## Installation

```bash
# via Go
go install github.com/disk0Dancer/climate/cmd/climate@latest

# via Homebrew
brew tap disk0dancer/tap
brew install climate
```

Binaries for Linux, macOS, and Windows are also available on the
[GitHub Releases](https://github.com/disk0Dancer/climate/releases) page.
