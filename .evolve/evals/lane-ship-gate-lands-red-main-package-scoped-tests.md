---
score_cap:
  - criterion: "The lane pre-ship repo-contract gate runs a CI-equivalent selection and blocks an UNTOUCHED package's red with the named REPO_CONTRACT_GATE ship error"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run 'TestRepoContractGate_(UsesRepoWideSuite|RepoWideSuiteBlocksUntouchedPackageRegression)' ./internal/phases/ship"
  - criterion: "Each wave boundary reads origin/main's completed check-runs and HALTs with a named LOOP_HALT reason before any lane is dispatched"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run 'TestMainCIAtWaveBoundary_(RedCheckRunHalts|RedHaltsBeforeAnyLaneDispatch)' ./cmd/evolve"
  - criterion: "A green main proceeds and an unavailable check-run API is warned/classified distinctly — never read as green, never as a false red"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run 'TestMainCIAtWaveBoundary_(GreenProceeds|UnavailableWarnsWithoutFalseRed)' ./cmd/evolve"
  - criterion: "The added-test backstop and the classified real-red/infra split survive the wider selection — the new gate is not bought by weakening the old ones"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestRepoContractGate_(NewlyAddedFailingTestBlocksShip|EnforceRedFailsWithDedicatedCode|PersistentAmbiguityIsInfraClassedExactlyTwoRuns|TransientFailureRetriesOnceThenShips)' ./internal/phases/ship"
  - criterion: "The lane pre-ship repo-contract gate AND its added-test backstop execute in the LANE WORKTREE — the tree the lane merges FROM — never the project root"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run 'TestRepoContractGate_(ScansLaneWorktreeNotProjectRoot|FallsBackToProjectRootWhenWorktreeEmpty)' ./internal/phases/ship"
  - criterion: "A red that exists only in the pre-merge project root does not false-block a green lane — a red main halts the wave, it never fails the lane"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run 'TestRepoContractGate_ProjectRootRedDoesNotBlockAGreenWorktree' ./internal/phases/ship"
---

# Eval: Lane ship gate runs only the packages it touched, so a contract change can land a RED main

> A lane's pre-ship repo-contract gate selects a FIXED four-package pack
> (`repoContractPackages` in `go/internal/phases/ship/repocontract.go:55-63`).
> It structurally cannot see a test break in a package the lane never touched,
> and its added-test backstop only inspects newly STAGED `_test.go` files, so an
> already-tracked test asserting a superseded contract is invisible to both.
> In cycles 1657/1659 lane commit `4db205a8` landed the new
> `triage-empty-commitment-claimable-work` stop; `internal/deliverable`'s
> ADR-0100 `declared_effects_e2e_test.go` still asserted the old contract and
> went red on main's `build+test` job for every subsequent main commit until
> hotfix #590 re-pinned the test. The loop never noticed: nothing at the wave
> boundary reads main's CI, so lanes kept stacking ships on a red main
> (ADR-0072: a red main is SYSTEM-fail evidence). This eval pins both halves —
> a CI-equivalent pre-ship selection with a named ship-error code, and a
> red-main halt at the wave boundary that fires BEFORE any launcher exists —
> plus the false-RED direction, because "GitHub is unreachable" is not evidence
> that main is red and must never stop the batch on its own.

> **Round-2 addendum (cycle-1673 audit H1).** Widening the selection to `./...`
> was necessary and not sufficient: `runRepoContractGate` derives its module dir
> from `req.ProjectRoot` (`repocontract.go:208`, fed by `ship.go:188`), while the
> orchestrator populates `ProjectRoot` and `Worktree` as two different values for
> a fleet lane (`cyclerun_dispatch.go:138,140`). The gate therefore ran CI's
> selection against the tree the lane merges INTO, so the 1657/1659 failure mode
> survived — at the new cost of a full repo suite per ship. Round 1's five green
> predicates could not catch it because each passed one directory as both roots.
> The two `score_cap` entries above pin the root, in both directions: a
> worktree-only red must block, and a project-root-only red must not.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| ci-equivalent-preship | Untouched-package red blocks the lane ship with `REPO_CONTRACT_GATE` | 9/10 | `go test -run 'TestRepoContractGate_(UsesRepoWideSuite\|RepoWideSuiteBlocksUntouchedPackageRegression)' ./internal/phases/ship` |
| red-main-halt-before-dispatch | Completed failing origin/main check-run halts the wave with a named reason, zero lanes launched | 9/10 | `go test -run 'TestMainCIAtWaveBoundary_(RedCheckRunHalts\|RedHaltsBeforeAnyLaneDispatch)' ./cmd/evolve` |
| outcome-classification | Green proceeds; unavailable API warns distinctly (no false red, no silent green) | 8/10 | `go test -run 'TestMainCIAtWaveBoundary_(GreenProceeds\|UnavailableWarnsWithoutFalseRed)' ./cmd/evolve` |
| no-weakening | The added-test backstop and the real-red vs infra classification still hold | 7/10 | `go test -run 'TestRepoContractGate_(NewlyAddedFailingTestBlocksShip\|EnforceRedFailsWithDedicatedCode\|PersistentAmbiguityIsInfraClassedExactlyTwoRuns\|TransientFailureRetriesOnceThenShips)' ./internal/phases/ship` |
| worktree-rooted-gate | The gate and the added-test backstop run in the lane worktree, not the project root | 9/10 | `go test -run 'TestRepoContractGate_(ScansLaneWorktreeNotProjectRoot\|FallsBackToProjectRootWhenWorktreeEmpty)' ./internal/phases/ship` |
| no-false-red-on-lane | A project-root-only red does not block a green lane worktree | 8/10 | `go test -run 'TestRepoContractGate_ProjectRootRedDoesNotBlockAGreenWorktree' ./internal/phases/ship` |
