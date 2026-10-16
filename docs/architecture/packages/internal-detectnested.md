# internal/detectnested

## Purpose

`internal/detectnested` ports the legacy `detect-nested-claude.sh`. The dispatcher uses its answer to enable the sandbox EPERM fallback and the no-worktree operator mode. This is for a parent process that is itself sandboxed (the nested-sandbox EPERM case of macOS Darwin 25.4 and later).

## Design

- `Detect` returns `nested` when any one of `CLAUDECODE`, `CLAUDE_CODE_ENTRYPOINT` or `CLAUDE_CODE_EXECPATH` is present, else `standalone`.
- `Options.Env` replaces `os.Getenv` for tests; a zero `Options` uses the real environment.

## Invariants

- The tests can pin only the injected-environment paths. The default-environment result depends on where the test runs.
