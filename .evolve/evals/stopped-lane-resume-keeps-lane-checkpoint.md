---
score_cap:
  - criterion: "A lane paused on quota and stopped during a foreground resume is still listed as resumable by the wave"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2298_00[123]' ./acs/cycle2298"
  - criterion: "Granting one named paused cycle among two grants only that cycle and refuses an unpaused or malformed request"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2298_00[45]' ./acs/cycle2298"
  - criterion: "A named grant starts no cycle run"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2298_006' ./acs/cycle2298"
---

# Eval: A stopped lane resume keeps its lane checkpoint

> Pins three behaviors of the lane resume path. The stop of a resumed lane run rewrites the checkpoint reason to operator-requested, and the lane filter accepted only quota-likely, so the wave lost the cycle. The operator also had no verb to grant one named paused cycle without a foreground run. Source incident: cycle 2284 (build rejected on an unrelated integration test), retried in cycle 2298.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| stop-keeps-checkpoint | the lane stays listed after a stopped foreground resume | 5/10 | `go test -tags acs -run 'TestC2298_00[123]' ./acs/cycle2298` |
| named-grant | one named cycle is granted, an unpaused or malformed request grants nothing | 6/10 | `go test -tags acs -run 'TestC2298_00[45]' ./acs/cycle2298` |
| grant-starts-no-run | the grant creates no file and moves no phase | 6/10 | `go test -tags acs -run 'TestC2298_006' ./acs/cycle2298` |
