# Build Explanation — Cycle 1801

## Build Binding
- Cycle: 1801
- Base SHA: a25b1e355da10559a65cdb2eb80f26ceebdfb325

## Summary
Against this base the diff makes four changes. First, a new `evolve ci watch (--sha S | --pr N | --tag T) [--workflow W]... [--cycle N]` waits for a pushed commit's, PR's or tag's CI to complete and gives `ciwatch.Watch` its first production caller. Second, a bare non-dry `evolve gc` now refuses before any tmux reaper runs, and the reapers become an injectable seam. Third, a new `evolve failures` command group (`list`, `reset`, `prune`) shares one prune-plus-acknowledge function with `loop --reset`. Fourth, the superseded cycle-1798 and cycle-1800 predicate packages and unshipped explanations are archived. The gc and failures work comes from the adopted cycle-1800 continuation. Cycle 1801 adds `ci watch`, a stronger gc refusal test and the cycle-1801 predicates.

## Rationale
`/commit` and `/evo:publish` watched CI by hand with `gh run watch`, which files nothing on red and prints no classification, while `ciwatch.Watch` (poll, verdict, fix-forward inbox item) had no production caller. `ci watch` puts both behind one command with the exit-code contract the skills need. At the base, `runGC` ran the session and socket reapers before `--project-root` was resolved, so a bare `evolve gc` killed tmux sessions and only then refused. cmd/evolve tests that ran a non-dry gc also reached the host's real reapers. Apart from `loop --reset`, the operator had no command to inspect or clear `failedApproaches`.

