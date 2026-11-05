---
score_cap:
  - criterion: "sync-main passes on a tree whose only change is the loop's quarantine move, and the item stays in quarantine"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2169_001|TestC2169_002|TestC2169_004' ./acs/cycle2169"
  - criterion: "A real edit or deletion of a tracked file is still refused, and a refusal commits and merges nothing"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2169_003|TestC2169_005' ./acs/cycle2169"
  - criterion: "Every function of internal/inboxstamps stays at 100% coverage"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2169_006' ./acs/cycle2169"
  - criterion: "refuseDirtBeyondInboxStamps stays at 100% coverage under the sync-main tests of cmd/evolve"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2169_007|TestC2169_008' ./acs/cycle2169"
  - criterion: "An origin edit of the moved item aborts the merge cleanly and keeps the move"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2169_009' ./acs/cycle2169"
---

# Eval: sync-main accepts a quarantine move

> Pins the rule that evolve sync-main treats the loop's own quarantine move of a tracked inbox item as loop-owned inbox state and not as foreign dirt. Source incident: the cycle 2148 move of bridge-factory-f3 stopped the boundary at sync-main until the console recorded the move by hand (commit 3c0090689).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| quarantine-passes | the move does not refuse; the item stays in quarantine; HEAD records the move | 3/10 | `go test -tags acs -run 'TestC2169_001\|TestC2169_002\|TestC2169_004' ./acs/cycle2169` |
| real-dirt-refused | a source edit, a source deletion, a deletion without a copy and a rewritten copy are refused | 4/10 | `go test -tags acs -run 'TestC2169_003\|TestC2169_005' ./acs/cycle2169` |
| full-coverage | each inboxstamps function at 100% | 6/10 | `go test -tags acs -run 'TestC2169_006' ./acs/cycle2169` |
| classify-failure-covered | refuseDirtBeyondInboxStamps at 100% under `-run SyncMain`; a non-git root exits 1 naming the read | 5/10 | `go test -tags acs -run 'TestC2169_007\|TestC2169_008' ./acs/cycle2169` |
| origin-edits-moved-item | the merge aborts, no MERGE_HEAD, the quarantine copy and the move commit stay | 6/10 | `go test -tags acs -run 'TestC2169_009' ./acs/cycle2169` |
