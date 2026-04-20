# Design: Local API Mock Server (`climate mock`)

## Overview

`climate mock` starts a local HTTP server that serves synthetic JSON responses
for every endpoint defined in an OpenAPI 3.x specification.  It acts as a
**simulator** of the real service, letting developers and agents work against a
live HTTP interface without hitting production, without credentials, and without
side-effects.

## Motivation

During development and testing, APIs are often unavailable because:

- The service is deployed only in a remote environment.
- Credentials or network access are restricted.
- Calling the real service would produce irreversible side-effects (billing,
  e-mails, database writes).
- The service doesn't exist yet (spec-first / design-first workflow).

A mock server derived directly from the OpenAPI spec gives developers a
realistic HTTP surface to code against immediately.

## Usage

```bash
# Default port 8080
climate mock ./openapi.yaml

# Custom port
climate mock --port 9090 https://petstore3.swagger.io/api/v3/openapi.json

# Simulate network latency (milliseconds)
climate mock --latency 200 ./orders.yaml
```

On start-up the command prints a route table and the listen address:

```
Mock server for "Petstore" listening on http://localhost:8080

Routes:
  DELETE  /pets/{petId}
  GET     /pets
  GET     /pets/{petId}
  POST    /pets

Press Ctrl+C to stop.
```

## Request Matching

Paths are matched against incoming requests using regular expressions compiled
from the OpenAPI path templates.  A path parameter placeholder like `{petId}`
becomes `[^/]+` in the pattern, matching any single path segment.  Patterns
are sorted longest-first so that more specific paths take precedence.

## Response Generation

For each incoming request the server:

1. Looks up the operation for the HTTP method.
2. Selects the first response with a 2xx status code (lowest code wins).
3. Extracts the JSON schema from the response's `content` map.
4. Recursively generates a synthetic value:

| Schema type | Generated value |
|-------------|----------------|
| `object`    | `{"field": <generated value>, ...}` for each declared property |
| `array`     | `[<one generated element>]` |
| `string`    | `"example"` (or the first `enum` value if present) |
| `integer`   | `1` |
| `number`    | `1.0` |
| `boolean`   | `true` |
| `$ref`      | resolved and generated recursively (max depth 4) |
| unknown     | `{}` |

A recursion-depth guard (max 4) prevents infinite loops from self-referential
schemas.

## Error responses

| Condition | HTTP status |
|-----------|-------------|
| Path not registered | 404 Not Found |
| Method not defined for path | 405 Method Not Allowed (with `Allow` header) |
| No 2xx response in spec | 200 with `{}` |

## Artificial Latency

Pass `--latency <ms>` to add a uniform sleep before every response.  This is
useful for testing timeout handling and UI loading states.

## Limitations

- **No request validation** — the server accepts any request body regardless
  of the schema.
- **No state** — each request is independent; there is no in-memory database.
  POST / PUT / DELETE do not actually modify anything.
- **No auth enforcement** — security scheme requirements in the spec are
  ignored.  Add a reverse proxy or middleware if auth simulation is needed.
- **Static mock data** — responses are always the same synthetic values.  Use
  a dedicated contract-testing tool (e.g. Prism, WireMock) for dynamic,
  example-driven mocks.
- **No WebSocket / SSE** — only regular HTTP/1.1 is supported.

## Integration with `compose`

`climate mock` works with composed specs.  Generate a composite spec, write it
to a file, and pass it to `mock`:

```bash
# Write the merged spec to a file first
climate compose orders.yaml:/api/orders users.yaml:/api/users \
  --no-build --out-dir /tmp/gateway-src

# Then start a mock server against the merged spec
climate mock /tmp/gateway-spec.json
```

Alternatively, use the spec produced by `compose` directly via a temporary
file or pipe.
