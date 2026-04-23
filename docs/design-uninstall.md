# Design: Lifecycle Uninstall (`climate uninstall`)

## Problem

`climate` can remove generated CLIs via `climate remove`, and it can remove its
own shell completion wiring via `climate completion uninstall`, but it has no
first-class workflow for uninstalling the `climate` binary itself. It also
deletes generated CLIs without any interactive confirmation.

That leaves two UX gaps:

- deleting a generated CLI is too easy to do by accident
- removing `climate` itself depends on how it was installed and should not be a
  manual memory exercise for the user

## Goals

- Make generated-CLI removal interactive by default.
- Add a root-level `climate uninstall` command for removing the `climate`
  executable itself.
- Detect the installation method and use the correct removal strategy.
- Support a stricter full uninstall mode that also removes climate-managed local
  artifacts.
- Keep automation possible with `--yes`.

## Non-goals

- Removing generated GitHub repositories created by `climate publish`.
- Cleaning package-manager caches such as Homebrew downloads or the Go module
  cache.
- Deleting arbitrary user files outside climate-managed paths or manifest-owned
  generated CLI paths.

## CLI UX

```bash
climate remove [--purge-sources] [--yes] <cli-name>

climate uninstall [--full] [--yes]
```

### `climate remove`

- prompts before deleting a generated CLI
- `--purge-sources` keeps the existing meaning
- `--yes` skips the prompt for automation

### `climate uninstall`

Removes the `climate` executable itself.

- default mode removes only the `climate` CLI
- `--full` additionally removes climate-managed local artifacts:
  - generated CLIs recorded in the manifest
  - generated source directories recorded in the manifest
  - the manifest file
  - climate-managed completion scripts and config blocks
- `--yes` skips the prompt

## Installation-method detection

The uninstall flow derives the installation method from the resolved executable
path:

- **Homebrew**: resolved path contains `/Cellar/climate/`
- **Go install**: executable lives in `GOBIN`, `GOPATH/bin`, or `~/go/bin`
- **Standalone**: any other path, including manually moved release binaries

## Removal strategy

### Homebrew

Run:

```bash
brew uninstall climate
```

This keeps removal aligned with the package manager that owns the binary.

### Go install

Delete the installed executable directly from the Go bin directory.

### Standalone binary

Delete the resolved executable path directly.

## Confirmation model

Both destructive flows prompt with a `y/N` confirmation unless `--yes` is
present.

- `remove` confirms the target generated CLI and whether sources will be purged
- `uninstall` confirms the detected installation method and whether full cleanup
  will also remove generated CLIs and climate-managed local state

Rejected approach:

- two-step "plan then confirm with exact typed phrase" flow
  - rejected because the commands are already explicit destructive entry points,
    and a normal `y/N` prompt is enough friction without making CLI use clumsy

## Safety rules

- Full uninstall removes only assets owned by the manifest and known climate
  completion paths.
- Shell config cleanup removes only climate-managed marker blocks.
- Empty climate-owned directories may be pruned after managed files are removed.
- If the user cancels at the prompt, the command exits 0 and reports a
  cancellation payload instead of deleting anything.
