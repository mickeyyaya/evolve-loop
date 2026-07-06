# Plan: End-of-Cycle Workspace Hygiene (clean branches/worktrees/markers, strict TDD)

> Supersedes the completed quota-budgeting plan previously in this file (shipped v21.8.0/v21.9.0).

## Context

Operator directive: *"the system should make sure the code branches and worktrees are kept clean and ready for the next tasks once the previous one is done."* Audit (2026-07-06): **65 worktrees** + **106 never-deleted `cycle-*` branches** + **579 run dirs (189 `.reset-*`)** accumulated; **every** `max_cycles` batch exit leaves `.evolve/cycle-state.json` active with a dead owner → operator ran `evolve cycle reset --force` before every relaunch (6× in one day). Root cause of the forced resets: liveness checks use **lease TTL freshness only (10 min)** — a dead pid with a 2-6 min-old heartbeat blocks sealing.

**Execution route:** I implement this directly on a dedicated worktree/branch (`fix/workspace-hygiene`), strict TDD red-first per slice, gated commit (`/evo:commit`) per slice or a single PR at the end. The queued inbox item `cycle-end-workspace-gc` (0.94) will be UPDATED to reference this plan and downgraded/withdrawn to avoid a loop lane duplicating the work.

## Verified facts that shaped the design (pressure-tested against code; 4 draft premises corrected)

- **Run-dir GC already wired, mode-off**: `runGCHook` (`cmd_loop_outcome.go:85-148`) runs `gc.Discover→Plan→shadow-manifest→Apply` at loop boot (`cmd_loop.go:355`); dormant only because `gc.Mode` default `""`→`off`.
- **Boot autoseal already pid-aware**: `core.AutosealStaleMarker` (`stale_marker_autoseal.go:27-84`) + `pidAlive` (`cmd_loop_boot_recovery.go:170-179`). The freshness-only stragglers are exactly: `SealCycle` fence (`reset.go:141`), loop guard (`cmd_loop.go:317`), gc's `leaseFresh` (`discover.go:165`, `gc.go:315`).
- **`SealCycle` is ABANDON semantics** — renames run dir to `.reset-<ts>`, writes a faillearn lesson + `OperatorReset` failedApproaches record (`reset.go:188-224,264-272`). Using it at clean exit would poison failure-learning every healthy batch → S2 needs a new lighter primitive.
- **Branch deletion can't happen at the ship merge site** (`gitops.go:453-471`) — the branch is still checked out in the worktree there. Correct site: `gitWorktree.Cleanup` (`worktree.go:147-159`), which runs in the cycle-exit defer on `!preserve && completedNormally` (`cyclerun.go:360-366`); worktree leaf == branch name (runscope invariant).
- Branch names: `cycle-<lane8hex>-<N>` (+ swarm `-integration`/`-w<id>`); sweep must be evidence-based, not name-parsed. **All 106 existing branches verified merged** (ancestors of main); ≥1 worktree dirty (preserved ship-fail evidence — must be flagged, never deleted).
- Reusable seams: `gitexec.Git{Dir, Exec sysexec.RunFunc}`; `fixtures.FakeExec` (Scripts keyed `"git <subcmd>"`); `fixtures.NewWorkspace(t).WithGitInit()`; gc's Plan/Apply + `TestPlanNeverTouchesLiveDirs_Property` (rapid) pattern; policy nil-able-pointer + resolver-defaults idiom; `policy.go:89` already embeds `GC *gc.Policy`.
- apicover: `gc`/`core`/`runlease` all already enforced → every new export lands with named-by-identifier tests in the same slice (lesson cycle-542).

## Slices (each: RED first → minimal green → refactor; -race; apicover-named)

### S1 — pid-aware lease staleness (the bugfix killing forced `--force`)
- **RED**: `runlease_test.go` `TestOwnerLive_DeadPidFreshHeartbeatNotLive` (+ table: fresh+alive→true, fresh+pid0→true, nil-alive→true back-compat, stale+alive→false pinning the documented invariant). `core/reset_pidfence_test.go` `TestSealCycle_DeadOwnerFreshLease_SealsWithoutForce` + regression twin `_LiveOwnerFreshLease_StillRefuses`. `cmd_loop_reset_guard_test.go` `TestUnfinishedCycleGuard_DeadOwnerFreshLease_NotReportedAsLive` (stop_reason `unfinished_cycle`, not `owned_by_live_run`).
- **Impl**: `runlease.OwnerLive(l, now, ttl, alive)` = `Fresh(...) && (OwnerPID==0 || alive==nil || alive(OwnerPID))`; `SealOptions.PidAlive` consumed at the `reset.go:141` fence (keep populating `LeaseOwnerPID`/`HeartbeatAge` for refusal reports; update the F1 comment); `cmd_cycle.go:~113` + `cmd_loop.go:317` pass `pidAlive`. Do NOT rewrite `markerShouldAutoseal` (different predicate by design).
- **Edges**: pid reuse → conservative block, self-resolves at TTL; no host field (same-host tool; additive `Host` noted as backlog); nil-PidAlive callers unchanged.

