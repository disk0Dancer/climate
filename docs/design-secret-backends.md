# Design: Encrypted Secret Backends for Generated CLIs (gopass by default)

## Problem

Generated CLIs store secrets set via `config set --secret` (including
`auth.bearer_token` written by `auth login`) as plaintext strings in the
`secrets` map of `<UserConfigDir>/<cli-name>/config.json`. The `--secret` flag
only affects masking in `config list` output. The only protection is file
permissions (`0600` file, `0700` dir).

Users who already run a password manager (gopass) reasonably expect secrets to
land there, not on disk in plaintext. Users without one should still get
encryption at rest by default, with zero extra ceremony.

## Goals

- Secrets from generated CLIs are encrypted at rest by default.
- If `gopass` is available and initialized on the machine, use it as the
  backend automatically — no flags, no configuration.
- If gopass is absent, transparently bootstrap a local encrypted store so the
  default is still "encrypted", not "plaintext".
- Keep the generated-CLI UX unchanged: `config set --secret`, `config get`,
  `auth login`, `auth status`, `auth logout` behave exactly as today. The
  backend is an implementation detail the user does not have to know about.
- Migrate existing plaintext secrets automatically and silently on first write
  (and opportunistically on first read).
- Keep generated CLIs testable and CI-friendly (hermetic fallback backend, no
  interactive prompts in non-TTY environments).

## Non-goals

- Managing the user's existing gopass store layout, mounts, or sync/git setup.
- Secrets for `climate` itself (`--github-token` for publish stays as-is; can
  adopt the same backend later).
- OS-keychain (macOS Keychain / secret-service) backends — possible follow-up,
  not part of this design.
- Multi-user or team secret sharing.

## Architecture

### `SecretStore` interface in generated CLIs

A new generated package `internal/secrets` defines:

```go
type Store interface {
    Get(profile, key string) (string, bool, error)
    Set(profile, key, value string) error
    Unset(profile, key string) (bool, error)
    List(profile string) ([]string, error) // keys only, never values
    Name() string                          // backend id for diagnostics
}
```

`internal/config.Store` keeps owning non-secret properties, profiles, and the
active-profile pointer in `config.json`. Its secret paths (`Set(..., true)`,
`Get`, `Unset`, `MaskedEntries`) delegate to the resolved `secrets.Store`.
`config.json` keeps only a marker per secret key (`"secrets": {"auth.bearer_token": "ref"}`)
so listing and existence checks stay cheap and the file documents which keys
live in the backend.

### Backend resolution order

Resolved once per process, in order:

1. **Env override** — `<CLINAME>_SECRETS_BACKEND=gopass|file|plaintext`
   (uppercased CLI name prefix, same convention as other generated env vars).
   `plaintext` is the legacy behavior, kept as an escape hatch and for CI.
2. **gopass** — if a `gopass` binary is on `PATH` and `gopass ls --flat`
   succeeds (store initialized), use it.
3. **Encrypted file fallback** — otherwise use the built-in encrypted local
   store (below).

The chosen backend is recorded in `config.json` (`"secrets_backend": "gopass"`)
on first use so the decision is stable; a machine that later gains gopass does
not silently split secrets across two backends. `auth status` and
`config list` keep working unchanged; backend name appears only in
`config list` JSON output as `secrets_backend` for debuggability — no extra
user-facing messaging.

### gopass backend

- Secret path convention: `climate/<cli-name>/<profile>/<key>`
  (e.g. `climate/openai/default/auth.bearer_token`).
- Implemented by shelling out to the binary: `gopass show -o <path>`,
  `gopass insert -f <path>` (value via stdin), `gopass rm -f <path>`,
  `gopass ls --flat climate/<cli-name>/<profile>`.
- Shelling out (vs. the `gopass/pkg/gopass/api` Go module) is deliberate:
  it respects the user's full gopass config, mounts, GPG agent, and avoids
  pulling gopass's heavy dependency tree into every generated CLI.
- Failures (locked GPG agent, store error) surface as normal command errors;
  the CLI never falls back to plaintext silently.

### Encrypted file fallback (no gopass on the machine)

Bootstrapping a full gopass+GPG install on a clean machine is not something a
generated CLI should do implicitly (package-manager side effects, GPG key
generation UX). Instead the fallback ships encryption in-process:

- `filippo.io/age` encrypts a JSON secrets blob at
  `<UserConfigDir>/<cli-name>/secrets.age`.
- The age identity (private key) is generated on first use and stored at
  `<UserConfigDir>/<cli-name>/identity.age` with `0600` — same trust model as
  an SSH private key. This keeps secrets safe against backup/sync leaks of
  `config.json` and requires no interaction.
- The layout is gopass-compatible in spirit: if the user later installs
  gopass, `climate secrets migrate` (on the generator side, below) re-inserts
  file-backend secrets into gopass and removes the local blob.

Rejected alternative: having climate auto-install gopass (brew/apt) — too
invasive for a generated CLI, breaks in CI/containers, and gopass without an
existing GPG/age setup still needs interactive `gopass setup`.

### Migration of existing plaintext secrets

On first `Load()` with a resolved encrypted backend, if `config.json` contains
non-marker plaintext values in any profile's `secrets` map:

1. write each to the backend,
2. replace values with the `"ref"` marker,
3. save `config.json`.

Read paths do the same lazily if a write never happens first. No prompts, no
output — the contract ("`--secret` values are stored securely") is finally
true, which is not a behavior change worth announcing per invocation.

## Changes in the climate generator

- New template `internal_secrets.go.tmpl` (interface + gopass + age file
  backends + resolution).
- `internal_config.go.tmpl`: delegate secret operations, add
  `secrets_backend` field, migration hook in `Load()`.
- `auth.go.tmpl` / `config.go.tmpl`: no UX changes; error paths now can return
  backend errors.
- `go.mod` template gains `filippo.io/age`.
- `climate upgrade <cli>` regenerates with the new store; migration then runs
  on the upgraded CLI's first invocation.
- New generated command (hidden from prominent help):
  `<cli> config secrets migrate --to gopass|file` for explicit backend moves.

## Testing

- Unit tests for the age file backend (round-trip, identity bootstrap,
  permissions) — hermetic, no gopass needed.
- gopass backend tests behind a fake `gopass` shim script on `PATH`
  (hermetic; also covers "binary present but store uninitialized" → fallback).
- Migration test: seed a legacy plaintext `config.json`, load, assert
  plaintext gone and values readable via the backend.
- E2E (existing generate-and-build tests): generated CLI builds and
  `config set --secret` / `config get` round-trips with
  `<CLINAME>_SECRETS_BACKEND=file` under a temp `HOME`.

## Rollout

1. Land templates + generator changes behind the resolution logic (default on;
   `plaintext` env escape hatch documented in generated `--help` for
   `config set`).
2. `climate upgrade` note in release changelog.
3. Follow-up candidates: OS keychain backend, gopass API mode, using the same
   store for `climate publish` GitHub tokens.
