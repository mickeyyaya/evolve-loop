# Build Explanation — Cycle 2105

## Build Binding
- Cycle: 2105
- Base SHA: a09ec38b0fc9d344ee01c104b905c180b7a11d33

## Summary
A new pure package, `internal/supersede`, tells a live cycle that the console took its item. A new strict policy key, `workflow.supersede.stage`, sets the stage of that check.

## Rationale
In cycle 2035 the cycle spent about 41 LLM minutes after the console took its item. The check reads the inbox state when it is asked. It does not poll. A failed read is an error and never a stop, so a broken inbox cannot end a healthy cycle.

## Changed Areas
- `go/internal/supersede/supersede.go` — `Check` returns none or superseded with the claim. `InboxReader` reads the claim cycle and the route of the item file.
- `go/internal/supersede/apicover_named_test.go` — tests that name every exported symbol of the package.
- `go/internal/policy/workflow_supersede.go` — `SupersedePolicy`, `SupersedeConfig`, the stage validation and the default `shadow`.
- `go/internal/policy/workflow.go` — the `supersede` field of the workflow block and of `WorkflowConfig`.
- `go/internal/policy/patch.go` — `Parse` calls the stage validation.
- `go/.apicover-enforce` — enrolls `./internal/supersede` in the apicover gate.
- `go/acs/cycle2105/predicates_test.go` — ACS predicates of the cycle.
- `.evolve/evals/cycle-stops-when-console-takes-its-item.md` — eval for the task.
- `docs/architecture/policy-config.md` — documents the new key.
- `docs/architecture/packages/internal-supersede.md` — package page.
- `docs/architecture/packages/internal-policy.md` — notes the new block.

## Design Decisions
An item is superseded when its route is a console route, or when another cycle holds its claim. A released item with no console route is none. The stage key reuses `config.GateStage`, as `builder_selfcheck` does, so the vocabulary has one home. The call site in `internal/core` is out of scope and belongs to the console item `wire-cost-aware-repair-call-sites`.

## Verification
The cycle 2105 ACS predicates, the tests of `internal/supersede`, `internal/policy` and `internal/config` pass.

## Compatibility
No existing key changes. The default stage is `shadow`, and nothing reads it yet.

## Limitations
No production code calls `Check` yet, so the stage has no effect until the console item wires it into the cycle loop.
