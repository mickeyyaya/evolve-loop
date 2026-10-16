# Build Explanation — Cycle 1890

## Build Binding
- Cycle: 1890
- Base SHA: 1e20b75e04922001b149e73d7171f6c99918ce5a

## Summary
`evolve release-preflight` now refuses a release when a commit subject since the last tag starts with `WIP`, `fixup!`, `squash!` or `amend!`. The error lists each such commit. The new `--require-ci` flag makes an unavailable remote CI verdict fail. The `/evo:release` skill now calls the preflight with `--require-ci` instead of its own `gh run list` and `git log` steps.

## Rationale
The two checks lived only as prose in the skill. No test pinned them, and a direct preflight call skipped them. The preflight already had the CI lookup and a seam pattern for every git call. Thus the smallest change adds one more seam and one more uncounted gate beside the CI gate.

## Changed Areas
- `go/internal/releasepreflight/releasepreflight.go` — adds the `RequireCI` and `WIPCommits` options, the default `git describe` plus `git log` scan and the anchored subject filter.
- `go/internal/releasepreflight/preflight_run.go` — resolves the new seam and flag, adds the no-WIP gate before the CI gate, and fails an unavailable CI verdict under `RequireCI`.
- `go/internal/releasepreflight/wip_gate_test.go` — tests the subject filter, the gate through `Run`, the scan error, the dry-run skip, `RequireCI` and the real-git scan.
- `go/internal/releasepreflight/releasepreflight_test.go` — stubs the new seam in `stubOpts`, because the shared fixture is not a git repo.
- `go/internal/releasepreflight/run_stage_order_test.go` — stubs the new seam in `poisonedOpts` for the same reason.
- `go/internal/cli/opscmd/release_preflight.go` — parses `--require-ci`, forwards it and documents both gates in the usage text.
- `go/internal/cli/opscmd/release_preflight_backfill_f2_test.go` — asserts that the CLI forwards `--require-ci`.
- `commands/release.md` — updates the command description: the readiness gate now runs `evolve release-preflight --require-ci`, so the separate CI-green and no-WIP checks leave the description.
- `skills/release/SKILL.md` — delegates the CI and WIP checks to the preflight and drops the `--dry-run` that skips them.
- `docs/architecture/packages/internal-releasepreflight.md` — records the no-WIP gate and the `RequireCI` rule.
- `docs/operations/runtime-reference.md` — names the no-WIP gate and `--require-ci` in the release lifecycle.
- `go/acs/cycle1890/predicates_test.go` — the cycle 1890 ACS predicates for this task.
- `.evolve/evals/cli-release-preflight-no-wip.md` — the evaluation rubric and graders for this task.
- `docs/private/research/archived-2026-10-16/superseded-predicate-packages/cycle1887/predicates_test.go` — archived cycle 1887 predicates superseded by cycle 1890.
- `docs/private/research/archived-2026-10-16/unshipped-build-explanations/cycle-1887-01m51hss19tfkp2226b47g1b72.md` — archived unshipped build explanation from cycle 1887.

## Design Decisions
The gate runs after the five counted steps, like the CI gate, so `StepsTotal` stays 5 and no existing step-count test changes. A scan error fails closed, because a release must not pass a gate that has not run. The filter matches the subject only, after the short SHA, so a SHA can never match. Dry-run skips the gate, as it skips every other git check.

## Verification
The cycle 1890 ACS predicates call the CLI entry against real `gittest` repos and a fake `gh`. The package tests cover the filter and each gate branch.

## Compatibility
Without `--require-ci` the CI behavior is unchanged. A repo with a WIP commit since its last tag now fails the preflight; this is the intended change.

## Limitations
The scan uses the nearest tag that `HEAD` reaches, not the newest release tag by version. A dry run does not check WIP commits.
