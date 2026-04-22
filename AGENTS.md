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
   - Run:
     - `go build ./...`
     - `go test ./...`
   - Run targeted tests during development for faster feedback.
7. **Validate CI health**
   - Ensure PR checks are green before merge.
8. **Commit discipline**
   - Small, meaningful commits with clear messages.
9. **Push and PR hygiene**
   - Push branch updates, keep PR description/checklist current, and respond to
     review comments with the commit hash that addresses each request.

## Quality rules

- Do not remove or weaken unrelated tests.
- Do not introduce breaking CLI changes without docs + migration notes.
- Prefer deterministic behavior (sorted output, stable iteration).
- Keep generated/manifest behavior backward compatible where practical.

## Feature checklist template

- [ ] Design doc added/updated
- [ ] README/docs updated
- [ ] Skills updated
- [ ] Tests added/updated
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes
- [ ] CI checks green
