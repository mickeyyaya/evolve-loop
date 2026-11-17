---
score_cap:
  - criterion: "the gc/success contract case leaves no go telemetry under the test HOME"
    max_if_missing: 6
    evidence: "cd go && go test -tags integration -count=1 -run '^TestCLIContract_GCSuccessLeavesNoGoTelemetryUnderTheTestHome$' ./cmd/evolve/"
  - criterion: "the gc contract case survives fifty repeats"
    max_if_missing: 7
    evidence: "cd go && go test -tags integration -count=50 -run '^TestCLIContract_EachCoreVerbPrintsOneEnvelope$/^gc$' ./cmd/evolve/"
---

# Eval: gc contract case and Go telemetry

> A Go child process that `evolve gc` starts writes telemetry under the scratch HOME of the contract test. The writer can still hold the directory when `t.TempDir` cleans up, so the case fails with "directory not empty". Cycle 2462 pins both the cause and the symptom.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cause | no telemetry under the test HOME | 6/10 | `go test -tags integration -run '^TestCLIContract_GCSuccessLeavesNoGoTelemetryUnderTheTestHome$' ./cmd/evolve/` |
| symptom | 50 repeats of the gc case pass | 7/10 | `go test -tags integration -count=50 -run '^TestCLIContract_EachCoreVerbPrintsOneEnvelope$/^gc$' ./cmd/evolve/` |
