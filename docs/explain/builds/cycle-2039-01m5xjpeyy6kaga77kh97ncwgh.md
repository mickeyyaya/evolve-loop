# Build Explanation — Cycle 2039

## Build Binding
- Cycle: 2039
- Base SHA: 7497f4649c2c27fab0c2c3a54026cdcb73dede9b

## Summary
The pure classifier sorts audit defects into lane or handoff categories. It identifies handoff defects that block the active lane.

## Rationale
Classification before repair rounds prevents wasted cycle time on defects that lanes cannot fix.

## Changed Areas
- `go/.apicover-enforce` — enrolls the internal fixability package in the apicover gate.
- `go/internal/fixability/doc.go` — provides substantive package documentation for the fixability package.
- `go/internal/fixability/fixability.go` — implements the pure classifier for lane and handoff findings.
- `go/internal/policy/patch.go` — invokes validation for repair policy settings during policy parse.
- `go/internal/policy/workflow.go` — adds repair policy fields to the workflow policy structures.
- `go/internal/policy/workflow_repair.go` — defines repair policy structures and validates the fixability stage.

## Design Decisions
The classifier operates as a pure package without side effects or process dispatch.

## Verification
ACS suite predicates and unit tests verify classification rules and policy stage validation.

## Compatibility
The fixability stage defaults to shadow mode and does not alter existing execution.

## Limitations
This change provides the pure classifier and does not wire repair call sites.
