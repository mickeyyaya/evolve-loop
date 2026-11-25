# Build Explanation — Cycle 2637

## Build Binding
- Cycle: 2637
- Base SHA: dcb2462f73117e37e0783aa35ebe49aed61bfd2d

## Summary
The policy key `code_index.on_absent` now controls if `evolve code` refuses or falls through when the primary backend returns `ErrAbsent`. The spec and the plan now state this rule.

## Rationale
The spec forbade failover while the plan and research dossier expected fallback to `trail` when the `gotypes` index is absent. One validated policy key resolves both modes with no hidden switch.

## Changed Areas
- `go/internal/policy/policy_codeindex.go` — adds `on_absent` (`CodeIndexOnAbsentRefuse`, `CodeIndexOnAbsentNext`, `ErrCodeIndexOnAbsent`, and `CodeIndexSettings.OnAbsent`) so one policy rule selects the behavior.
- `go/internal/codeindex/backends.go` — adds `Option`, `WithAbsentFallThrough`, and optional `ErrAbsent` fall-through in `Backends.SearchServed` so `ErrNotFound` and other errors still return immediately.
- `go/cmd/evolve/cmd_code.go` — passes `WithAbsentFallThrough` from `codeIndexLocation.onAbsent` in `newCodeBackends` so `evolve code` obeys `code_index.on_absent`.
- `docs/architecture/code-index-cli.md` — replaces the no-failover statements with the `on_absent` key table row and behavior rules.
- `docs/plans/code-index-backends-2026-10.md` — updates the G4 component row to name the `on_absent` rule.
- `docs/architecture/packages/internal-codeindex.md` — updates the chain design and wiring notes to describe `WithAbsentFallThrough`.

## Design Decisions
`code_index.on_absent` defaults to `refuse` to preserve current command exits until G4 flips the route. `SearchServed` falls through only on `ErrAbsent` and reports the first `ErrAbsent` backend if every capable backend is absent.

## Verification
Unit tests in `go/internal/policy`, `go/internal/codeindex`, and `go/cmd/evolve` and the cycle 2637 ACS predicates pass.

## Compatibility
Existing callers of `codeindex.New` need no edit because `opts ...Option` is variadic. Stale-index handling stays under `code_index.stale` (exit 21 under `refuse`).

## Limitations
The default route in `policy.json` remains `["trail"]` and `on_absent` defaults to `refuse` until the G4 item switches them.
