# Build Explanation — Cycle 2473

## Build Binding
- Cycle: 2473
- Base SHA: de22fd9eeaecac66f9b14ffb7631521b3ac0c4d5

## Summary
A review report whose only verdict line sits in a code fence now reads as the verdict that it states.

## Rationale
The sentinel reader and the visible-line reader both skip fenced text. A reviewer that wrote the verdict in a fence gave no stated verdict, so the run was FAIL with a parse defect. A third read, used only when the other two find nothing, removes the fence marker lines and reads the verdict again. The reader lives in agentcmd because reportdoc is a protected path.

## Changed Areas
- `go/internal/cli/agentcmd/verdict.go` — `statedVerdict` falls back to a read of the report with its fence marker lines removed, when the sentinel and visible reads find no verdict.
- `go/internal/cli/agentcmd/agent_verdict_test.go` — the fenced-verdict tests now state the new behavior.
- `go/internal/cli/agentcmd/agent_verdict_fence_test.go` — tests through the real agent run entry point.
- `go/acs/cycle2473/predicates_test.go` — the four cycle predicates.

## Design Decisions
The fallback comes last, so a visible or sentinel verdict always wins. The severity count reads the same text as the verdict. A visible verdict means visible text. A verdict from the fallback means the fence-stripped text. A fenced HIGH finding next to a fenced PASS keeps the run at FAIL. A fenced example finding next to a visible verdict stays uncounted.

## Verification
The agentcmd and reportdoc packages pass with `go test -count=1`. The cycle 2473 ACS predicates pass.

## Compatibility
No signature changes. A report with no verdict still fails and emits the parse defect.

## Limitations
A fenced example line that states a verdict counts when no other verdict exists. A fenced example finding in that same report counts too.
