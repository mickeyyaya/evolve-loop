---
score_cap:
  - criterion: "Every resumable reason is a valid checkpoint reason"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestResumableReasonsAreValidCheckpointReasons$' ./internal/checkpoint"
  - criterion: "An overlay mutant adding a bogus resumable key turns the link test red"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2323_002 ./acs/cycle2323"
---

# Eval: Link the checkpoint reasons and the resumable reasons

> Pins the link between `checkpoint.Reason` and `core.resumableReasons`. The core package cannot import checkpoint, so the test lives in checkpoint and reads the real map through `core.ResumableReasons()`. Source task: inbox item checkpoint-reasons-one-list (cycle 2323).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| link-test | resumable implies valid | 6/10 | `go test -run TestResumableReasonsAreValidCheckpointReasons ./internal/checkpoint` |
| overlay-mutant | bogus key is caught | 7/10 | `go test -tags acs -run TestC2323_002 ./acs/cycle2323` |
