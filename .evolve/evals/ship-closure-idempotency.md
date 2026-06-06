---
score_cap:
  - criterion: "Post-push ship re-dispatch is report-only (no new commit/push, no HEAD_MOVED dead-end); pre-push correction still full-ships"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestShip_PostPush_Idempotent_CorrectReportOnly|TestShip_PrePush_CorrectionStillFullShip' ./internal/phases/ship/"
  - criterion: "Worktree go/evolve binary churn is discarded before staging; real source changes preserved"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestShip_BinaryChurnDiscarded|TestShip_SourceChangesPreserved' ./internal/phases/ship/"
  - criterion: "expected_ship_sha pinned from the binary as committed at HEAD, not the on-disk blob"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestShip_PinPostCommitSha' ./internal/phases/ship/"
---

# Eval: Ship closure idempotency (cycle-233 landing defects)

> Pins the three ship-closure defects that made cycle 233's fully-successful
> push batch-fatal (inbox `2026-06-06T03-27-08Z-ship-closure.json`):
> (1) a deliverable-contract correction re-dispatch AFTER the push re-ran the
> full ship and dead-ended on AUDIT_BINDING_HEAD_MOVED — HEAD was ship's own
> commit; (2) `git -C <worktree> add -A` swept unaudited go/evolve binary
> churn into the cycle commit, breaking audit AC5 soundness (the build
> phase's recoverBuildLeak pattern was never applied to ship); (3)
> `expected_ship_sha` was pinned from a pre-commit blob, forcing two manual
> state.json deletions this campaign (SELF_SHA_TAMPERED). All evals are
> real-git behavioral tests asserting on history, remote refs and state.json.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| post-push-idempotent | re-dispatch = report-only; stale binding ≠ short-circuit | 4/10 | `go test -run 'TestShip_PostPush_Idempotent_CorrectReportOnly\|TestShip_PrePush_CorrectionStillFullShip' ./internal/phases/ship/` |
| churn-discard | go/evolve churn never reaches the cycle commit; source edits do | 5/10 | `go test -run 'TestShip_BinaryChurnDiscarded\|TestShip_SourceChangesPreserved' ./internal/phases/ship/` |
| post-commit-pin | expected_ship_sha = sha256(HEAD:go/evolve content) | 6/10 | `go test -run TestShip_PinPostCommitSha ./internal/phases/ship/` |