### S2 — clean-exit marker finalize (no more manual seal at boundaries)
- **RED**: `core/cycle_finalize_test.go` `TestClearCompletedCycleMarker_RemovesTerminalMarker` (asserts: marker gone, state.json untouched, **no `.reset-*` dir, no lesson written** — pins the ≠SealCycle semantics), `_PreservesUnfinishedMarker` (cycle_id > lastCycleNumber → kept; protects --resume/quota-pause/signal), `_LiveOwnerUntouched`. `cmd_loop_finalize_test.go` `TestLoop_MaxCyclesExit_ClearsCompletedMarker` + `TestLoop_SignalExit_PreservesMarker` (via `loopOrchOverride`/`wireOrchestratorDepsFn` seams).
- **Impl**: new `core/cycle_finalize.go` `ClearCompletedCycleMarker(evolveDir, FinalizeOptions{Now, LeaseTTL, PidAlive}) (bool, error)` — resolve via `ResolveCycleStatePath` (fleet-safe: supervisor sees the global file), guards: cycle_id==0 / cycle_id>lastCycleNumber / OwnerLive → no-op; else `os.Remove`. Call (WARN-only, never changes rc) at the terminal-exit sites in `cmd_loop.go` (before `lr.emit` ~766-779, + `resumed_complete` ~286). NOT on signal/quota-pause paths (predicate would no-op anyway — explicit intent).
- **Edges**: final-cycle FAIL → counter not advanced → marker preserved (resumable evidence, correct); fleet lanes write per-run markers — supervisor finalize touches only global; torn marker → error+WARN+leave.

