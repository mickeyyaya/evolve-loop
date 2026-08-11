---
score_cap:
  - criterion: "All 6 rescue commits (a94bad9..160b135) are cherry-picked onto the cycle worktree based at 81d2c2f with no content drift"
    max_if_missing: 8
    evidence: "git log --oneline 81d2c2f..HEAD | grep -c 'worktree-build'"
  - criterion: "go test ./... PASS and gofmt -s produces no diff on staged tree"
    max_if_missing: 8
    evidence: "cd go && go test ./... 2>&1 | tail -5 && gofmt -s -l . | grep -v vendor | head -5"
  - criterion: "build-report.md describes commits as staged in worktree (not landed on main); cites 81d2c2f as base and 160b135 as rescue tip"
    max_if_missing: 9
    evidence: "grep 'staged in worktree' .evolve/runs/cycle-237/build-report.md && grep '81d2c2f' .evolve/runs/cycle-237/build-report.md && grep '160b135' .evolve/runs/cycle-237/build-report.md"
  - criterion: "Audit-phase binary-churn leak recovery test passes (primary rescue payload)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestOrchestrator_AuditLeakRecover' ./internal/core/"
  - criterion: "Ship-closure idempotency test passes (post-push re-entry ships report-only)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestShip_PostPush_Idempotent_CorrectReportOnly|TestShip_PinPostCommitSha' ./internal/phases/ship/"
  - criterion: "Failure supervision tree tests pass (cycle-level bridge failure classification)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestErrCycleLevelFailure_WrapsCauseForErrorsIs|TestOrchestrator_BridgeExhaustion_CycleLevelFailure|TestLoop_CycleLevelFailureContinues' ./internal/core/ ./cmd/evolve/"
  - criterion: "Landed feature evals are tracked by git (no orphan evals)"
    max_if_missing: 5
    evidence: "git ls-files --error-unmatch .evolve/evals/ship-closure-idempotency.md .evolve/evals/cycle-level-bridge-failure.md .evolve/evals/phase-boundary-checkpoint.md .evolve/evals/audit-phase-leak-recover.md"
---

# Eval: Cherry-pick rescue/cycle-236-green @ 160b135 onto main (cycle 237 landing)

> Pins the cycle-237 landing operation: cherry-picking 6 commits from
> `rescue/cycle-236-green` @ `160b135` onto main, bringing in
> failure-supervision-tree step 3, ship-closure-idempotency, and
> audit-phase-leak-recover. Content was already audit-grade (ACS 71/71 green,
> go test PASS, gofmt -s clean) on the rescue branch. Cycle 236 failed ONLY
> because the build report claimed the commits were "landed on main" while
> they were staged in a worktree — the root wording defect this eval catches.
>
> Prior killers confirmed cleared before this cycle:
> - `expected_ship_sha` pin: DELETED from state.json (TOFU re-pin at ship)
> - Audit leak-recover code: PRESENT in rescue branch orchestrator.go line 1669
>
> This eval is SHA-free for behavioral criteria (the rescue branch is deleted
> post-landing); wording criteria explicitly cite both the base SHA (81d2c2f)
> and the rescue tip (160b135) to enforce the correct lifecycle description.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cherry-pick-complete | All 6 `[worktree-build]` commits staged | 8/10 | `git log --oneline 81d2c2f..HEAD \| grep -c 'worktree-build'` |
| tests-green | go test PASS + gofmt -s clean on staged tree | 8/10 | `go test ./... && gofmt -s -l .` |
| report-wording | build-report cites "staged in worktree" + exact SHAs | 9/10 | `grep 'staged in worktree' .../build-report.md` |
| audit-leak-recover | Binary churn during audit → discard + continue | 7/10 | `go test -run TestOrchestrator_AuditLeakRecover ./internal/core/` |
| ship-idempotency | Post-push correction = report-only, no re-ship | 7/10 | `go test -run 'TestShip_PostPush_Idempotent_CorrectReportOnly\|TestShip_PinPostCommitSha' ./internal/phases/ship/` |
| batch-survival | Bridge exhaustion classifies cycle-level, not batch-fatal | 6/10 | `go test -run 'TestErrCycleLevelFailure_WrapsCauseForErrorsIs\|TestOrchestrator_BridgeExhaustion_CycleLevelFailure\|TestLoop_CycleLevelFailureContinues' ./internal/core/ ./cmd/evolve/` |
| eval-integrity | Feature eval files tracked in git | 5/10 | `git ls-files --error-unmatch .evolve/evals/<4 files>` |

The report-wording criterion (9/10 max_if_missing) is the HIGHEST-WEIGHT
criterion because the cycle-236 killer was precisely this: a build report
that described staged worktree commits as "landed on main", causing the
auditor to flag a fabricated ship claim. The `grep 'staged in worktree'`
combined with SHA presence is the direct structural check for the recurrence.

## Negative and Edge Cases

- **Negative:** build-report.md that says "landed on main" or "commits on main" (not just "staged") MUST cause max_if_missing=9 score cap to trigger.
- **Negative:** If `go test -run TestOrchestrator_AuditLeakRecover` exits 0 with "no tests to run" (test deleted), the `grep 'discarded binary rebuild churn' go/internal/core/orchestrator.go` evidence in the companion eval (`audit-phase-leak-recover.md`) must catch it.
- **Edge:** If cherry-pick produced merge conflicts and the builder force-resolved them, the content-drift check (git log commit count) will still pass — auditor should also verify functional tests, not just commit count.
