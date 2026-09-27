# Build Explanation — Cycle 1730

## Build Binding
- Cycle: 1730
- Base SHA: ad816b703875ab36167fe3b78e18592242ca80c1

## Summary
Each of the eight oversized `internal/cli/opscmd` functions named by the
`sizeratchet-shrink-opscmd` task is now at or under `sizeratchet.MaxLines`
(50). The eight are `RunChangelogGen`, `RunConsoleLease`, `RunMarketplacePoll`,
`RunReleasePipeline`, `RunReleasePreflight`, `RunRollback`, `runDoctorBoot`
and `runDoctorLive`. Their `go/internal/sizeratchet/offenders.json`
allowances are deleted. The diff adds no comment lines, and the eight
exported or entry-point doc comments are byte-identical to the baseline.
New characterization tests pin how the doctor live/boot commands report
their results.

## Rationale
Each function did three jobs in one body: argument parsing, validation, and
the operation call with its error-to-exit-code mapping. Pulling those parts
out into named, single-call-site helpers brings each entry point under the
limit. The helpers' names carry the intent. They get no doc comments, because
the repo's comment convention (`docs/conventions/code-comments.md`, graded by
`commentaudit comments`) forbids new comments that restate the code.

## Changed Areas
- `go/internal/cli/opscmd/changelog.go` — `RunChangelogGen` hands argument
  parsing to `parseChangelogGenArgs`. It hands ref verification, the git log
  and entry rendering to `buildChangelogEntry`.
- `go/internal/cli/opscmd/console_lease.go` — `RunConsoleLease` hands flag
  and positional parsing to `parseConsoleLeaseArgs`, the `--clear` branch to
  `clearConsoleLease`, and the write path to `writeConsoleLease`. The
  baseline's interspersed-parse comment (why the loop re-parses instead of
  using `cmdutil.ReorderArgs`) moves verbatim with its loop.
- `go/internal/cli/opscmd/marketplace_poll.go` — `RunMarketplacePoll` hands
  argument parsing to `parseMarketplacePollArgs`, which also uses
  `marketplacePollFlagCase`, `marketplacePollIntFlag` and
  `printMarketplacePollUsage`. Exit-code mapping moves verbatim to
  `marketplacePollExitCode`. The two positive-int flags now share
  `marketplacePollIntFlag`. It prints the same bytes as the two baseline
  literals because the flag name is substituted with `%s`.
- `go/internal/cli/opscmd/release_pipeline.go` — `RunReleasePipeline` hands
  argument parsing to `parseReleasePipelineArgs`, which also uses
  `releasePipelineFlagCase` and `printReleasePipelineUsage`. Exit-code mapping
  moves to `releasePipelineExitCode` as the baseline's exact `if` chain, so
  `ErrPrePublishFailed` is still checked first.
- `go/internal/cli/opscmd/release_preflight.go` — `RunReleasePreflight` hands
  argument parsing to `parseReleasePreflightArgs`, with usage printed by
  `printReleasePreflightUsage`.
- `go/internal/cli/opscmd/rollback.go` — `RunRollback` hands argument parsing
  to `parseRollbackArgs`. Exit-code mapping moves verbatim to
  `rollbackExitCode`.
- `go/internal/cli/opscmd/doctor_boot.go` — `runDoctorBoot` hands the bridge
  config and the optional sandbox worktree to `doctorBootConfig`, which
  returns a cleanup func. Result reporting moves verbatim to
  `reportDoctorBootResult`.
- `go/internal/cli/opscmd/doctor_live.go` — `runDoctorLive` hands result
  reporting to `reportDoctorLiveResult`, a verbatim move.
- `go/internal/cli/opscmd/doctor_report_characterization_test.go` — adds characterization tests that pin the exact stdout/stderr bytes and exit codes of the extracted doctor report helpers and of the `RunDoctor` entry point.
  Why: the only earlier test reaching `runDoctorLive` / `runDoctorBoot` was a
  smoke test that asserted `rc ∈ {1,10}`. It would not go red on a mutant of
  the moved `report*Result` code, so these tests are what prove the move kept
  behavior. They were authored by the TDD phase.
- `go/internal/sizeratchet/offenders.json` — the eight now-compliant
  `internal/cli/opscmd.*` entries are deleted. The ratchet's contract is that
  a fixed function's allowance is deleted, never lowered.

## Design Decisions
The code moved as-is; only its location changed. No baseline comment was
added or deleted: every comment inside moved code travels with that code. The
helpers do reshape it mechanically in four ways, and each keeps the output
bytes, exit codes and evaluation order the same:
- Parse-loop locals become fields of a small per-command flags struct
  (`marketplacePollFlags`, `releasePipelineFlags`, `releasePreflightFlags`),
  or become named return values.
- An early `return N` becomes `return ..., N, true`. The entry point then
  returns that code.
- `doctorBootConfig` returns the worktree cleanup as a func. The entry point
  defers it after the workspace cleanup, so the two removals still run in the
  baseline's LIFO order.
- In `parseMarketplacePollArgs` and `parseReleasePipelineArgs`, the baseline
  `i := 0; for i < len(args) { switch …; i++ }` loop becomes
  `for i := 0; i < len(args); i++`, and the flag cases move into
  `marketplacePollFlagCase` / `releasePipelineFlagCase`, which advance `i`
  through a pointer. The baseline switch had no `continue`, so every
  iteration still increments `i` exactly once after its case. The
  `--help`/`-h` case was the switch's first case, so hoisting it into an `if`
  ahead of the flag helper keeps it checked first. Its `Fprintln` lines move
  verbatim into `printMarketplacePollUsage` / `printReleasePipelineUsage`.

This round reverts two first-round rewrites that were not straight moves.
The first rewrite collapsed `releasePipelineExitCode` into one up-front
`Fprintf` plus a `switch` that did not check `ErrPrePublishFailed`. An error
wrapping both `ErrPrePublishFailed` and `ErrShipFailed` would then have exited
2 instead of 1. The second rewrite dropped the `ErrRuntime` branch of
`marketplacePollExitCode`. That was equivalent, but it was not a verbatim
move. Both helpers are now the baseline code.

## Verification
- `cd go && go test -tags acs -count=1 -v ./acs/cycle1730/...` — 9/9 PASS.
  The predicates cover the line limit, the offenders cleanup, the
  module-wide ratchet check, frozen baseline tests, the package tests, and
  build/vet. They also cover zero added comment lines, target docs equal to
  the baseline, and characterization tests that kill 5 mutants.
- `cd go && go test -count=1 ./internal/cli/opscmd/... ./internal/sizeratchet/...`
  — PASS.

## Compatibility
No CLI flag, exit code, usage string or output message changed. All eight
functions keep their names, signatures and doc comments.

## Limitations
The "one level of abstraction per function" readability judgment has no
mechanical grader, so it is left to the auditor. The characterization tests'
report-helper layer cannot run against the baseline, because the helpers did
not exist there. For those helpers, equivalence rests on the verbatim move
plus the `RunDoctor` layer, which does run on the baseline. Other oversized
functions in `offenders.json` are untouched.
