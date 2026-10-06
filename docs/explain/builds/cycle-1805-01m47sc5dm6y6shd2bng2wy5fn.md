# Build Explanation — Cycle 1805

## Build Binding
- Cycle: 1805
- Base SHA: 992007a9e4d453d17f02132c5d9372cc55854d6d

## Summary
This build adds `evolve scan secrets [--staged | --diff <ref>] [--project-root P]`, the first production caller of `secretleakscan.ScanDiff`. The verb scans the staged diff (or a git range), prints each finding as `<file>:<line>: <rule>: <masked match>` and a `scan secrets: PASS|FAIL` summary, and exits 0 clean, 1 on a finding, 2 on a git failure and 10 on a usage error. `secretleakscan.Finding` gains `File` and `Line`, which ScanDiff fills from the diff's `+++` and `@@` headers. The /commit skill now runs the scan first in step 4.

## Rationale
The secret-leak-scan phase only ran inside a cycle, so a console change reached `evolve ship --class manual` with no deterministic secret check. Reusing the existing detector keeps one rule set for both paths. File and line come from ScanDiff itself instead of a second diff walker in `cmd/evolve`, so detection and location can never disagree. The repo is the cwd or `--project-root`, never `EVOLVE_PROJECT_ROOT`, because scanning the runtime plane instead of the operator's worktree would print a false PASS.

## Changed Areas
- `go/internal/phases/secretleakscan/secretleakscan.go` — `Finding` gains `File string` and `Line int`; ScanDiff tracks a small `location` value (`advance`, `step`, `headerPath`, `hunkNewStart`) so each finding carries the post-image path and 1-based line, while the (Rule, Match) sequence stays exactly as before.
- `go/internal/phases/secretleakscan/secretleakscan_test.go` — adds a table test for every header shape: count left out, removed and no-newline lines, trailing-TAB and quoted paths, missing or garbage hunk headers, and a second file section.
- `go/cmd/evolve/cmd_scan.go` — the new verb: flag parsing through the package's `cliFlags`, a pinned `git diff` argv (no color, no external diff or textconv, fixed `a/`/`b/` prefixes, `--` after the revision), rune-preserving masking, and the 0/1/2/10 exit codes.
- `go/cmd/evolve/cmd_scan_test.go` — in-process tests for the staged finding, clean and empty diffs, a range that ignores the index, git failures with empty stdout, the usage table, masking and registry lookup.
- `go/cmd/evolve/registry.go` — registers the `scan` command next to `commit-gate`.
- `go/cmd/evolve/main.go` — lists `scan secrets` in the top-level usage next to `ship`.
- `docs/operations/runtime-reference.md` — documents the verb under Operator commands: flags, sources, root selection, output and masking, exit codes and limits.
- `skills/commit/SKILL.md` — step 4 now runs `evolve scan secrets --staged` before `commit-gate run`; exit 1 sends the operator back to step 3 and exit 2 stops. The step numbers stay 1..6.
- `go/acs/cycle1805/predicates_test.go` — the TDD-authored predicates for the task (13 tests).
- `go/acs/cycle1805/helpers_test.go` — the TDD-authored helpers: binary builds, isolated env, runtime-built secret samples and git-state snapshots.
- `.evolve/evals/cli-scan-secrets.md` — the eval that pins the verb's acceptance to the cycle 1805 predicates.

## Design Decisions
`--diff` is parsed as a list flag so a repeated, empty or `-`-prefixed value is a usage error before git runs; this blocks option injection such as `--output=<file>`. Exit 2 prints nothing on stdout, so a git failure can never be read as a PASS. Masking keeps the first four runes and replaces the rest with `*`, which no detector rule matches, so a pasted report never re-trips the scanner. Masking lives unexported in `cmd/evolve` because it has one caller.

## Verification
`go test -tags acs -count=1 ./acs/cycle1805/` passes all 13 predicates, `./acs/cycle925/` (the scanner's earlier predicates) still passes, and `go test -count=1 ./internal/phases/secretleakscan/ ./cmd/evolve/` passes.

## Compatibility
`ScanDiff` and `Verdict` keep their signatures and the detector rules are unchanged. The new `Finding` fields are appended, the struct stays comparable, and the only composite literal uses field names. The /commit skill keeps its step numbering.

## Limitations
Binary and untracked files are not scanned; with a single ref, `--diff` compares against the working tree; an added line whose content starts with `++` is still read as a header; quoted paths are not unescaped.
