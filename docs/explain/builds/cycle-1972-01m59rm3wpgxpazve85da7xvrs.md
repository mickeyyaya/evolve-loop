# Build Explanation — Cycle 1972

## Build Binding
- Cycle: 1972
- Base SHA: 34b02f8de82ae42fa1df512164c7f62431ca6359

## Summary
A todo that edits only leaf packages, such as `go/cmd/evolve`, now has an empty reach in admission. It no longer holds Go todos in other packages out of the pool.

## Rationale
`cmd/evolve` imports most of the module, so its reach met almost every other todo. In wave 131 the pool fell to 3 of 4 lanes while 18 items were lane-ready. Nothing imports a leaf package, so no other todo can break by compiling against it. The same-package rule and the hub rule stay in force.

## Changed Areas
- `go/internal/admission/modulegraph.go` — records the packages with zero importers and exposes `IsLeaf`.
- `go/internal/admission/admission.go` — adds the optional `leafGraph` interface and skips the whole reach only when every edited package is a leaf, so a mixed todo keeps the full reach of all its packages.
- `go/internal/admission/admission_test.go` — the admission fixture gets an importer of fleet, policy and quotastate, so they stay non-leaf.
- `go/internal/admission/modulegraph_test.go` — tests `IsLeaf`.
- `go/cmd/evolve/cmd_loop_admission_test.go` — the fixture gets an importer of fleet for the same reason.
- `docs/architecture/packages/internal-admission.md` — documents the leaf rule.
- `docs/architecture/wave-orchestrator.md` — records approach (b), the rejected (a) direct imports only, the rejected (c) reach by exported API, and the wave 131 evidence.
- `docs/plans/wave-orchestrator-2026-10.md` — resolves open question Q4 with the same decision.

## Design Decisions
Rejected: a hard-coded `cmd/evolve` exception. The graph already knows every package without an importer, so the rule needs no name list. A mixed todo keeps the full reach of every package it edits, leaf ones too.

## Verification
`go test -count=1 ./internal/admission/...` passes. All nine `TestC1972_*` predicates (001-009) pass.

## Compatibility
`Graph` is unchanged. `ModuleGraph` gains `IsLeaf`; the admitter reads it through an optional `leafGraph` interface, so other `Graph` implementers such as the loopwave test graph compile and keep full reach.

## Limitations
A leaf package that a later change starts to import stops being a leaf when the graph reloads.
