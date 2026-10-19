# Build Explanation — Cycle 1954

## Build Binding
- Cycle: 1954
- Base SHA: c24feca09f4796399f364883c2b354c23ee64d46

## Summary
The policy package now supports the phase handoff policy block in `.evolve/policy.json`. Each phase pair configures a stage (`shadow` or `enforce`) and an optional judge tier.

## Rationale
Structured phase handoffs require explicit stage controls per phase pair. Strict decoding prevents configuration typos from silently passing into runtime execution.

## Changed Areas
- `docs/operations/runtime-reference.md` — documents the `handoff` block in `.evolve/policy.json` and its stage options.
- `go/internal/policy/handoff.go` — defines `HandoffPolicy` with strict JSON decoding and the `HandoffConfig()` resolver.
- `go/internal/policy/policy.go` — adds the `Handoff` field to `Policy` with JSON tag `handoff,omitempty`.
- `go/internal/policy/handoff_test.go` — tests for handoff policy decoding, default stage resolution, and invalid inputs.
- `go/internal/policy/handoff_named_test.go` — names and exercises the `HandoffPolicy` type export.
- `go/acs/cycle1954/predicates_test.go` — ACS acceptance predicates for the handoff policy block.
- `.evolve/evals/handoff-h6-policy-block.md` — eval graders for the handoff policy task.

## Design Decisions
When the handoff block or a pair stage is absent, the pair resolves to `shadow`. The decoder refuses unknown pair names, unknown keys, non-string stages, and stages outside `shadow` or `enforce`.

## Verification
All 18 unit tests in `go/internal/policy/` pass. All five ACS acceptance predicates in `go/acs/cycle1954/` pass.

## Compatibility
The checked-in `.evolve/policy.json` file does not set a handoff block. Existing configurations continue to resolve all pairs to stage `shadow`.

## Limitations
This change provides configuration parsing only. Downstream consumers for each pair will connect in later cycles.
