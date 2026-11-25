---
score_cap:
  - criterion: "No bridge test builds an Engine that reaches the real log show poll"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestBridgeTestEnginesInjectSandboxFakes$' ./internal/bridge"
  - criterion: "The test engine answers the fence and the reports without a spawn"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestNewTestEngine_Sandbox' ./internal/bridge"
  - criterion: "The guard flags a bare NewEngine call in every test file, tagged or not"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestRealSandboxEngines_FlagsADirectNewEngineWithoutBothFakes$' ./internal/bridge"
---

# Eval: Bridge test engines inject the sandbox fakes

> Pins the rule that each bridge test engine gets a fake `SandboxReports` and a fake `SandboxFence`.
> `NewEngine` falls back to the real `log show` poll when both are nil, and the poll waits up to 60 s on macOS.
> Source: inbox item bridge-test-engine-fakes-sandbox-fence, cycle 2559.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| guard | No test engine reaches the real poll | 6/10 | `go test -run TestBridgeTestEnginesInjectSandboxFakes` |
| behavior | The fakes answer on the first read | 7/10 | `go test -run TestNewTestEngine_SandboxReadsNeverSpawnABinary` |
| detector | The guard finds the bad shape | 5/10 | `go test -run TestRealSandboxEngines_FlagsADirectNewEngineWithoutBothFakes` |
