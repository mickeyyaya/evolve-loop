---
score_cap:
  - criterion: "A lock wait over the policy bound stops a widening of the window, and a wait at or under the bound gives the old result"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2676_00[123]' ./acs/cycle2676"
  - criterion: "The shadow sample line records a widening that the lock wait held, and the production caller feeds the sampler the lock wait of the run signal logs"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2676_004' ./acs/cycle2676"
  - criterion: "wave status shows the total lock wait of the wave from the run signal logs and the global log, one count for each event"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2676_00[567]' ./acs/cycle2676"
---

# Eval: the lock wait feeds the fleet window

> Pins the rule that the host-adaptive fleet window reads the host verification lock wait as a pressure input. Source incident: wave 186 (2026-11-21), where HOST_VERIFY_LOCK_WAIT reached only runs/cycle-N/signals.ndjson, so wave-level readers saw 0, while waits ran from 280 to 701 s and one 900 s run went unserialized.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| window-rule | over the bound no widening; at or under it the old result; a decrease is never stopped | 4/10 | `go test -tags acs -run 'TestC2676_00[123]' ./acs/cycle2676` |
| shadow-and-wiring | shadow line names the held widening; the production caller feeds the wait | 5/10 | `go test -tags acs -run TestC2676_004 ./acs/cycle2676` |
| wave-status | total lock wait of the wave, each event once | 5/10 | `go test -tags acs -run 'TestC2676_00[567]' ./acs/cycle2676` |
