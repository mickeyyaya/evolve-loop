## top_n (commit to THIS cycle)
- fix-inserted-phase-worktree-dispatch: Fix advisor mint template + abort-cleanup preservation — priority=H, evidence=inbox:inserted-phase-empty-worktree-provisioning+abort-cleanup-destroys-uncommitted-worktree+carryover:cycle-280-failed-test-amplification, source=scout+carryover+inbox
- adversarial-fault-injection-suite: Build fault-injection test suite for all 3 driver families — priority=H, evidence=scout-report§AdversarialCapabilityGap, source=scout
- coverage-push-core-and-lower-packages: Push internal package coverage from 88.1% toward 93% — priority=M, evidence=scout-report§CoverageGap, source=scout

## deferred (carry to NEXT cycle's carryoverTodos)
- cmd-evolve-deep-coverage: Cobra binary-invoke harness for cmd/evolve 63.5% gap — priority=M, defer_reason=Requires dedicated binary-invoke cobra harness session; benefit materialises after P0 fix is in place
- bridge-tmux-real-controller-coverage: RealTmuxController 0% coverage — priority=L, defer_reason=Requires live OS tmux binary; legitimate integration-test-tagged effort best isolated from unit coverage push
- full-98pct-coverage-milestone: Close remaining 5pp gap to reach 98% — priority=M, defer_reason=10pp gap is too large for one cycle; realistic milestone this cycle is 93%; remaining gap concentrates in cmd/evolve and orchestrator integration paths

## dropped (rejected with reason)
- inserted-phase-empty-worktree-provisioning: Advisor-inserted phases dispatch with empty worktree — reason=subsumed-by:fix-inserted-phase-worktree-dispatch (Task 1 fully addresses the provisioning seam)
- abort-cleanup-destroys-uncommitted-worktree: Abort-cleanup deletes uncommitted worktree on guard abort — reason=subsumed-by:fix-inserted-phase-worktree-dispatch (Task 1 addresses the abort-cleanup preservation path)

## carryoverTodos warnings (if any)
- cycle-280-failed-test-amplification: defer_count=0; operator P0 — reserved slot in top_n (subsumed by fix-inserted-phase-worktree-dispatch which directly resolves the cycle-280 failure root cause)

## Inbox Processing Log
