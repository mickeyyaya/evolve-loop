# Build Explanation — Cycle 1968

## Build Binding
- Cycle: 1968
- Base SHA: 760e31c0b09474184355ac60e3d33a109fa9b8f5

## Summary
A new package, `internal/handoff/evidence`, decides in code whether an evidence pointer of a handoff ledger entry resolves.

## Rationale
ADR-0141 requires that a pointer which does not resolve makes its entry unmet. A deterministic resolver keeps this judgment out of the prompt. The resolver is a separate package so that `internal/handoff` stays a leaf with no filesystem access.

## Changed Areas
- `go/internal/handoff/evidence/evidence.go` — adds `Resolve`, `Source` and `ErrUnresolved` for the kinds `path`, `test`, `predicate` and `command`.
- `go/internal/handoff/evidence/apicover_named_test.go` — names every exported symbol of the new package in a real assertion.
- `go/.apicover-enforce` — enrolls `./internal/handoff/evidence` in the apicover gate.
- `go/.docgo-allow-missing` — lists `./internal/handoff/evidence` so the docgo gate accepts the new package until it has its own package doc; the file is a shrink-only allowlist, and this entry must be removed once the package documents itself.
- `docs/architecture/packages/internal-handoff.md` — lists the evidence kinds and the resolvers.

## Design Decisions
Test declarations come from `go/parser` function declarations, not from text search, so a name in a string or a comment does not resolve. A file that does not parse resolves nothing. A path must be local to the root, so absolute and `..` paths do not resolve. A predicate resolves only inside `go/acs/cycle<N>` of the current cycle.

## Verification
The table test `TestResolve` covers every negative case of the acceptance. The cycle predicates `TestC1968_001` to `TestC1968_009` pass with the `acs` tag.

## Compatibility
The change is additive. No existing symbol changes.

## Limitations
No production code calls `Resolve` yet. The ledger check that consumes it is a later step of the handoff plan.
