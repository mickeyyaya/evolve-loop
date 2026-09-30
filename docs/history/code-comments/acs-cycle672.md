# Comment history: `acs/cycle672`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle672/predicates_test.go:3` — above `package cycle672`

```text
// Package cycle672 materialises the acceptance criteria for the single
// triage-committed top_n task of cycle 672, echo-veto-wiring-completion
// (weight 0.92). Cycle 654 landed the leaf helpers
// (Classifier.SetInjectedPrompt, stripPromptEchoLines) with green helper-level
// tests; cycle 656's wiring attempt was quota-killed. Grep of this tree
// confirms both helpers still have ZERO production call sites, so the live
// paths misfire exactly as the cycle-656 retro (D3) caught: a 100%-echoed
// pane classified rate_limit. This cycle wires consumption:
//
//   - AC1 (C672_001) Produce mechanism: ProduceConfig.InjectedPrompt threads
//     the phase prompt into the Classifier — echoed stderr line suppressed,
//     genuine 429 frame still emits.
//   - AC1 (C672_002) runner caller: BaseRunner's REAL default events producer
//     passes the composed prompt through to phasestream.Produce — asserted on
//     the emitted <phase>-events.ndjson via Run() end-to-end.
//   - AC2 (C672_003) auto-responder: tick() strips echoed prompt lines ahead
//     of the exhaustion/escalation scan (behavioral, rc-85 assertion) and both
//     production construction sites populate the responder's injected prompt.
//   - AC3 (C672_004) negative axis: a genuine quota banner / genuine runtime
//     infra signal still escalates/classifies — the veto is not a blanket
//     disable. Includes the cycle-654 regression arms (TestC654_002/003/004),
//     which must STAY green after the wiring.
//
// Predicate strategy: behavioural-via-subprocess (cycle-549…654 precedent) —
// every predicate shells `go test -run` over RED wiring tests that EXERCISE
// the SUT (Produce over an on-disk workspace; BaseRunner.Run; tick() over a
// scripted pane); none is source-grep-only. RED now: phasestream and bridge
// fail to compile (ProduceConfig.InjectedPrompt / autoResponder.injectedPrompt
// absent); the runner case fails behaviorally (echoed line emits
// infra_failure). GREEN once Builder lands the wiring. The
// Acceptance-Criteria-Summary line "go test -race PASS; apicover clean" is
// dispositioned manual+checklist in test-report.md (repo-wide toolchain gates
// the cycle audit already runs), not predicated here.
```

### `go/acs/cycle672/predicates_test.go:95` — above `func TestC672_004_GenuineSignalsSurvive(t *testing.T) {`

```text
// TestC672_004_GenuineSignalsSurvive — AC3 negative axis: genuine exhaustion
// banners and genuine runtime infra signals still escalate/classify after the
// wiring; includes the cycle-654 regression arms which must stay green.
```
