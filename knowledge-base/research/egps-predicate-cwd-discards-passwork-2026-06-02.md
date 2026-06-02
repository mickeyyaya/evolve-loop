# EGPS Predicate Suite Runs Against MAIN, Not the Worktree → Discards PASS-Audited Work

**Date:** 2026-06-02
**Found in:** cycle-190 (harden test-resilience run), tmux multi-CLI config (agy builder / codex→claude auditor).
**Severity:** HIGH — any cycle that adds NEW code + a predicate asserting it will RED-flag and **discard valid, PASS-audited work**, and (with cost=$0 budget tracking on subscription auth) the loop re-does the same goal indefinitely without shipping.
**Status:** FIXED on branch `fix/egps-predicate-cwd` (worktree `../evolve-loop-egpsfix`), TDD, all runtime-tier tests + race green — **UNCOMMITTED, awaiting operator sign-off before merging** (trust-kernel EGPS gate change). See "Implemented fix" below.

## Symptom

Cycle 190 implemented all 4 seeded deliverables (FilePresent helper, 106 skip-guard migration, floor test, chaos tests, CI gate) and the **auditor reported PASS (7/7 ACs)**, explicitly citing "predicate 001 GREEN (exit 0)". But the orchestrator logged:
```
WARN cycle 190 ended without shipping (SKIPPED_UNKNOWN): phases ran but HEAD
did not advance ... any worktree changes were discarded.
cycle 190 cost: $0.0000
```
`acs-verdict.json` showed **9 RED predicates**, incl. `cycle-190/001-filepresent-helper`: *"TestFilePresent matched no tests — FilePresent unit test not authored yet"*. `red_count>0` → EGPS ship gate blocked → work discarded. Main tree confirmed to **lack** `FilePresent` (discarded).

## Root cause (definitive)

`generateACSVerdict` (`go/internal/phases/audit/audit.go:224-234`):
```go
root := req.Worktree                       // predicates DISCOVERED from worktree
acssuite.Run(Options{Root: root, ProjectRoot: req.ProjectRoot, Cycle: ...})
```
`acssuite.runBash` (`go/internal/acssuite/acssuite.go:244-260`):
```go
cmd := exec.CommandContext(ctx, "bash", path)
// sets EVOLVE_PROJECT_ROOT=main … but NEVER sets cmd.Dir
```
So predicate scripts are **discovered from the worktree** but **executed with cwd = the `evolve loop` Go process's cwd = MAIN**. A predicate's `go test` therefore compiles the **MAIN** source tree, which does NOT contain the builder's changes (committed in the worktree, then `normalizeWorktreeToBase` soft-resets them to *pending/uncommitted* in the worktree — `orchestrator.go:441`). The auditor subagent runs cwd=worktree (the documented "issue #9 audit-cwd=worktree") and sees the changes → GREEN. The two evaluate **different source trees** → contradiction → valid work discarded.

`grep os.Chdir` across audit/runner/acssuite/orchestrator: none. `Options` has `Root`, `ProjectRoot`, `Timeout`, `Now`, `Exec` — **no execution-cwd field**.

## Proposed fix (small code, high stakes)

Set the predicate execution cwd to the worktree (the source tree being shipped) while keeping `.evolve/` resolved to main via `EVOLVE_PROJECT_ROOT` (already correct):
- Add `cmd.Dir = <worktree>` in `runBash` (thread `opts.Root` — already the worktree — or a new `Options.WorkdirRoot`).
- Since `Root` is already the worktree for worktree cycles and `ProjectRoot` (main) for non-worktree cycles, `cmd.Dir = opts.Root` is correct in both cases and matches the documented "issue #9 audit-cwd=worktree" intent that was never wired into execution.

**TDD:** inject a fake `Exec`/runner (Options.Exec seam already exists) OR assert `cmd.Dir` via a probe; predicate writes a marker resolving cwd → assert it sees the worktree tree. **Validation risk:** regression-suite predicates accumulated from prior cycles may assume cwd=main; they read `.evolve/` via the absolute `EVOLVE_PROJECT_ROOT` (unaffected), but any that compute `REPO_ROOT=$(git rev-parse --show-toplevel)` will now resolve the worktree. Run the FULL regression suite under a worktree cycle before merging.

## Implemented fix (branch `fix/egps-predicate-cwd`)

- **#2 EGPS cwd:** `acssuite.runBash` now takes a `workdir` arg and sets `cmd.Dir = opts.Root` (the worktree for worktree cycles, main otherwise), so a predicate's `go test` compiles the tree being shipped. `.evolve/` still resolves to main via `EVOLVE_PROJECT_ROOT`. Wires the documented "issue #9 audit-cwd=worktree" intent into execution. TDD: `acssuite_cwd_test.go:TestRun_PredicateExecutesWithCwdAtRoot` (sentinel-in-worktree predicate is RED before / GREEN after).
- **#4 cost unobservable:** `budgetGatingUnobservable(budgetDriven, cycleCostDelta)` (cmd_loop.go) → the loop emits a ONE-TIME loud `WARN BUDGET-UNOBSERVABLE` when a `--budget-usd` run's cycle reports $0 cost (tmux/subscription auth surfaces no usage), telling the operator the cost stop is inert and the cycle cap governs (use `--cycles N`). It deliberately does NOT fabricate a stop. TDD: `cmd_loop_test.go:TestBudgetGatingUnobservable`.
- Validation: full runtime tier (`go list ./... | grep -v /acs/`) PASS; acssuite/audit/ship/core/cmd race-clean; gofmt -s + vet clean.

## Related / compounding

- **cost=$0 budget tracking:** on tmux-driver / subscription auth the per-cycle cost reads `$0.0000`, so `--budget-usd N` never trips → the loop runs to the 50-cycle safety cap. Separate issue; relevant because it removes the natural stop on a spinning loop. Worth a dedicated fix (capture usage from tmux scrollback, or fall back to a per-cycle count cap when cost is unobservable).
- See also [observer-false-stall-tmux-liveness-2026-06-02.md](observer-false-stall-tmux-liveness-2026-06-02.md) (separate cycle-190 issue, FIXED on branch `fix/observer-tmux-liveness`).
