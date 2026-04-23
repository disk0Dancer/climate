# Design: Generated CLI Event Commands (`<cli> events ...`)

## Problem

Generated CLIs can call request/response APIs, but they cannot currently act as
local webhook receivers on their own. That makes event-driven APIs awkward:
users still need an external ad hoc HTTP listener plus a tunneling tool if the
provider must reach a public URL.

## Goals

- Give every generated CLI built-in event commands plus a local config store.
- Let generated CLIs optionally expose the local listener through `cloudflared`.
- Avoid new dependencies in generated projects.
- Keep the runtime generic and configurable, using HMAC instead of
  provider-specific webhook logic.

## Non-goals

- Native provider-specific webhook compatibility layers.
- Managing tunnel accounts, auth tokens, or self-hosted tunnel backends.

## CLI UX

Every generated CLI gets:

```bash
<cli> config list
<cli> config set <key> <value>
<cli> config set --secret events.signing_secret <value>

<cli> events list

<cli> events listen [event-name] \
  [--host 127.0.0.1] \
  [--port 8081] \
  [--path /] \
  [--response-status 202] \
  [--response-body '{"ok":true}'] \
  [--tunnel none|auto|cloudflared] \
  [--signature-mode none|hmac] \
  [--signature-header X-Signature] \
  [--signature-algorithm sha256|sha1|sha512] \
  [--include-timestamp] \
  [--timestamp-header X-Signature-Timestamp]

<cli> events emit <event-name> \
  --target-url <url> \
  [--data-json '{"id":"evt_123"}' | --data-file payload.json] \
  [--signature-mode none|hmac]
```

Behavior:

1. `config ...` manages local CLI defaults and secrets using named
   configurations with one active profile at a time.
2. `events list` prints named event definitions extracted from OpenAPI
   `callbacks` and top-level `webhooks`.
3. `events listen [event-name]` starts a local HTTP server on the requested
   host/port/path and defaults to the selected event's path/methods.
4. Print one JSON startup record with the listen URL.
5. For every received request, print one JSON event record to stdout.
6. If HMAC signing is enabled, verify either `body` or `timestamp + "." + body`
   using the configured header name and algorithm.
7. Reply with the configured status/body.
8. If `--tunnel` is enabled, start a `cloudflared` process and
   print a JSON tunnel record when a public URL is detected.
9. `events emit <event-name>` sends the generated sample payload (or an
   override payload) to a target URL.

## Config keys

The generated CLI stores local defaults in its config file. Useful keys:

- `events.tunnel`
- `events.signature_mode`
- `events.signature_header`
- `events.signature_algorithm`
- `events.include_timestamp`
- `events.timestamp_header`
- `events.signing_secret`

Secret values are masked in `config list`.

Configuration UX is intentionally gcloud-like:

- `config profiles list`
- `config profiles create <name>`
- `config profiles use <name>`

All `config set/get/unset` operations target the active configuration.

## Interactive auth

When the generated CLI includes supported auth schemes, it also exposes:

- `auth login`
- `auth status`
- `auth logout`

`auth login` stores credentials into the active configuration. For supported
OAuth2 flows it can fetch and store an access token interactively; otherwise it
falls back to prompting for the required token/credentials and storing them.

## Tunnel

This version supports only `cloudflared`.

- `--tunnel auto` resolves to `cloudflared`
- command: `cloudflared tunnel --url http://127.0.0.1:<port>`

The listener does not install `cloudflared`. If it is missing from `PATH`, the
command returns an error.

## Event extraction

The generated CLI builds event definitions from:

- top-level OpenAPI `webhooks`
- operation-level `callbacks`

Webhook names become event names directly after normalization unless overridden
by `x-climate-event-name`. Callback names are namespaced with the parent
operation when needed to keep them stable and unique.

Supported generic event metadata extensions:

- `x-climate-event-name`
- `x-climate-event-path`
- `x-climate-signature-mode`
- `x-climate-signature-header`
- `x-climate-signature-algorithm`
- `x-climate-signature-include-timestamp`
- `x-climate-signature-timestamp-header`

## Output shape

The long-running command streams JSON records:

- startup:
  - `type: "listener.started"`
- tunnel discovery:
  - `type: "listener.tunnel"`
- received event:
  - `type: "listener.event"`

This keeps the command scriptable even though it is long-running.

## Follow-up path

After this lands, the next step is richer replay tooling and optional event
delivery fixtures/examples on top of the config-driven HMAC contract.
