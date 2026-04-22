# Contributing to climate

Thanks for your interest in contributing.

## Before you start

- Search existing issues and pull requests to avoid duplicate work.
- For larger changes, open an issue first to discuss approach and scope.

## Required workflow for feature work

For feature changes, follow this sequence:

1. **Design first**
   - Describe problem, goals, non-goals, API/CLI UX, and edge cases.
   - Add or update design docs in `docs/`.
2. **Document behavior**
   - Update user-facing docs (`README.md`, `docs/index.md`) when commands or capabilities change.
3. **Write tests**
   - Add targeted unit tests for new logic before/with implementation.
4. **Implement code**
   - Keep changes focused and consistent with project style.
5. **Update skills**
   - Update `skills/climate.md` and `skills/climate-generator/SKILL.md` when command set or workflows change.
6. **Validate locally**
   - Run:

   ```bash
   go build ./...
   go test ./...
   ```

7. **Validate CI health**
   - Ensure PR checks are green before merge.

## Pull request expectations

- Keep PRs small and clearly scoped.
- Explain what changed and why.
- Include tests for behavior changes.
- Update docs and skills files when required.
- Ensure CI is green.

## Commit style

Use clear, imperative commit messages (for example: `Add security policy`).

## Reporting bugs and requesting features

Use the issue templates to provide all required context.

## Code of Conduct

By participating, you agree to abide by the
[Code of Conduct](./CODE_OF_CONDUCT.md).
