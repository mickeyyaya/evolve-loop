# internal/treestate

## Purpose

`internal/treestate` computes the tree-state SHA, `sha256(git diff HEAD)`. The SHA binds a reviewed or audited change set to the exact tree that is committed.

## Design

- **Single source of truth.** The commit-gate attestation reader and the audit-binding verifier (both in `internal/phases/ship`) call `SHA`, so the writer and the reader hash byte for byte the same.
- **Git behind a seam.** `SHA` takes a `sysexec.RunFunc`. Production passes `sysexec.DefaultRunner`, and tests inject a fake. `dir` is the git working directory, and `""` inherits the working directory of the caller. `env` passes straight through, and `nil` inherits the parent.
- **Typed failures.** `RunError` has exactly two shapes. `Err != nil` with `ExitCode` -1 is an unrecoverable runner failure (binary missing, context cancelled). `Err == nil` with `ExitCode > 1` is a fatal git exit such as 128. Callers map them to their own error codes. `Unwrap` exposes the runner error and gives `nil` for a fatal git exit.

## Invariants

- **Exit code 1 is not an error.** `git diff HEAD` exits 1 when differences exist. This was the semantics of the bash Auditor, and the port keeps it. `SHA` hashes the diff for exit 0 and 1. Only exit above 1 is fatal. Pinned by the exit-code table of `treestate_test.go`.
- **A clean tree hashes the empty string.**
- **Go equals the shell.** The Go fingerprint equals `git diff HEAD | shasum -a 256` over a real working tree. The commit-gate and audit binding depend on this. Pinned by `TestSHA_RealGitParity`.
- **Error text is stable.** The ship wrapper re-derives the message strings of `RunError`, so the text stays byte-identical after the extraction. Pinned by `TestRunError_MessagesAndUnwrap`.
