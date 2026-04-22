# Design: OpenAPI Spec Composition (`climate compose`)

## Overview

`climate compose` merges multiple OpenAPI 3.x specifications — each assigned a
dedicated path prefix — into a single composite specification and generates a
CLI from the result.  The CLI acts as a **facade** over several microservices:
one binary, one authentication model, all services.

## Motivation

Modern back-ends are decomposed into microservices.  Each service owns its own
OpenAPI document.  Without tooling, developers and agents must juggle many
separate CLIs or issue raw HTTP calls to reach different services.  A single
gateway CLI is far more ergonomic and aligns with the Backends-for-Frontends
(BFF) pattern.

## Usage

```bash
# Two microservices, each mounted under a different prefix
climate compose \
  orders.yaml:/api/orders \
  users.yaml:/api/users

# With all optional flags
climate compose \
  --name gateway \
  --title "My Gateway API" \
  --api-version 2.0.0 \
  --description "Unified facade over Orders and Users services" \
  --out-dir /tmp/gateway \
  --force \
  https://orders.svc/openapi.json:/orders \
  https://users.svc/openapi.json:/users
```

Each positional argument has the form `<spec>:<prefix>` where `<spec>` is a
local file or URL and `<prefix>` starts with `/`.

## Algorithm

The merge is performed by `internal/compose.Merge`:

1. **Load** — each source spec is loaded and validated with `spec.Load`.
2. **Namespace components** — every schema, parameter, and security scheme
   defined under `components/` is renamed to `<ns>-<original>` where `<ns>` is
   derived from the prefix (e.g. `/api/orders` → `api-orders`).  This prevents
   name collisions between services that happen to share component names.
3. **Rewrite `$ref`s** — every `$ref` inside the spec (operation parameters,
   request bodies, responses) is updated to point at the new namespaced name.
4. **Prefix paths** — every path key is prepended with the caller-specified
   prefix (trailing slashes on the prefix are removed; the path's leading `/`
   is preserved).
5. **Merge** — paths, components and tags are accumulated into a single output
   `spec.OpenAPI` struct.  Tags are de-duplicated by name.  Security schemes
   follow a last-writer-wins strategy so that a centralised gateway scheme can
   override individual service schemes.
6. **Generate** — the merged spec is passed to `generator.Generate` exactly as
   a regular `climate generate` would; the resulting CLI binary works against
   all services.

## Authentication

Security schemes from all input specs are merged into the composed document.
The recommended approach for a real gateway is to pass a shared Bearer-token
scheme via the first or only input that defines one, letting the gateway
validate it and forward downstream with service-account tokens.  Alternatively,
pass a custom `--auth` header per sub-command using the generated CLI's
persistent flags.

## Component namespacing example

Given two services that each define a `components/schemas/Error` schema:

| Service | Original ref | Namespaced ref |
|---------|-------------|----------------|
| `/api/orders` | `#/components/schemas/Error` | `#/components/schemas/api-orders-Error` |
| `/api/users`  | `#/components/schemas/Error` | `#/components/schemas/api-users-Error` |

Both schemas are preserved in the merged document without conflict.

## Limitations

- Only OpenAPI 3.x specs are supported (same constraint as `climate generate`).
- `allOf` / `oneOf` / `anyOf` schema combiners are not rewritten in the
  current implementation; plain `$ref` strings are rewritten.
- The generated CLI uses the primary merged server URL and can override it via
  `--base-url` (or `<CLI>_BASE_URL`). If the server URL contains template
  variables (`{region}`), the generated CLI also supports
  `--server-var-<name>` / `<CLI>_SERVER_VAR_<NAME>` overrides.
- If you want a true API gateway (single ingress), deploy a reverse proxy in
  front of the services and point `--base-url` at it.
