---
score_cap:
  - criterion: "the stageevidence guard test passes on the current tree and names the F-item that will add the routed source"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run '^TestStageEvidenceRoutedSourceGuard$' ./internal/stageevidence 2>&1 | grep -q -- '--- PASS: TestStageEvidenceRoutedSourceGuard' && go test -count=1 -v -run '^TestStageEvidenceRoutedSourceGuard$' ./internal/stageevidence 2>&1 | grep -q F4"
  - criterion: "the guard fails when an emit site fills Outcome.Routed or the held-proposal item, and ignores test files"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2620_00[2-6]_' ./acs/cycle2620"
---

# Eval: A guard fails when stage evidence gains a routed-item source but the scorer's conflict class stays unreachable

> Pins the pending-source contract of the stage-evidence scorer. `stageevidence.Outcome.Routed` and the held-proposal `item` field have no producer today (the K2c-K2e change, PR #1682), so the scorer's conflict class cannot fire. The guard turns red on the first change that adds a producer. Source incident: the architecture review of PR #1682 rated this gap CRITICAL, and cycle 2620 carries the inbox item.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| guard-green-now | The guard passes on the current tree and names F4 | 6/10 | `go test -run '^TestStageEvidenceRoutedSourceGuard$' ./internal/stageevidence` |
| guard-red-on-source | A fixture emit site that fills the source fails the guard; a clean fixture and a test file pass | 8/10 | `go test -tags acs -run 'TestC2620_00[2-6]_' ./acs/cycle2620` |
