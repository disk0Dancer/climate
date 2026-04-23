# Design: Shell Completions (`climate completion`)

## Problem

`climate` already exposes a growing command surface through Cobra, but it does
not ship an ergonomic way to enable shell completion locally. Users can still
wire completions manually by writing shell-specific boilerplate themselves, but
that is friction for both humans and agents.

## Goals

- Generate completion scripts for supported shells directly from the CLI.
- Provide a one-command local install flow that writes the completion script and
  wires it into the user's shell startup config when needed.
- Provide a matching uninstall flow that only removes climate-managed wiring.
- Keep the behavior deterministic and idempotent.

## Non-goals

- Managing completions for generated CLIs built by `climate generate`.
- Detecting or modifying every possible shell startup file variant.
- Prompting interactively before file edits; the install and uninstall commands
  are the explicit opt-in surface.

## CLI UX

```bash
climate completion bash
climate completion zsh
climate completion fish
climate completion powershell

climate completion install [--shell bash|zsh|fish|powershell]
climate completion uninstall [--shell bash|zsh|fish|powershell]
```

Generation commands print the raw completion script to stdout.

Install and uninstall commands:

- accept `--shell`; when omitted, detect the current shell from `$SHELL`
- write a managed completion script under a climate-owned location
- add or remove a climate-managed block in the relevant shell config
- print structured JSON describing which files were touched

## File layout

Managed script locations:

- Bash: `~/.climate/completions/climate.bash`
- Zsh: `~/.climate/completions/climate.zsh`
- Fish: `~/.config/fish/completions/climate.fish`
- PowerShell: `~/.climate/completions/climate.ps1`

Managed config targets:

- Bash: prefer `~/.bashrc`; on macOS use `~/.bash_profile` when `~/.bashrc`
  does not exist
- Zsh: `~/.zshrc`
- Fish: no config edit required because Fish autoloads files from its
  completions directory
- PowerShell:
  - macOS/Linux: `~/.config/powershell/Microsoft.PowerShell_profile.ps1`
  - Windows: `~/Documents/PowerShell/Microsoft.PowerShell_profile.ps1`

## Managed block contract

For shells that need config wiring, `climate completion install` appends or
updates a marker-bounded block:

```text
# >>> climate completion >>>
...
# <<< climate completion <<<
```

`uninstall` removes only that managed block and the managed script file. It
does not rewrite unrelated user configuration.

## Edge cases

- If shell auto-detection fails, the command returns an error asking for
  `--shell`.
- Re-running install rewrites the managed script and keeps a single managed
  config block.
- Re-running uninstall is safe: missing files are treated as already removed.
- `climate completion <shell>` prints a tip on stderr suggesting
  `climate completion install --shell <shell>` so users discover the managed
  install path without corrupting stdout.
