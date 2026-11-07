---
score_cap:
  - criterion: "The claude-span e2e passes when the host lags before idle_reached and after idle_reached"
    max_if_missing: 4
    evidence: "cd go && go test -race -count=1 -run '^TestChannelE2E_RealFixtures_ClaudeSpan_UnderHostLag$' ./internal/bridge/"
  - criterion: "The original claude-span e2e still passes on an idle host"
    max_if_missing: 5
    evidence: "cd go && go test -race -count=1 -run '^TestChannelE2E_RealFixtures_ClaudeSpan$' ./internal/bridge/"
  - criterion: "The TempDir cleanup joins the producer and driver, the driver is cancellable and no short time.After guards the driver"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2209_00[456]' ./acs/cycle2209"
  - criterion: "The package note names the driver, producer and supervisor race with its reproducer"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2209_007' ./acs/cycle2209"
---

# Eval: Channel e2e claude-span under host load

> Pins the fix for `TestChannelE2E_RealFixtures_ClaudeSpan` failing once at host load 60 to 70 (cycle 2209).
> The fault is in the test harness: a supervisor clock that counts polls, a 3 s wall-clock driver wait,
> and a producer and a driver that nothing joins before the TempDir removal.
> The reproducer `TestChannelE2E_RealFixtures_ClaudeSpan_UnderHostLag` injects the lag that a loaded host adds.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| lag-survival | Both lag transitions pass | 4/10 | `go test -race -run '^TestChannelE2E_RealFixtures_ClaudeSpan_UnderHostLag$' ./internal/bridge/` |
| idle-preservation | Original test still passes | 5/10 | `go test -race -run '^TestChannelE2E_RealFixtures_ClaudeSpan$' ./internal/bridge/` |
| goroutine-join | Cleanup joins both goroutines | 5/10 | `go test -tags acs -run 'TestC2209_00[456]' ./acs/cycle2209` |
| race-named | Package note names the race | 7/10 | `go test -tags acs -run 'TestC2209_007' ./acs/cycle2209` |
