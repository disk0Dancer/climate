# OpenAPI 3.0 support matrix (current + planned)

This document summarizes what `climate` already supports when generating CLIs,
what is partially supported, and what should be designed/implemented next.

## Scope

- OpenAPI version: 3.x (3.0 / 3.1 input accepted by parser/validator)
- Main commands affected: `generate`, `compose`, `mock`, `skill generate`

## Matrix

| OpenAPI feature | Status | Current behavior | Next step |
|---|---|---|---|
| `info`, `paths`, HTTP methods | ✅ Implemented | Used as core input for command tree generation | Keep stable |
| Path/query/header parameters | ✅ Implemented | Converted to CLI flags | Add richer validations from schema constraints |
| Request body (`application/json`) | ✅ Implemented | Supported via generated payload flags | Add optional strict schema validation before send |
| Response handling | ✅ Implemented | Structured output + generated error envelope | Add per-status typed formatting hooks |
| Auth: API key / bearer / basic / OAuth2 | ✅ Implemented | Mapped to generated auth flags/env vars | Add multi-scheme policy docs |
| Tags → command groups | ✅ Implemented | Tag-based group hierarchy | Add optional custom grouping strategies |
| `components.schemas` + `$ref` | ✅ Implemented | Used in generator and mock response synthesis | Improve support for schema combiners |
| Multi-spec composition (`compose`) | ✅ Implemented | Path prefixing + component namespacing + merge | Add callbacks/webhooks merge policy |
| Local mock simulator (`mock`) | ✅ Implemented | Auto responses from spec schema + latency | Add optional examples-first mode |
| `enum` | ✅ Implemented (mock) / ⚠️ partial (CLI) | Mock prefers first enum value | Add flag-level enum validation/help text |
| `allOf`, `oneOf`, `anyOf`, `not` | ⚠️ Partial | Core flow works for simple schemas; advanced combiners not fully synthesized | Add schema normalizer for combiners |
| `servers` and server variables | ✅ Implemented | Generated CLIs use the primary server URL and support server-template interpolation via `--server-var-<name>` and `<CLI>_SERVER_VAR_<NAME>` env vars | Keep stable |
| `callbacks` | ✅ Implemented / ⚠️ Partial | Generated CLIs expose callback-derived named event commands via `events list`, `events listen <name>`, and `events emit <name>` | Improve callback expression/path inference and richer config-driven defaults |
| `webhooks` (3.1) | ✅ Implemented / ⚠️ Partial | Top-level `webhooks` become named generated event commands with local listener + emit flow | Add richer schema-aware event metadata and replay tooling |
| Links | ❌ Planned | Ignored | Add optional “follow-up command hint” output |
| Examples (`example` / `examples`) | ⚠️ Partial | Not consistently preferred in generation | Use examples as first-class sample payload/response source |

## Webhooks and event APIs — proposed behavior

Some APIs are event-driven and include webhooks/callbacks instead of (or in
addition to) plain request/response endpoints.

Current baseline for generated CLIs:

1. **Named event surface**
   - `myapi events list`
   - `myapi events listen <event-name>`
   - `myapi events emit <event-name> --target-url ...`
2. **Local config store**
   - `myapi config list`
   - `myapi config set`
   - `myapi config set --secret events.signing_secret ...`
3. **Optional tunnel exposure**
   - `--tunnel auto|cloudflared`
4. **Generic HMAC signatures**
   - configurable header, algorithm, and optional timestamp signing
5. **Structured event stream**
   - startup, tunnel, and received-event records are streamed as JSON

Proposed next direction for generated CLIs:

1. **Add richer event metadata to OpenAPI extensions**
   - signature defaults, path overrides, and replay hints
2. **Support production replay/import**
   - `myapi events import --file payload.json --event <name>`
   - `myapi events replay --source prod-export.ndjson`
3. **Compose awareness**
   - In `compose`, namespace event names with prefix (same as path/components)
4. **Mock integration**
   - `climate mock` can emit synthetic webhook payloads at intervals or on
     demand for integration tests

## Pagination — proposed behavior

Pagination should be generated as a first-class UX pattern, regardless of API
style:

- Page/pageSize style (`page`, `size`, `limit`, `offset`)
- Cursor style (`cursor`, `next`, `before`, `after`)
- Token style (`nextPageToken`)
- Link-header style (`Link: <...>; rel="next"`)

Proposed CLI conventions:

- `--all` to auto-fetch all pages safely
- `--max-items <n>` as guardrail
- `--page-size <n>` override when supported
- `--starting-token <token>` for resume
- `--pagination-debug` to print paging metadata

Safety defaults:

- `--all` should require confirmation for very large transfers unless
  `--yes` is set
- bounded retries/backoff for transient failures

## Prioritized implementation roadmap

1. Pagination abstraction + generated paging flags (`--all`, `--max-items`)
2. config-driven secret and signature UX hardening
3. production replay/import workflow for named events
4. examples-first generation mode (payloads + mock responses)
5. advanced schema combiner normalization (`allOf`/`oneOf`/`anyOf`)