### S3 — in-cycle branch deletion (stops NEW debt)
- **RED**: `core/worktree_test.go` `TestCleanup_DeletesMergedCycleBranch` (FakeExec: asserts `branch -d <leaf>` in projectRoot AFTER `worktree remove`), `TestCleanup_UnmergedBranchSurvives_WarnsOnly` (rc=1 → Cleanup still nil, never `-D`). Real-git integration: extend `worktree_realgit_integration_test.go` (merged→branch gone; unmerged→survives). Same pair for `swarm/provision.go` Cleanup.
- **Impl**: in `gitWorktree.Cleanup` after remove+RemoveAll: `branch := filepath.Base(worktree)`; if `cycle-` prefix → `git branch -d` in projectRoot; non-zero → WARN, never fail, never `-D` (git's merged-check is the safety). Mirror in swarm Cleanup. Verify `cmd_worktree.go` cleanup inherits or add same.
- **Edges**: runs only post-ship (`!preserve && completedNormally`) → branch merged or at base HEAD, `-d` succeeds; WARN-verdict orphan commits → `-d` refuses → S4 backlog; single-ref delete atomic, no ship.lock needed.

### S4 — worktree+branch backlog GC (gc-sibling planner, Plan/Apply command pattern)
- **RED** (`internal/gc/`): `worktrees_test.go` — `TestPlanWorktrees_MergedCleanDeadIsCollected`, `_DirtyIsFlaggedNeverRemoved`, `_UnmergedBranchKept`, `_LiveLeaseExcluded` + `_DeadPidFreshLeaseCollected` (gc pid-awareness red), `_KeepRecentAndMinAge`, `_BranchBacklogSweep_NotCheckedOutMergedOnly` (legacy `cycle-7`, lane, `-integration`, `-w0` names via `cycle-` prefix + merged + unattached), `TestApplyWorktrees_TOCTOURecheckRefusesNewlyLiveOrDirty`, `_NonForceRemoveAndPrune` (exact argv: NO `--force`, `-d` never `-D`, trailing `worktree prune`, under `.evolve/ship.lock`). Property: `worktrees_fuzz_test.go` `TestPlanWorktreesNeverTouchesLiveDirtyUnmerged_Property` (rapid; mirrors `TestPlanNeverTouchesLiveDirs_Property`). One real-git end-to-end `worktrees_realgit_test.go`.
- **Impl**: new `gc/worktrees.go`: `WorktreesPolicy{KeepRecent(2), MinAgeMinutes(15 — wider than 10-min lease TTL, covers create→lease window)}` embedded as `gc.Policy.Worktrees` + `withDefaults()`; separate `WorktreeItem/WorktreeManifest` types (don't overload `gc.Manifest`); actions `remove-worktree|delete-branch|flag-dirty|flag-unmerged` (flags report-only); `PlanWorktrees(WorktreeOptions{ProjectRoot, WorktreeBase, EvolveDir, Policy, Now, Exec sysexec.RunFunc, PidAlive, LeaseTTL})` — evidence pipeline: `git worktree list --porcelain` registered + `cycle-` leaf + under base; liveness = mapped run-dir lease `OwnerLive` (fail closed) + `active_worktree` refs from global AND all per-run cycle-states (fleet lanes); unparseable leaf → require age>7d else skip. `ApplyWorktrees`: whole-apply under ship.lock (`adapters/flock`, context timeout like `orphanGCTimeout`), per-item TOCTOU re-check, non-force remove, `-d`, joined errors, trailing prune. Update gc package doc (no longer stdlib-only; still no `core` import — exec injected).
- **Edges**: fleet lanes mid-sweep → triple shield (lease-before-state ordering + MinAge + Apply re-check under ship.lock); sealed cycles' run dirs moved → no lease → collected if merged+clean; swarm integration branches unmerged-by-design → kept; locked worktrees skipped; corrupt stubs → flag-only in v1.

### S5 — wiring: batch-end hook, shadow default, operator surface
- **RED**: `cmd_loop_gc_test.go` `TestRunGCHook_ShadowWritesWorkspaceManifest`, `_EnforceAppliesWorktreeSweep`, `TestRunGCHook_DefaultModeIsShadow` (red for the `""`→`shadow` flip at `cmd_loop_outcome.go:94-105`); `TestLoop_BatchEndRunsGCHook` (introduce `runGCHookFn` seam mirroring `bootRecoverFn`). `cmd_gc_test.go` `TestGC_DryRunPrintsWorkspacePlan` / `_AppliesWorkspaceSweep` (explicit `evolve gc` ≡ enforce, documented asymmetry). ACS batch-level anti-no-op predicate (cf. `acs/cycle298`).
- **Impl**: `runGCHook` += `PlanWorktrees`→manifest (`workspace-gc-manifest.json`, tmp+rename)→Apply iff enforce; mode default `""`→`shadow` (non-mutating; enforce stays a policy.json decision — update the `gc_test.go:563-567` zero-value pin). `cmd_loop.go` batch-end: `finalizeCompletedCycleMarker` (S2) THEN `runGCHook` (finalize first so the ended marker doesn't pin its workspace as current). `cmd_gc.go` += workspace sweep with dry-run parity, report counts + every flagged path.
- **Edges**: boot ordering already correct (autoseal :295 → hook :355); double invocation per batch fine (fresh Plan each time; never re-Apply stored manifests).

## Sequencing

S1 → S2 (consumes OwnerLive) → S3 (independent) → S4 → S5 (land S4+S5 adjacent — avoid the "green unit, absent integration" cycle-506/507 gap). Backlog drain: flip `policy.json gc.mode` to `enforce` only after reviewing one shadow batch's `workspace-gc-manifest.json` (expected: ~60 removals, ~106 branch deletes, dirty trees flagged-not-removed).

## Verification

- Per slice: `go test -race ./internal/runlease/... ./internal/core/... ./internal/gc/... ./cmd/evolve/` + repo-wide `go vet ./...`, `gofmt -s`, `apicover -enforce` (CI-parity).
- End-to-end: after S5 + one shadow batch → review manifest → flip enforce → next batch boundary needs ZERO operator intervention (no seal, no manual sweep), worktree base contains only live/keep-recent trees, `git branch --list 'cycle-*'` shows only live lanes, relaunch starts cycle N+1 without `cycle reset`.
- Success metrics vs audit baseline: 65→≤3 worktrees, 106→≤2 branches, forced-reset count 6/day→0.

---

## EXECUTION ROUTE (amended 2026-07-06, operator directive)

**The evo loop implements this plan** via five weighted inbox items `workspace-hygiene-s1..s5` (one per slice). The earlier "operator implements directly" route is WITHDRAWN.

**Reference implementation note:** S1 was already built and verified green (race/vet/fmt, full cmd/evolve suite) on branch `fix/workspace-hygiene` (worktree `../evolve-loop-hygiene`, based on e9afbda1) before this amendment. That branch is REFERENCE ONLY — the loop should implement S1 from this plan's spec; builders may consult the branch diff (`git diff main...fix/workspace-hygiene`) but must produce their own cycle-shipped implementation. Slice dependencies: S2 and S4 consume S1's `runlease.OwnerLive`; triage should defer them until S1 ships.
