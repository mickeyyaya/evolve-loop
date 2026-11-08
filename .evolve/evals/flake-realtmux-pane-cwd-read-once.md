---
score_cap:
  - criterion: "TestRealTmux_NewSessionInBindsPaneCwd polls pane_current_path past transient empty reads"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2230_001 ./acs/cycle2230"
  - criterion: "the assertion still fails with the current message when pane_current_path stays empty"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2230_002 ./acs/cycle2230"
  - criterion: "the test passes -race -count=100 with -tags integration under transient empty reads"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2230_003 ./acs/cycle2230"
---

# Eval: TestRealTmux_NewSessionInBindsPaneCwd polls pane_current_path

> Pins the fix for the flaky-test census rank 7 (docs/research/flaky-tests-2026-10.md F4.7).
> The test read `#{pane_current_path}` once after new-session; on Linux tmux can report an empty
> value for some milliseconds. Each predicate runs the real integration test with a shim `tmux` that
> answers the pane path read with an empty value on two of every three reads. Source incident:
> run 38919986816, ubuntu Go 1.23. Cycle 2230.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| poll-past-empty | transient empty reads do not fail the test | 6/10 | `go test -tags acs -run TestC2230_001 ./acs/cycle2230` |
| fail-message-kept | persistent empty value still fails with the current message after repeated reads | 7/10 | `go test -tags acs -run TestC2230_002 ./acs/cycle2230` |
| race-count-100 | `-race -count=100 -tags integration` passes | 6/10 | `go test -tags acs -run TestC2230_003 ./acs/cycle2230` |
