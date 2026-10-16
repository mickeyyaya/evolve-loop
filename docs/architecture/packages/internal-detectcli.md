# internal/detectcli

## Purpose

`internal/detectcli` ports the legacy `detect-cli.sh`. It names the CLI that is driving the skill, for platform-detect and the overlay selection.

## Design

- **Probe order.** In order: `Options.Platform` (the operator override and DI seam), `CLAUDE_CODE_*` (claude), `GEMINI_CLI` or `GEMINI_API_KEY` (gemini), `CODEX_HOME` or `CODEX_API_KEY` (codex), `agy` on PATH (antigravity), else unknown.
- **Seams.** `Options.Env` (default `os.Getenv`) and `Options.LookPath` (default `exec.LookPath`) must be non-nil when supplied. A zero `Options` uses the production defaults. A non-empty `Platform` is returned without consulting env or PATH.

## Invariants

- **`Canonical` is the single authority for CLI-name aliases.** Detection and some profiles say `antigravity`, but the adapter, the capability manifest and the PATH binary are all `agy`. Dispatch used to carry three inline copies of that rewrite (in `subagent`), three places to drift. `Canonical` owns it, rewrites that one alias, and returns every other name unchanged.
- **The empty string passes through.** The callers' `cli unresolved` guards must keep firing on an empty CLI. Pinned by `TestCanonical_AliasTable`, which also pins that the match is case-sensitive: only the exact alias moves.
