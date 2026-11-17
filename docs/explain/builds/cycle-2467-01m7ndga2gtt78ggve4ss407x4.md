# Build Explanation — Cycle 2467

## Build Binding
- Cycle: 2467
- Base SHA: de22fd9eeaecac66f9b14ffb7631521b3ac0c4d5

## Summary
The enforce-stage actor now publishes the ejected event with the brief file of the lane. The orchestrator then sends the brief to the lane and holds its cells until the landing or the give-up.

## Rationale
The actor wrote the brief but never emitted the event that the orchestrator reads. An ejected lane got no brief. Its cells stayed free. One emit call in the existing eject path closes the gap.

## Changed Areas
- `go/internal/integration/enforce_actor.go` — emits the ejected event with the cause, paths and brief file after the brief is written.
- `go/internal/integration/prefix_ejection_test.go` — covers the A+B+C prefix, the diff of A+B in the brief, the textual conflict with no gate run, and the published event.
- `go/internal/orchestrator/oversight_enforce_ejection_test.go` — covers the brief to the lane and the held cells until the landing.
- `go/acs/cycle2467/predicates_test.go` — holds the four predicates of the task.

## Design Decisions
The emit reuses `emitEvent` and `Ejection.Fields`, the same helpers as the shadow actor, so the event shape has one home.

## Verification
`go test -count=1 ./internal/integration ./internal/orchestrator` passes, and the four predicates of cycle 2467 are green.

## Compatibility
A nil `Publish` port emits nothing, so existing callers are unchanged.

## Limitations
No production code builds `EnforceActor` yet, so the new emit has only test callers until the wiring cycle.
