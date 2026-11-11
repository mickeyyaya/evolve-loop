# Build Explanation — Cycle 2323

## Build Binding
- Cycle: 2323
- Base SHA: 2b3c322a0bfa08dbe198248734accf709d5bcfad

## Summary
The core package now exports the list of resumable reasons. A new test in the checkpoint package checks that each one is a valid checkpoint reason.

## Rationale
The core package cannot import the checkpoint package. A test in checkpoint can read the real map through the new accessor, so the two lists cannot drift apart.

## Changed Areas
- `go/internal/core/resume.go` — adds ResumableReasons, which returns the sorted reasons of the resumable map.
- `go/internal/core/resume_reason_test.go` — tests that ResumableReasons lists the four resumable reasons.
- `go/internal/checkpoint/resumable_reasons_test.go` — tests that each resumable reason is a valid checkpoint reason and that settled reasons are not resumable.
- `go/acs/cycle2323/predicates_test.go` — predicates, including an overlay mutant that adds a bogus resumable key.
- `.evolve/evals/checkpoint-reasons-one-list.md` — eval graders for the task.

## Design Decisions
The accessor returns a sorted copy, so a caller cannot change the map and the order is stable.

## Verification
The core and checkpoint tests pass. The cycle 2323 predicates pass, and the overlay mutant turns the link test red.

## Compatibility
IsResumableReason and the resumable map keep their behavior.

## Limitations
The link is one way: a new checkpoint reason does not become resumable by this test.
