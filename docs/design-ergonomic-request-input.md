# Design: Ergonomic Request Input for Generated CLIs

## Problem

Operations with a request body accept it only as `--data-json '<blob>'` or
`--data-file <path>`. That means hand-writing escaped JSON on every call,
repeating invariant fields (like `model`) each time, and receiving the full
response JSON even when the caller needs one field. This is unpleasant for
humans and token-expensive for AI agents driving the CLI — the audiences the
generated skill prompts explicitly target.

## Goals

- Compose request bodies from compact command-line arguments without writing
  JSON by hand (httpie-style `field=value` syntax).
- Let invariant body fields be configured once as per-operation defaults and
  omitted from calls.
- Let callers reduce response output to just the fields they need.
- Accept large text values from stdin to avoid shell quoting entirely.
- Serve humans and AI agents with the same syntax; keep `--data-json` /
  `--data-file` working unchanged as the low-level path.
- Teach `climate skill generate` to document the compact forms so agents
  learn the cheapest syntax first.

## Non-goals (follow-ups, in priority order)

- Chat-style interactive REPL for conversation-shaped APIs.
- User-defined aliases/recipes (`<cli> alias set ask '...'`).
- TUI form wizard (`-i`) generated from the schema.
These build on the body-composition engine introduced here and land
separately.

## Body composition syntax (httpie-style)

Positional `key=value` arguments after the operation name compose the JSON
body:

| Form | Meaning |
| --- | --- |
| `field=text` | string value |
| `field:=raw` | raw JSON: number, bool, array, object (`temp:=0.2`, `stream:=true`, `messages:='[{...}]'`) |
| `a.b=x` | nested object `{"a":{"b":"x"}}` |
| `items[0].name=x` | array element |
| `field=@path` | string value read from file |
| `field=@-` | string value read from stdin |

Rules:

- Later arguments override earlier ones; deep-merge for objects.
- Precedence: `--data-json`/`--data-file` provide the base document (if
  given), then configured defaults are merged under it, then `key=value`
  arguments are merged on top. So param args always win, and the low-level
  flags stay a complete escape hatch.
- Values are validated against the operation's request-body schema when the
  schema is known: unknown top-level fields and type mismatches fail with a
  message listing valid fields (agents self-correct from this).
- `=`-in-value needs no escaping (split on first `=` / `:=`).

Example, before and after:

```bash
# before
openai chat create-chat-completion --data-json '{"model":"gpt-5-nano","messages":[{"role":"user","content":"привет"}]}'

# after
openai chat create-chat-completion model=gpt-5-nano messages:='[{"role":"user","content":"привет"}]'

# with a configured default model and stdin content (for an operation that has a top-level string field)
echo "объясни X" | openai <tag> <operation> prompt=@-
```

(`@-` inside `:=` raw JSON is NOT expanded — only `field=@-` string form reads
stdin; the example above therefore uses the string form where applicable.)

## Per-operation defaults

New config namespace `defaults.<tag>.<operation>.<field>` plus a tag-level
fallback `defaults.<tag>.<field>`:

```bash
openai config set defaults.chat.model gpt-5-nano
```

Defaults are merged into the body below explicit arguments (see precedence
above). `config list` shows them like any other property. Defaults apply only
to fields that exist in the operation's request-body schema; others are
ignored with a warning to stderr.

## Response shaping

Two flags on every generated operation command:

- `--jq <expr>` — apply a jq expression to the response body before output.
  Implemented with `github.com/itchyny/gojq` (pure Go, no external binary).
- `--pick <path>` — sugar for the common case: dot/bracket path
  (`--pick choices[0].message.content`). Compiled to the equivalent gojq
  query internally.

Both compose with `--output`: the filtered value is printed raw when it is a
string (no JSON quotes — this is the agent-friendly mode), JSON otherwise.

## Skill prompt updates

`climate skill generate` output documents, per operation: the `key=value`
syntax with one concrete example, available defaults keys, and `--pick`
usage — compact form first, `--data-json` mentioned only as fallback.

## Implementation notes

- New generated package `internal/body`: parser for `key=value` /
  `key:=json` / paths / `@file` / `@-`, deep-merge, and schema validation
  hooks. Unit-tested heavily; this is the engine later features (REPL,
  aliases, TUI) reuse.
- Templates: `commands.go.tmpl` (operation commands gain `Args` handling and
  the two output flags), `root.go.tmpl` (shared helpers), skill templates.
- Generated `go.mod` gains `github.com/itchyny/gojq`.
- Body schema is already available to the generator per operation; embed the
  minimal field/type map needed for validation and defaults filtering into
  the generated code (not the whole spec).

## Testing

- Unit: parser (all syntax forms, merge precedence, error messages), pick →
  gojq compilation, defaults merging and filtering.
- Generator e2e: generated CLI builds; a round-trip against `climate mock`
  composing a body from params + defaults + stdin, asserting the mock
  received the expected JSON and `--pick` printed the expected field.
- Hermetic per repo rules (temp HOME, no network beyond the local mock).
