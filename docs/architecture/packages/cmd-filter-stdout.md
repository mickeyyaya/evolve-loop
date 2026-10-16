# cmd/filter-stdout

## Purpose

`cmd/filter-stdout` is an operator utility. Given a workspace and a phase, it writes the `<phase>-stdout.clean.txt` companion of that phase's raw log. It runs the same `logfilter.Process` that the runner uses, so an operator can clean a workspace after the fact.

## Design

- **Exit codes.** `run` returns 2 for a usage error (anything but exactly two arguments), 1 when `logfilter.Process` fails, and 0 on success.
- **`run` is the seam.** `main` only calls `os.Exit(run(os.Args[1:]))`, so the exit codes are testable without exiting the test process.

## Invariants

- **Exactly two arguments.** Extra arguments are a usage error, not ignored. `TestRun_ThreeArgs_ExitsWithCode2` pins this.
- **A filter failure exits 1.** `TestRun_ProcessError_ExitsWithCode1` makes the workspace read-only so the temp file cannot be created, and checks the exit code. The test restores write permission in a cleanup registered after `t.TempDir()`, so the removal succeeds.
- **The happy path writes the clean file.** `TestRun_HappyPath_WritesCleanFile` pins the output file name.

## Findings

- None.
