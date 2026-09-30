---
score_cap:
  - criterion: "A composed-tree gate decline at RUNG 0 (compositionCarryForward) registers one coded Signal Center event under signalcenter.ModuleOrchestrator naming the cycle and the failing gate"
    max_if_missing: 8
    evidence: "cd go && go test -run TestCompositionCarryForward_DeclineEmitsCodedSignalEvent ./internal/core/ -count=1"
  - criterion: "A decline naming multiple failing gates emits exactly ONE event naming every failing gate and its tail, not one event per gate"
    max_if_missing: 6
    evidence: "cd go && go test -run TestCompositionCarryForward_MultiGateFailure_NamesAllInOneEvent ./internal/core/ -count=1"
  - criterion: "An all-green composed tree emits zero decline events (no spurious signal)"
    max_if_missing: 5
    evidence: "cd go && go test -run TestCompositionCarryForward_AllGatesGreen_NoDeclineEvent ./internal/core/ -count=1"
  - criterion: "A composed-tree gate decline at RUNG 2 (scopedMergeCarryForward) registers the same one coded event shape as RUNG 0"
    max_if_missing: 8
    evidence: "cd go && go test -run TestScopedMergeCarryForward_GateDeclineEmitsCodedSignalEvent ./internal/core/ -count=1"
  - criterion: "composedGatesTo carries each gate's captured output tail in its own return value (not only the log sink), so the coded event's Reason can quote it"
    max_if_missing: 7
    evidence: "cd go && go test -run TestComposedGatesTo_ReturnsPerGateTail ./cmd/evolve/ -count=1"
  - criterion: "Each failing gate's realistic (>2 KB, 20-line) tail keeps its last line in the delivered event after Signal Center normalization, for two failing gates and for all four"
    max_if_missing: 8
    evidence: "cd go && go test -run TestCompositionCarryForward_RealisticTails_EveryFailingGatesLastLineReachesTheEvent ./internal/core/ -count=1"
  - criterion: "A composed-gate decline at RUNG 0 and RUNG 2 writes no gate output to stderr, and any stderr line about the gates names the cycle and ORCHESTRATOR_COMPOSED_GATE_DECLINED"
    max_if_missing: 8
    evidence: "cd go && go test -run 'TestCompositionCarryForward_GateDeclineWritesNoUntaggedStderr|TestScopedMergeCarryForward_GateDeclineWritesNoUntaggedStderr' ./internal/core/ -count=1"
  - criterion: "The composed-tree gate runner keeps a failing gate's output out of its log (no multi-line block); the output rides in the returned outcome"
    max_if_missing: 7
    evidence: "cd go && go test -run 'TestComposedGatesTo_FailingGateOutputStaysOutOfTheLog|TestRunComposedGates_RunsInTheCIEnvAndNamesAFailingGate' ./cmd/evolve/ -count=1"
  - criterion: "An identity-carry (ADR-0105 B3) composed-gate decline emits exactly one coded event from Orchestrator.identityCarryForward naming the cycle, each failing gate and its tail, with tagged stderr"
    max_if_missing: 8
    evidence: "cd go && go test -run TestIdentityCarryForward_GateDeclineEmitsOneCodedSignalEvent ./internal/core/ -count=1"
  - criterion: "Identity-carry outcomes that are not a failing gate (green carry, a writer failure) emit no composed-gate decline event"
    max_if_missing: 5
    evidence: "cd go && go test -run TestIdentityCarryForward_NonGateOutcomesEmitNoDeclineEvent ./internal/core/ -count=1"
---

# Eval: Composed-gate decline registers a coded Signal Center event

> Pins the filed item's two acceptance criteria ("A carry decline is one coded
> event naming the cycle, each failing gate and its output tail"; "No untagged
> multi-line stderr block remains for a decline") at all three
> `ciparity.MissingComposedGates` trip sites: RUNG 0 `compositionCarryForward`
> and RUNG 2 `scopedMergeCarryForward` (`go/internal/core/composition_carryforward.go`)
> and the ADR-0105 B3 identity carry (`go/internal/core/identity_carry_forward.go`).
> Each decline registers one `ORCHESTRATOR_COMPOSED_GATE_DECLINED` event under
> `signalcenter.ModuleOrchestrator` (precedent: `CodeRebaseReentryAborted`,
> `go/internal/core/ship_recovery_debugger.go`). The gate runner
> (`composedGatesTo`, `go/cmd/evolve/cmd_composition_wiring.go`) returns each
> gate's tail instead of dumping it to stderr, and the tail must survive the
> Signal Center's 512-rune per-value bound (`go/internal/log/field.go`) at
> realistic sizes. Source: console, 2026-09-29, C6 architecture review (M3);
> cycle 1772 audit round 1 (H1: stderr dump kept and identity carry left out;
> H2: tails packed into `Reason` and cut at 512 runes).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| rung0-decline-coded | RUNG 0 decline is one coded orchestrator event naming cycle + gate | 8/10 | `go test -run TestCompositionCarryForward_DeclineEmitsCodedSignalEvent ./internal/core/` |
| rung0-multi-gate-one-event | Multiple failing gates still produce exactly one event, all named | 6/10 | `go test -run TestCompositionCarryForward_MultiGateFailure_NamesAllInOneEvent ./internal/core/` |
| rung0-green-silent | An all-green composed tree stays silent (no spurious event) | 5/10 | `go test -run TestCompositionCarryForward_AllGatesGreen_NoDeclineEvent ./internal/core/` |
| rung2-decline-coded | RUNG 2 (scoped-merge) decline mirrors RUNG 0's coded-event shape | 8/10 | `go test -run TestScopedMergeCarryForward_GateDeclineEmitsCodedSignalEvent ./internal/core/` |
| gate-runner-carries-tail | The gate-runner boundary returns per-gate tail, not just status | 7/10 | `go test -run TestComposedGatesTo_ReturnsPerGateTail ./cmd/evolve/` |
| realistic-tail-survives | Every failing gate's last tail line survives the 512-rune bound (2 and 4 gates) | 8/10 | `go test -run TestCompositionCarryForward_RealisticTails_EveryFailingGatesLastLineReachesTheEvent ./internal/core/` |
| core-stderr-tagged | RUNG 0/2 decline: no tail on stderr; gate lines name cycle + code | 8/10 | `go test -run 'TestCompositionCarryForward_GateDeclineWritesNoUntaggedStderr\|TestScopedMergeCarryForward_GateDeclineWritesNoUntaggedStderr' ./internal/core/` |
| runner-log-no-block | The gate runner's log carries no gate output | 7/10 | `go test -run 'TestComposedGatesTo_FailingGateOutputStaysOutOfTheLog\|TestRunComposedGates_RunsInTheCIEnvAndNamesAFailingGate' ./cmd/evolve/` |
| identity-carry-coded | Identity-carry gate decline is one coded event, tagged stderr | 8/10 | `go test -run TestIdentityCarryForward_GateDeclineEmitsOneCodedSignalEvent ./internal/core/` |
| identity-carry-silent | Non-gate identity-carry outcomes emit no decline event | 5/10 | `go test -run TestIdentityCarryForward_NonGateOutcomesEmitNoDeclineEvent ./internal/core/` |
