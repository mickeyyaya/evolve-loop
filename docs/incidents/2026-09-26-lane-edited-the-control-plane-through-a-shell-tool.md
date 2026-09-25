# 2026-09-26 — a lane edited the control plane through a shell tool, and nothing looked until ship

**Class:** pipeline. A lane's diff touched the protected control plane. The one check that could have made the builder undo it ran only after the audit, and its refusal would have recovered into a loop that could not help.
**Surface:** `core.DefaultBuildFloorChecks` (the build handoff floor), `phases/ship/integrity.go` (ADR-0064 P2), `router/recovery.go` (the ship-failure recovery chain).
**Found by:** the console, watching wave 7 (cycle 1689, lane `lost-ship-closeout-universal-landing-witness`) during the six-consecutive-ships campaign.

## What happened

Cycle 1689's builder reshaped `landingEvidence` in `go/internal/core/lost_landing_floor.go` (dropping its `ProjectRoot` field). It then fixed the one call site in `go/internal/core/cyclerun.go`, which is on the control-plane manifest ("fresh-cycle explanation contract-version stamp"). The edit went through a shell tool: `guards.log` in the lane worktree has no role-gate decision at the file's modification time (18:54:49Z). ADR-0064 P1, the Edit/Write role guard, only sees Edit and Write.

The build handoff floor then approved the build. Its deterministic engine checks removal claims, placeholder tokens, the persona budget and the changed packages' tests. It never asked the protected-surface question. The lane moved to audit with the protected edit in its base diff. The audit's integration tier then found a regression in the lane's own change (`TestResumeLifecycle_LostLandingUsesSameTerminalFloor`), and the operator stopped the loop before ship.

Had it reached ship:
- P2 (`verifyNoControlPlaneEdits`) would have refused the diff with `CONTROL_PLANE_VIOLATION`, class `precondition`.
- The recovery chain sends every precondition to `precondition-reaudit`, which re-verifies the same diff. So the lane could only have repeated audit → ship refusal until the recovery budget (2) aborted it. That is the cycle-230 audit↔ship loop `shipLocalCodes` exists to prevent.

No lane had reached ship this way before (no `CONTROL_PLANE_VIOLATION` in cycles 1500–1688). The role guard and the seed- and triage-time protected-surface filters normally keep lanes off protected files. A shell write inside an admissible item is what got past all three.

## Root cause

The integrity boundary had two checks. P1, at edit time, covers only two tools. P2, at ship, runs at the one phase that cannot act on its finding: neither ship nor audit can reshape a diff. The phase that owns the diff, build, had a deterministic handoff floor with a correction ladder built for exactly this ("fix these exactly before handoff"). P2's question was not on it.

## Fix (F37)

1. **The build handoff floor asks P2's question.** `core.ProtectedSurfaceFloorChecks(member)` fails the handoff for every changed path on the manifest. It judges the cycle-base diff (HEAD when no base is recorded) plus untracked files. That is the set P2 judges once the post-record soft-reset puts HEAD back at the base.
   - Each failure line names the path, the boundary, and the exact restore (`git checkout <cycle base> -- <path>`, or delete the file if it is new).
   - It tells the builder to restore the file in every case. A note in `build-report.md` cannot clear a path check.
   - `guards.IsProtectedSurface` is injected at the composition root, because `guards` imports `core`.
   - The one composition, `productionBuildFloorChecks`, is a named function in the protected `cmd/evolve/cmd_cycle_config.go`. It runs the protected check FIRST, then the engine. Both the cycle reviewer and `evolve selfcheck build` run it; a pointer pin, a source-scan pin and a behavioral pin keep every root on it.
2. **Rename detection is off on both sides** (architecture review M1). With it on, `git diff --name-only` prints only a rename's new path, so a protected file moved to an unprotected name passed P2 as well.
3. **P2's refusal recovers into BUILD.** A new `control-plane-rebuild` handler sits ahead of `precondition-reaudit`, within the constant ship-recovery budget of 2. ship→build is a legal edge. On re-entry the floor names the path; ship passes the builder nothing else.

## Pins

- `core/build_floor_protected_test.go::TestProtectedSurfaceFailures_NamesEveryProtectedPath`, `TestProtectedSurfaceFloorChecks_FailsOpenWithoutAWorktree`
- `core/build_floor_protected_integration_test.go::TestProtectedSurfaceFloorChecks_RefusesTheCycle1689Shape` (a committed protected change is named, alone; the HEAD fallback names an uncommitted one)
- `core/build_floor_protected_integration_test.go::TestProtectedSurfaceFloorChecks_SeesARenameOutOfTheSurface` (red without `--no-renames`)
- `phases/ship/integrity_test.go::TestVerifyNoControlPlaneEdits_RejectsARenameOutOfTheSurface` (red without `--no-renames`)
- `core/orchestrator_recovery_test.go::TestRunCycle_ShipControlPlaneViolation_RebuildsThenShips` (end-to-end ship → build → audit → ship; with the handler disabled it shows the old loop, build=1 audit=2)
- `cmd/evolve/cmd_selfcheck_protected_integration_test.go::TestProductionBuildFloorChecks_RefusesAProtectedFileThroughTheRealManifest` (the real manifest, the protected finding first, and the engine's placeholder finding too)
- `cmd/evolve/cmd_selfcheck_test.go::TestBuildFloorRoots_ComposeOnlyThroughTheProductionFloor`, `TestSelfcheckSeam_DefaultsToBuildFloorChecks`
- `router/recovery_test.go::TestRecover_Branches` (the `control-plane-rebuild` row)

## Not fixed here (follow-up F38, console-owned)

- **P1's blind spot for shell tools.** Parsing shell commands for writes is heuristic. The floor now catches the result deterministically, whatever channel produced it.
- **A stale base after some fleet rebases** (architecture review m1). A clean rebase of an explanation-version-0 cycle does not persist the new base, so a later return to Build would diff against the old base. Fresh cycles stamp a version above 0.
- **A blind rebuild when `workflow.build_floor` is off** (m3). The rebuilt builder then gets no path.
- **`evolve selfcheck build` diffs against HEAD, not the cycle base** (m5).
- **The manifest in a leaf package** (m6). `guards/integrity_surface.go` imports only the standard library. Moving it would let `core` and `inboxbatch` call membership directly, retiring this injection and the F35c port together.
- **Items whose natural fix needs a protected file** (the triage-derives-beyond-the-record residual, F32). The floor turns them into an in-phase stop instead of a wasted audit, but the lane still cannot ship them.
