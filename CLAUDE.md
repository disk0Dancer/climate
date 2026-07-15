# climate — Claude Code instructions

Follow `AGENTS.md` (implementation-first workflow). Key rules for this repo:

## Definition of Done (non-negotiable)

A task is **not done** until:
1. `make verify` passes locally (fmt-check + vet + build + lint + `go test -race`).
2. PR CI checks are green.
3. **Post-merge CI on `main` is green** — check with:
   `gh run list --branch main --workflow CI --limit 1`
   If it is red after your merge, fixing it is part of the same task. Do not stop.

## Hermetic tests

Tests must pass on a bare Linux CI runner, not just this Mac:
- Any test that shells out to `git` must set identity via `t.Setenv`
  (`GIT_AUTHOR_NAME/EMAIL`, `GIT_COMMITTER_NAME/EMAIL`) and pin branches
  (`git init --bare --initial-branch=main`) — macOS masks both failure modes.
- Reproduce CI locally before pushing:
  `env HOME=$(mktemp -d) GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null go test -race ./...`

## Debugging red CI

Use `gh`: `gh run list --workflow CI`, `gh run view <id> --log-failed`.
`CI Auto-Fix` workflow only fixes gofmt/lint — test failures need a human/agent.
