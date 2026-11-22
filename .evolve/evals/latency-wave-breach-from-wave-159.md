---
score_cap:
  - criterion: "The wave latency report names the time that queued at the host verification lock in each phase, apart from gate work"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2598_001|TestC2598_004|TestC2598_005' ./acs/cycle2598"
  - criterion: "A wave whose host time is queueing at the lock is not a gate breach, and the check reports the lock wait share"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2598_002' ./acs/cycle2598"
  - criterion: "Gate work above the ceiling still breaches, and the reason names the lock wait share"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2598_003' ./acs/cycle2598"
---

# Eval: Wave latency separates lock queueing from gate work

> Pins the cause of the wave latency breach that the inbox item latency-wave-breach-from-wave-159 reports. The host gate share counted each minute that a run waited at the host verification lock as gate work. Cycle 2596 spent 18.5 of 37 host minutes of its build phase in two waits. The lock wait signal carries `wait_ms` from wave 186 on. Source incident: cycle 2579 failed on an empty diff; cycle 2598 continues it.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| attribution | each phase reports its lock wait; a bad or out-of-span wait counts for nothing; a wait never exceeds the host time | 7/10 | `go test -tags acs -run 'TestC2598_001\|TestC2598_004\|TestC2598_005' ./acs/cycle2598` |
| queueing is not gate work | the gate share leaves the lock wait out and the verdict reports it | 8/10 | `go test -tags acs -run 'TestC2598_002' ./acs/cycle2598` |
| no hidden breach | real gate work above the ceiling still breaches and names the lock wait share | 8/10 | `go test -tags acs -run 'TestC2598_003' ./acs/cycle2598` |
