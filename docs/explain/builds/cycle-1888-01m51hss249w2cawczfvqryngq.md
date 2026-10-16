# Build Explanation — Cycle 1888

## Build Binding
- Cycle: 1888
- Base SHA: cdc0a4083bb2788518b371f4ed7b85721bdc0b85

## Summary
The broker now emits one gap `msg.dead` event for each distinct gap, and still reports the gap in every mailbox. The agent messaging spec now agrees with itself on the seal, the retry of a planned phase and a read from the head.

## Rationale
The ADR-0129 skeleton review found four follow-ups. A re-read of one gap emitted a second event, which inflated the dead-letter signal. The spec had three statements that the code or another section contradicted.

## Changed Areas
- `go/internal/agentmsg/broker/service.go` — `read` records each gap id in the existing dead-letter set and skips the event for a gap it already reported.
- `go/internal/agentmsg/broker/read_test.go` — the TDD phase added the cross-queue, gap dedup, planned retry and head-read tests that pin this behavior.
- `go/acs/cycle1888/predicates_test.go` — the TDD phase added the cycle predicates for the code and the spec.
- `docs/architecture/agent-messaging.md` — §4 seals planned entries like §10. §3 and §10 say a retry gets a planned-phase message again. §6 and T12 state the gap dedup and the head-read clear.
- `docs/architecture/packages/internal-agentmsg.md` — the package page states the gap dedup.

## Design Decisions
The gap id goes in the existing `state.dead` set, not a new set. A message key always holds a `|`, and a gap id never does, so the keys cannot collide. The dedup is for each service, which is for one cycle. A second reader of the same gap gets the gap in its mailbox but no second event.

## Verification
`go test -count=1 ./internal/agentmsg/broker` passes. `go test -tags acs -count=1 ./acs/cycle1888` passes 7 of 7 predicates.

## Compatibility
No exported symbol changes. The `msg.dead` event of a gap keeps its id and reason.

## Limitations
The seal of planned entries is a spec statement for component M7, which is not built yet.
