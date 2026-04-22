---
name: climate-generator
description: >
  Generate Go CLIs from OpenAPI 3.x specs with climate, inspect generated CLIs,
  and emit Markdown skill prompts so those CLIs can be attached to agent workflows.
license: Apache-2.0
compatibility: Requires Go 1.21+ and climate CLI (brew install disk0Dancer/tap/climate)
metadata:
  author: disk0Dancer
  version: "1.0"
---

# Skill: climate-generator

You have access to `climate`, a CLI that generates production-ready Go command-line
clients from OpenAPI specifications and can emit Markdown prompts for agent skills.

## When to use this skill

- The user has an OpenAPI 3.x URL or file and wants a CLI quickly.
- The user wants a human-usable API client rather than writing SDK glue code.
- The user wants to turn a generated CLI into a reusable agent skill.
- The user wants to compose multiple microservice specs into one facade CLI.
- The user wants a local OpenAPI simulator (mock server) for testing.
- The user wants to list, remove, or upgrade a previously generated CLI.

## Core workflow

1. Generate a CLI from the provided spec.
2. If user provides multiple specs, use `climate compose` instead of `climate generate`.
3. Capture the resulting `cli_name`, `binary_path`, and `source_dir`.
4. If the user wants agent integration, run `climate skill generate <cli-name>`.
5. If the user needs sandbox/simulator behavior, run `climate mock <openapi_spec>`.
6. If the user wants the CLI managed on GitHub, run `climate publish <cli-name>`.
7. Follow the generated instructions from that Markdown prompt.

## Commands

### Generate a CLI

```bash
climate generate [--name <cli-name>] [--out-dir <dir>] [--no-build] [--force] <openapi_spec>
```

- `<openapi_spec>` can be a local path or an HTTP(S) URL.
- `--no-build` generates source only.
- `--force` overwrites an existing output directory.

Success output is JSON:

```json
{
  "cli_name": "<name>",
  "binary_path": "<absolute path to compiled binary>",
  "source_dir": "<absolute path to generated source>",
  "version": "<API version>",
  "openapi_hash": "<sha256 of the spec>"
}
```

### List generated CLIs

```bash
climate list
```

### Compose multiple OpenAPI specs into one CLI

```bash
climate compose [--name <cli-name>] [--out-dir <dir>] [--no-build] [--force] [--title <title>] [--api-version <version>] [--description <text>] <spec1>:<prefix1> [<spec2>:<prefix2> ...]
```

Each positional argument is `<spec>:<prefix>` where `<spec>` can be a local
path or URL and `<prefix>` starts with `/`.

### Start local mock server

```bash
climate mock [--port <port>] [--latency <ms>] [--emit-url <url> --event-path <path> [--event-method <method>]] <openapi_spec>
```

### Remove a generated CLI

```bash
climate remove [--purge-sources] <cli-name>
```

### Upgrade a generated CLI

```bash
climate upgrade [--openapi <spec>] <cli-name>
```

### Generate a skill prompt for a CLI

```bash
climate skill generate [--mode=full|compact] [--out <file>] <cli-name>
```

### Publish a generated CLI to GitHub

```bash
climate publish [--owner <owner>] [--repo <repo>] [--visibility public|private] [--github-token <token>] <cli-name>
```

This command creates or reuses a GitHub repository through the GitHub API,
writes lifecycle files, initializes git, and pushes the generated source tree.

### Print the built-in climate skill

```bash
climate skill generator
```

## Typical examples

Generate a CLI:

```bash
climate generate --name petstore https://petstore3.swagger.io/api/v3/openapi.json
```

Compose two service specs into one facade CLI:

```bash
climate compose orders.yaml:/api/orders users.yaml:/api/users --name gateway
```

Run a local mock server:

```bash
climate mock --port 9090 --latency 150 https://petstore3.swagger.io/api/v3/openapi.json
```

Generate a compact skill prompt for that CLI:

```bash
climate skill generate petstore --mode=compact
```

Publish it to GitHub:

```bash
climate publish petstore --owner disk0Dancer
```

## Notes

- All climate output is JSON on success.
- Errors are emitted as structured JSON on stderr.
- Generated CLIs follow the shape `<cli-name> <tag> <operation> [flags] --output=json|table|raw`.
- Homebrew install is available via `brew tap disk0Dancer/tap && brew install climate`.
- GitHub publish auth is read from `--github-token`, `GITHUB_TOKEN`, or `GH_TOKEN`.
