# internal/deliverable/gatesignal

## Purpose

`internal/deliverable/gatesignal` is the contract gate's producer on the Signal Center (ADR-0101, S2b). Every decision that `deliverable.Reviewer.Review` reaches becomes one event under module `gate.contract`. The orchestrator and triage read these events to see what was checked and why a phase moved on or was sent back.

## Design

- **One event per decision.** A passed phase is `gate.passed`; a refused one is `gate.rejected`. The codes are `GATE_CONTRACT_VERIFIED`, `GATE_CONTRACT_REJECTED`, `GATE_CONTRACT_WOULD_BLOCK`, `GATE_CONTRACT_SALVAGED`, `GATE_CONTRACT_DEMOTED` and `GATE_CONTRACT_FAIL_OPEN`, each registered with the Signal Center.
- **Reasons carry the correction.** A rejection's reason is the gate's own summary, one `[code] message` per violation. The orchestrator's ladder re-dispatches the phase with that text.
- **Breaker count.** The `blocks` field is 0 for a breaker-exempt phase, an optional evaluate phase whose exhausted ladder degrades to SKIPPED.
- **Null Object.** A nil Center makes every method a no-op. The production root always wires a Center, and `Reporter.Wired` proves the wiring.
- **The gate stays the one verifier.** This package only reports. It never decides.

## Invariants

- **Empty-valued fields are dropped from the emitted copy.** `emit` builds a new map, so the caller's map is never mutated. `TestReporter_EmitDoesNotMutateTheCallersFields` pins this.
- **Every event carries the gate's module, origin and the check's identity.** `TestReporter_NilCenterIsTheNullObject` and the `one` helper in the tests pin the shape.

## Findings

- None.
