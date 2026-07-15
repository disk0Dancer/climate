# AGENTS contribution workflow

This repository uses an implementation-first workflow for all feature work.

## Required sequence for any feature

1. **Design first**
   - Describe problem, goals, non-goals, API/CLI UX, and edge cases.
   - Add or update design docs in `docs/`.
2. **Document behavior**
   - Update user-facing docs (`README.md`, `docs/index.md`) when commands or
     capabilities change.
3. **Write tests**
   - Add targeted unit tests for new logic before/with implementation.
4. **Implement code**
   - Keep changes surgical and consistent with existing project style.
5. **Update skills**
   - Update `skills/climate.md` and `skills/climate-generator/SKILL.md` when
     command set or workflows change.
6. **Validate locally**
   - Run `make verify` (fmt-check + vet + build + lint + `go test -race`).
     This mirrors the CI pipeline and is mandatory before every push.
   - Run targeted tests during development for faster feedback.
7. **Validate CI health**
   - Ensure PR checks are green before merge.
   - After merge, confirm CI on `main` is green:
     `gh run list --branch main --workflow CI --limit 1`.
   - A task is not done until post-merge CI on `main` is green. If it is red,
     fixing it is part of the same task — do not stop or hand off.
8. **Commit discipline**
   - Small, meaningful commits with clear messages.
9. **Push and PR hygiene**
   - Push branch updates, keep PR description/checklist current, and respond to
     review comments with the commit hash that addresses each request.

## Quality rules

- Do not remove or weaken unrelated tests.
- Tests must be hermetic: they must not depend on the developer's machine or
  global state. Tests that shell out to external tools (e.g. `git`) must set
  the environment explicitly (`t.Setenv` for `GIT_AUTHOR_*`/`GIT_COMMITTER_*`,
  temp `HOME`, etc.) so they pass on a bare CI runner. "Passes on my machine"
  is not evidence.
- Do not introduce breaking CLI changes without docs + migration notes.
- Prefer deterministic behavior (sorted output, stable iteration).
- Keep generated/manifest behavior backward compatible where practical.

## Feature checklist template

- [ ] Design doc added/updated
- [ ] README/docs updated
- [ ] Skills updated
- [ ] Tests added/updated
- [ ] `make verify` passes locally
- [ ] PR CI checks green
- [ ] Post-merge CI on `main` green (`gh run list --branch main --workflow CI --limit 1`)
