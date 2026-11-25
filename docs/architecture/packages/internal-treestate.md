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
- **Error text is stable.** The ship wrapper re-derives the message strings of `RunError`, so the text stays byte-identical after the extraction. `RunError` carries the revisions that it ran and shows `git diff <revisions>`. `SHA` runs `HEAD`, so its text is the same as before, and a failed `Between` names its own revisions. Pinned by `TestRunError_MessagesAndUnwrap`.
- **Between shares the one argument builder.** `Between(from, to)` runs `git diff <from> <to>` through the same helper as `SHA`. It adds no flag, so no existing digest changes. The digest of a committed change equals the digest that the gate took at its staged state. Pinned by `TestChangeDigest_WorktreeChangeEqualsCommitted`, `TestChangeDigest_AnUnstagedEditAtGateTimeDoesNotMatch` and `TestBetween_ReportsAFailedRun`, `TestBetween_AFailedRunNamesTheGitMessage` and `TestSHA_AFailedRunNamesHEAD`.

## Pinned digest

The pinned digest is the new home of the change digest. `SHA` and `Between` stay frozen as the readers of bindings that exist before the change. The plan is [change-digest-has-one-pinned-home-2026-11.md](../../plans/change-digest-has-one-pinned-home-2026-11.md).

- **One argument builder.** `DiffArgs(revs...)` returns the argv of `git` without the program name. It pins `core.quotePath`, `diff.suppressBlankEmpty` and `core.bigFileThreshold` through `-c`. Flags pin the prefixes, the algorithm, the context and the renames. Flags also pin the colour, the external and textconv drivers, the order file and the submodule form.
- **Presentation inputs only.** A content input such as `core.autocrlf` or a `text` attribute must reach git the same way on the gate path and on the commit path. A pin of `core.attributesFile` makes the worktree digest differ from the commit digest, so the set has none. `TestPinned_WorktreeChangeEqualsPinnedBetween` pins this.
- **`--ignore-submodules=dirty`.** With `none`, a worktree diff prints `-dirty` for a dirty submodule and a commit diff never does. `TestPinned_DirtySubmoduleDoesNotSeparateWorktreeFromCommit` pins the choice.
- **No `--binary`.** The bytes of a binary patch depend on `core.compression`. `--full-index` still separates two binary contents. `TestPinned_BinaryContentChangesTheDigest` pins both facts.
- **Hunk header strip.** `Pinned` removes the text after the second `@@` of each hunk header before it hashes. A host `diff=<driver>` sets that function context, and a flag cannot pin it.
- **Env scrub.** `Pinned` removes `GIT_DIFF_OPTS` from the child env, because it takes precedence over `-U3`. A nil env means `os.Environ()`. `GIT_EXTERNAL_DIFF` needs no scrub, because `--no-ext-diff` neutralizes it.
- **Evidence.** `TestPinned_HostileConfigMatrix` has one row for each input that a flag or the scrub pins. Each row also proves that a plain digest moves under its config, so a row cannot pass without probing its input. The build removed each flag in turn, and each removal turned a row red.
- **Real argv in errors.** A failed pinned run sets `RunError.Args`, and `Error()` prints `git` with those arguments. A failed `SHA` or `Between` keeps the old text. `TestRunError_PrintsTheRealArgv` pins this.
- **Rule.** `internal/repocontract` fails on any other production code that builds a `git diff` for a digest, or calls `DiffArgs`. See [internal-repocontract.md](internal-repocontract.md).
