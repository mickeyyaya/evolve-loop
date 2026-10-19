# Build Explanation — Cycle 1974

## Build Binding
- Cycle: 1974
- Base SHA: 34b02f8de82ae42fa1df512164c7f62431ca6359

## Summary
The new package `internal/handoff/request` builds the handoff `Request` of the pairs scout-triage, triage-tdd and tdd-build, each from one source.

## Rationale
Each builder reads one existing source of truth: the selected slugs, the committed set with the inbox acceptance, and the predicates of an ACS cycle package. A builder returns an error instead of a partial request, so a ledger can never validate against a request that lost an ask.

## Changed Areas
- `go/internal/handoff/request/request.go` — adds `ScoutTriage`, `TriageTDD`, `TriageTDDFromItems` and `TDDBuild`, the pair names and the three refusal errors.
- `go/internal/handoff/request/apicover_named_test.go` — names every exported symbol for the repo-wide apicover gate.
- `go/.cover-strict` — sets the package minimum to 100.
- `go/.apicover-enforce` — enrolls the package in the apicover gate.
- `go/.docgo-allow-missing` — lists the package like its siblings, because the zero-comment rule forbids a package doc comment.
- `docs/architecture/packages/internal-handoff.md` — lists the four builders in the package note.

## Design Decisions
`TriageTDD` reuses `committedset.Committed` and `inboxbatch.LoadFile`, so the committed set and the acceptance text keep one owner. `TDDBuild` uses `go/parser` and lists only functions that take `*testing.T`. The package imports neither `core` nor anything that reaches it; a test enforces this.

## Verification
`go test -tags integration -cover ./internal/handoff/...` reports 100.0% for the package. The ACS suite for cycle 1974 reports green=244 red=0.

## Compatibility
No existing exported symbol changes. The package is new and has no production caller yet.

## Limitations
No production caller wires the builders into a phase handoff. A later plan step must add it.
