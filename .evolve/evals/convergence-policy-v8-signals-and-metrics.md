---
score_cap:
  - criterion: "the ten convergence codes beside CONVERGENCE_RUNG are registered to the convergence module with a doc that names the fields of spec section 9"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestRegistry_Convergence' ./internal/signalcenter && go test -tags acs -count=1 -run 'TestC2328_001' ./acs/cycle2328"
  - criterion: "docs/architecture/signal-codes.md is regenerated and its generator check passes"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2328_002' ./acs/cycle2328"
  - criterion: "evolve convergence metrics prints the rounds to land, the deferred-then-closed share and the falsification drop rate, as text and with --json"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestMeasure|^TestCmd_ConvergenceMetrics' ./internal/convergence ./cmd/evolve && go test -tags acs -count=1 -run 'TestC2328_003' ./acs/cycle2328"
  - criterion: "the metrics verb refuses a missing file, an unreadable file and an unknown flag with exit 10 and no stdout"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2328_004' ./acs/cycle2328"
  - criterion: "every function of metrics.go, cmd_convergence_metrics.go and runConvergence has 100% coverage"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2328_005' ./acs/cycle2328"
---

# Eval: Convergence V8, signals and the metrics readout

> Pins plan component V8 of the convergence policy (docs/plans/convergence-policy-2026-10.md, section 6). The convergence package writes `CONVERGENCE_*` reason codes that the signal registry did not hold. The registry gains the ten codes that the package emits besides `CONVERGENCE_RUNG`, and `evolve convergence metrics` reads rounds files and prints the rounds to land, the deferred-then-closed share and the falsification drop rate. Source: inbox item `convergence-policy-v8-signals-and-metrics`, cycle 2328.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| registry | ten codes registered with field docs | 4/10 | `TestRegistry_Convergence*` and `TestC2328_001` |
| generated doc | signal-codes.md in sync | 5/10 | `TestC2328_002` |
| readout | three metrics, text and JSON | 4/10 | `TestMeasure*`, `TestCmd_ConvergenceMetrics*`, `TestC2328_003` |
| negative | bad invocation exits 10 | 6/10 | `TestC2328_004` |
| coverage | 100% on new and changed functions | 5/10 | `TestC2328_005` |