## Changed Areas
- `go/cmd/evolve/cmd_ci_watch.go` — new `ci watch` subcommand. It parses and validates the target before any `gh` call (usage exit 10), resolves the target to a full SHA, loads `ci_watch.timeout_s`/`poll_s` from policy.json, and calls `ciwatch.Watch` once per workflow: `required.yml`, plus `release.yml` for `--tag`, or the repeated `--workflow` values instead. It prints one line per workflow with its conclusion, runs `ci classify` on each red run, and exits 0 green / 1 red / 2 unobservable.
- `go/cmd/evolve/cmd_ci.go` — `runCI` routes `watch` to `runCIWatch`, and the ci usage lists both subcommands, so a typo shows `ci watch` as well.
- `go/cmd/evolve/cmd_pr.go` — the shared `cliFlags` parser gains `lists`, a repeatable value flag that `--workflow W` needs; `bools` and `values` behave as before.
- `go/cmd/evolve/main.go` — the help block lists `ci watch` and its usage under `ci`, so `evolve help` exposes the command.
- `go/cmd/evolve/registry.go` — the `ci` summary names `ci watch`; it also registers the `failures` verb from the adopted continuation.
- `go/internal/ciwatch/ciwatch.go` — new `Options.NoEscalation`. With it set, a red conclusion is reported without filing an inbox item and `InboxDir` is not required. This is how `--pr` honours "a red PR files nothing" without a throwaway directory.
- `go/internal/ciwatch/ghfetcher.go` — new `NewGHWorkflowFetcher(root, workflow)`. `NewGHFetcher` and `LatestRequiredRunOnBranch` delegate to the same `latestRun` with `required.yml`, so their behaviour is unchanged.
- `go/internal/ciwatch/classify_source.go` — new `ResolveCommit` (tag or short SHA, through the commits API) and `ResolvePRHead` (`gh pr view --json headRefOid`). Both refuse anything that is not a 40-hex SHA, so `gh run list --commit` always gets a full SHA.
- `go/internal/ciwatch/watch_target_test.go` — unit tests for the named-workflow fetcher, both resolvers (including the refusal and gh-error paths) and `NoEscalation`.
- `go/cmd/evolve/cmd_ci_watch_test.go` — a table test of `ci watch` argument parsing and workflow selection, plus a `runCI` routing and usage test.
- `go/cmd/evolve/cmd_gc.go` — `runGC` resolves the project root right after flag parsing and returns the refusal (exit 1) before `newGCRun` or any reaper runs. The `-project-root` help says the flag is required for a non-dry run and that only `--dry-run` falls back to the cwd. New `gcReapers{sessions,sockets}` seam, which defaults to the `swarm` exec reapers, with a package-level override for tests.
- `go/cmd/evolve/gc_reapers_fake_test.go` — installs no-op reapers for every cmd/evolve test so no test reaches host tmux.
- `go/cmd/evolve/cmd_gc_workspace_test.go` — the bare-gc refusal test now also injects counting reapers and asserts that none ran before the refusal.
- `go/cmd/evolve/cmd_loop_maintenance.go` — extracts `resetFailures`. The prune and the fingerprint acknowledgement run independently, as at the base, and both errors are returned in `resetOutcome`. `resetBatchState` prints the base `[loop] --reset:` and `[loop] --reset --fingerprint:` messages on every path.
- `go/cmd/evolve/cmd_failures.go` — new 176-line `failures` command group. `list [--class C] [--json]` is read-only and treats an absent state file as empty. `reset [--fingerprint F]` goes through `resetFailures`. `prune` drops expired entries. `reset` and `prune` refuse (exit 1) without an explicit `--project-root`; usage errors exit 10.
- `go/cmd/evolve/cmd_failures_test.go` — table tests for `failures list` filtering and rejections; `failures prune` (removes only expired entries, refuses without `--project-root`, leaves unparseable state untouched); `failures reset` (drops only the reset classes, acks after a prune error, reports a committed prune and keeps the operator's file when the ack fails); `resetBatchState`; and `runLoop --reset` driven through the real entry point on the unparseable-state, ack-error and no-fingerprint paths. These replace the binary-level coverage that left with the archived cycle-1800 predicates.
- `go/acs/cycle1801/predicates_test.go` — TDD-authored predicates 001-013 for gc refuse-before-reap and `ci watch`, driving the real binary.
- `go/acs/cycle1801/helpers_test.go` — the predicates' fixtures: the built binary, a git fixture, and a fake `gh` served by the test binary.
- `.evolve/evals/cli-ci-watch.md` — new eval whose score caps cite the cycle-1801 `ci watch` predicates.
- `.evolve/evals/gc-reaps-before-its-refusal.md` — eval for the gc fix, pointed at the cycle-1801 predicates.
- `.evolve/evals/cli-failures.md` — eval for the `failures` group, from the adopted continuation. It still cites the archived `./acs/cycle1800` package. The builder sandbox denies writes to `.evolve/evals`, so repointing it at the live `cmd/evolve` tests above is left to the eval's owner (see Limitations).
- `docs/operations/runtime-reference.md` — documents `ci watch`: targets, workflows, `--workflow`, `--cycle`, output, the inbox rule and exit codes 0/1/2/10.
- `skills/commit/SKILL.md` — the CI-watch step uses `evolve ci watch --sha` on main or `--pr` on a PR branch instead of `gh run watch`.
- `skills/publish/SKILL.md` — the post-release watch uses `evolve ci watch --tag` instead of a hand loop over `gh run watch` for `required.yml` and `release.yml`.
- `docs/private/research/archived-2026-10-05/superseded-predicate-packages/cycle1798/predicates_test.go` — archived predicates of the superseded cycle-1798 attempt.
- `docs/private/research/archived-2026-10-05/unshipped-build-explanations/cycle-1798-01m45zb1eybc2fbds854t09qhq.md` — archived explanation of the unshipped cycle-1798 attempt.
- `docs/private/research/archived-2026-10-06/superseded-predicate-packages/cycle1800/predicates_test.go` — the cycle-1800 predicates, moved out of `go/acs` because cycle 1801's predicates supersede them.
- `docs/private/research/archived-2026-10-06/unshipped-build-explanations/cycle-1800-01m46d125gd0yawht36w4yhxkf.md` — the unshipped cycle-1800 explanation, archived. This document replaces it for the whole diff.

## Design Decisions
`ci watch` calls `ciwatch.Watch` sequentially, once per workflow, so each workflow gets the full policy timeout and the inbox item still comes from `Watch`'s own escalation (`source: ciwatch`). `--workflow` replaces the defaults rather than adding to them, which gives the operator exact control. A red classify that fails is only a stderr warning, because exit 2 is reserved for failing to observe the watched runs. Red runs are classified by run id, so a red `release.yml` or `go.yml` is classified as that run, not as the newest `required.yml` run. The explicit command ignores `ci_watch.enabled`, which governs only the in-loop watch. For gc, root resolution moved to the top of `runGC`, so every side effect sits behind the single refusal check, and a package-level override keeps `runGC`'s registry signature unchanged.

## Verification
The cycle-1801 predicates pass 13/13 (`go test -tags acs -count=1 ./acs/cycle1801`). The new `internal/ciwatch` and `cmd/evolve` unit tests pass. The `failures` and `loop --reset` tests were mutation-checked: skipping the ack after a prune error, or pruning against a shifted clock, turns them red. gofmt, go vet and `go test -count=1 ./...` were run from `go/`; build-report.md records the counts.

## Compatibility
This change alters CLI behaviour. `evolve ci watch` and `evolve failures` are new verbs with their own output and exit codes. The `ci` usage text gains a second line. A bare non-dry `evolve gc` still exits 1 with "mutating run refused", but it no longer reaps or prints reaper output before refusing, and its `-project-root` help text changed. `loop --reset` keeps the base messages on every path. `ci classify`, `NewGHFetcher` and `LatestRequiredRunOnBranch` behave as before. The `/commit` and `/evo:publish` skills now call `evolve ci watch`, so they need a binary that includes it.

## Limitations
If several watched workflows go red on the same SHA in the same second, their fix-forward items share `Watch`'s file name, so only one item survives. The watch is sequential, so the worst-case wait is one policy timeout per workflow. `.evolve/evals/cli-failures.md` still cites the archived cycle-1800 predicates, so its graders select no tests until the TDD phase, which owns `.evolve/evals`, repoints them. The replacement graders are `go test -count=1 -run 'TestRunFailures_ListFiltersAndRejects|TestRunFailures_PruneRemovesOnlyExpiredEntries|TestRunFailures_ResetErrorPaths' ./cmd/evolve` and `go test -count=1 -run 'TestRunLoop_ResetErrorPathsKeepBasePrefixes|TestResetBatchState_KeepsBasePrefixesOnEveryErrorPath' ./cmd/evolve`. The `failures` group's coverage is unit-level in `cmd/evolve` (it calls `runFailures` and `runLoop`), not a built-binary ACS predicate, because the builder may not author predicates. The deferred lane items cli-boundary-run and cli-land-patch are not built.
