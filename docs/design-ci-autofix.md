# CI Auto-Fix Design

## Problem

The primary CI workflow should remain deterministic and reviewable, but simple
formatting and auto-fixable lint failures are still noisy and time-consuming to
clean up by hand.

## Goals

- Keep `CI` as a check-only workflow.
- Run a separate workflow automatically after a failed `CI` run.
- Apply `gofmt` and `golangci-lint --fix` on same-repository branches.
- Push fixes back to the branch and explicitly re-dispatch `CI` on the updated
  branch head.
- Reuse the same lifecycle behavior for repositories bootstrapped by
  `climate publish`.

## Non-Goals

- Auto-fixing test, build, or `go vet` failures.
- Pushing changes to forked pull request branches.
- Rewriting user-managed workflow files that do not carry the climate marker.

## Workflow Shape

1. `CI` runs on `push`, `pull_request`, and `workflow_dispatch`.
2. `CI Auto-Fix` listens for failed `CI` completions via `workflow_run`.
3. The auto-fix job only runs when the failing branch belongs to the same
   repository and the branch head still matches the SHA that failed.
4. The job runs `gofmt -w .` and `golangci-lint --fix`.
5. If changes were produced, the workflow commits and pushes them, then calls
   the Actions dispatch API to run `CI` again on that branch.

## Edge Cases

- If the branch advanced after the failing `CI` run, the auto-fix job exits
  without writing to a stale branch state.
- If the failure is not auto-fixable, the workflow exits cleanly with no commit.
- A push created with `GITHUB_TOKEN` does not trigger `push` workflows, so the
  explicit re-dispatch step is required for the fixed commit to get a fresh
  `CI` run.
