# Contributing to climate

Thanks for your interest in contributing.

## Before you start

- Search existing issues and pull requests to avoid duplicate work.
- For larger changes, open an issue first to discuss approach and scope.

## Development workflow

1. Fork the repository and create a branch from `main`.
2. Make focused, minimal changes.
3. Run local checks:

   ```bash
   go build ./...
   go test ./...
   ```

4. Update docs when behavior or UX changes.
5. Open a pull request using the PR template.

## Pull request expectations

- Keep PRs small and clearly scoped.
- Include tests for behavior changes.
- Explain what changed and why.
- Ensure CI is green.

## Commit style

Use clear, imperative commit messages (for example: `Add security policy`).

## Reporting bugs and requesting features

Use the issue templates to provide all required context.

## Code of Conduct

By participating, you agree to abide by the
[Code of Conduct](./CODE_OF_CONDUCT.md).
