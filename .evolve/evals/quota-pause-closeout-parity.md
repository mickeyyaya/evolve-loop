---
score_cap:
  - criterion: "the cycle-run root exits 5 and writes no failure-walk lifecycle on a quota wall"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestCycleRunRoot_QuotaWallPausesLikeResumeAndChargesNobody$'"
  - criterion: "a non-quota cycle-level failure still exits 1 and walks the inbox failure lifecycle"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestCycleRunRoot_NonQuotaCycleFailureStillWalksTheFailureOutcome$'"
---

# Eval: quota-pause-closeout-parity

> Pins that the cycle-run root (every fleet lane's entrypoint) treats a quota wall as a resumable pause like the sequential and resume roots, not as a charged failure; seen at cycle 1709 (wave 14: failure closeout, "wave 0: 0/2 lanes ok").

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| quota-pause | exit 5, no failure walk | 5/10 | `go test -run TestCycleRunRoot_QuotaWall...` |
| non-quota-negative | plain failure still walks | 6/10 | `go test -run TestCycleRunRoot_NonQuota...` |
