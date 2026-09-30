# Comment history: `acs/regression/protectedsurface`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/protectedsurface/predicates_test.go:3` — above `package protectedsurface`

```text
// Package protectedsurface is the L4 durable guard (architecture review
// 2026-07-16): every "gate-shaped" Go file in the repo must be covered by the
// protected-surface manifest, so a NEW gate or guard file can never sit
// silently OUTSIDE the control-plane write boundary.
//
// Why this guard exists: guards.IsProtectedSurface denies in-cycle writes
// against guards.ProtectedSurfaceManifest (go/internal/guards/
// integrity_surface.go) — the SSOT for the pipeline integrity control plane.
// Before L4 that list was a narrow hardcoded literal, so a new gate file
// created outside the listed fragments was writable by the very cycle it
// judges: the trust kernel's perimeter rotted as it grew. This predicate makes
// perimeter growth LOUD — when a gate-shaped file appears that the manifest
// does not cover, the audit REDs until an operator extends the manifest via a
// human-gated manual ship (the manifest itself is protected surface, so no
// autonomous cycle can both add a gate and quietly bless it).
//
// "Gate-shaped" is a deliberate, mechanical class: every .go file under the
// known control-plane directories (go/internal/guards, go/internal/commitgate,
// go/internal/phaseintegrity, go/acs/regression), plus any file named
// *_gate.go or *guard*.go anywhere under go/internal. Coverage is checked by
// calling the REAL guards.IsProtectedSurface — this package holds no duplicate
// fragment list, so it can never drift from the boundary it verifies.
```

### `go/acs/regression/protectedsurface/predicates_test.go:80` — above `"go/internal/phaseintegrity/source.go",`

```text
// dir lane: ADR-0065 integrity chain
```
