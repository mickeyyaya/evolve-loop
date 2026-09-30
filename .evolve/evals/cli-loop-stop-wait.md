---
score_cap:
  - criterion: "loop-stop --wait returns 0 once no run lease is live and a live lease past --timeout exits 1 naming the run with the brake kept"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1786_00[1-5]' ./acs/cycle1786"
  - criterion: "loop-stop --wait --release exits 10 and leaves the brake"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1786_006' ./acs/cycle1786"
  - criterion: "sync-main and loop-stop --wait reach the same liveness verdict on every lease through one shared runlease function"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1786_017' ./acs/cycle1786"
  - criterion: "--wait prints the live run and its phase whenever they change, and runtime-reference boundary step 1 runs a loop-stop that waits"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1786_02[23]' ./acs/cycle1786"
---

# Eval: evolve loop-stop --wait

> Pins `evolve loop-stop --wait [--timeout D]` blocking on the shared runlease liveness check (fresh heartbeat and live owner pid), cycle 1786. The audit of the first build (round 1, M1 and M3) found sync-main still on its own inline check with a different pid probe (a lease owned by another user's live process read live to loop-stop and idle to sync-main), `--wait` silent until it returned, and runtime-reference's boundary step 1 still telling the operator to tail the loop log. The later rows pin those behaviors.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| wait-liveness | wait returns/timeouts by lease liveness | 6/10 | `go test -tags acs -run TestC1786_00[1-5] ./acs/cycle1786` |
| wait-release-conflict | --wait --release exits 10 | 7/10 | `go test -tags acs -run TestC1786_006 ./acs/cycle1786` |
| shared-liveness | sync-main and loop-stop --wait agree on every lease fixture | 8/10 | `go test -tags acs -run TestC1786_017 ./acs/cycle1786` |
| wait-progress-and-doc | run/phase line on change; boundary step 1 command waits | 6/10 | `go test -tags acs -run TestC1786_02[23] ./acs/cycle1786` |
