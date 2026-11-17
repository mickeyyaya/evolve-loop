---
score_cap:
  - criterion: "A usage reading at the limit stops the loop at its next boundary with no operator brake and no console"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC2447_001 ./acs/cycle2447"
  - criterion: "A new evolve agent run seat past the limit is refused and the text names the limit, the reading and the reset"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC2447_002 ./acs/cycle2447"
  - criterion: "An unreadable reading (corrupt windows file, or a probe that recorded an error) is never under the limit, for the loop and for a seat"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2447_003 ./acs/cycle2447"
  - criterion: "cli_health.usage_hold_percent is a policy key and an absent key means the brake is off"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2447_004 ./acs/cycle2447"
---

# Eval: usage brake as policy

> Pins inbox item usage-brake-as-policy (cycle 2447). The loop and each new seat read the usage window that the usage probe already records in `.evolve/usage-windows.json`, and they stop or refuse when the highest live reading reaches the operator limit `cli_health.usage_hold_percent`. An unreadable reading fails closed. An absent key keeps today's behavior, so a plane that sets no limit sees no change.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| loop-boundary | At-limit reading stops the loop with stop_reason `loop_usage_hold` and one `loop.halt` signal | 4/10 | `go test -tags acs -run TestC2447_001 ./acs/cycle2447` |
| seat-refusal | Seat past the limit exits 3, launches nothing, names limit, reading and reset | 4/10 | `go test -tags acs -run TestC2447_002 ./acs/cycle2447` |
| fail-closed | Corrupt file or error-only observation holds the loop and the seat, and the text names `usage-windows.json` | 5/10 | `go test -tags acs -run TestC2447_003 ./acs/cycle2447` |
| policy-key | `usage_hold_percent` loads from the `cli_health` block, absent is 0 | 6/10 | `go test -tags acs -run TestC2447_004 ./acs/cycle2447` |
