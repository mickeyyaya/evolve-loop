# Comment history: `acs/cycle1310`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1310/predicates_test.go:3` — above `package cycle1310`

```text
// Package cycle1310 materialises the cycle-1310 acceptance criteria for the one
// fleet-scoped item pinned to this lane: `phase-mint-carries-select-metadata`
// (scout tasks `mint-default-select-metadata` + `mint-live-path-wiring-proof`).
//
// The defect class. phaseregistrar.Registrar.Register persists a minted
// phase.json + profile stub built verbatim from the advisor-supplied
// PhaseConfig, so a config with empty Description/WhenToUse mints a stub that —
// once swept into git tracking by ship's whole-tree bind — fails the repo-wide
// guard TestPhaseCatalog_OptionalPhasesHaveSelectMetadata, and a config with an
// empty Dispatch.CLI mints a driverless profile that fails
// TestSmoke_RealProfiles / TestRepoPersonaProfilePairing and dies later at
// dispatch preflight. Four live instances (#399, #404, #406, #407) were each
// fixed by hand; this cycle closes the class at the mint seam.
//
// Predicate strategy — every predicate DRIVES the live seam
// (phaseregistrar.Registrar.Register, the real function the orchestrator's mint
// path calls) and asserts on its return value, its error, or the artifact it
// actually wrote to disk. No predicate greps registrar.go for a magic string
// (the cycle-85 degenerate-predicate ban):
//
//   - 001 mints with empty Description AND WhenToUse and asserts the RETURNED
//     spec no longer satisfies the guard's missing-metadata condition, and that
//     the PERSISTED phase.json carries the same metadata (a default applied only
//     in memory would still write a contract-breaking stub to disk).
//   - 002 is the negative/anti-no-op predicate: an empty Dispatch.CLI must be
//     REJECTED (non-nil error) and NOTHING may be persisted — a driverless
//     profile stub is the literal #406 failure.
//   - 003 is the wiring proof named in the inbox item: mint an unknown phase
//     name through the LIVE Register into a temp project root, then reload it
//     through phasespec.MergedCatalog — the exact loader the guard test uses —
//     and apply the guard's own condition to the merged catalog. Passing on the
//     returned spec alone (001) would not prove the round-trip through disk +
//     the real catalog loader stays guard-green.
//   - 004 asserts the wiring-proof TEST itself exists in the normal (untagged)
//     suite and passes: `go test -run TestRegister_UnknownPhaseNameStaysCatalogGreen
//     ./internal/phaseregistrar`. ACS predicates are cycle-scoped; the durable
//     regression lives in registrar_test.go, so its absence is a failed AC.
//   - 005 is the semantic/edge predicate: the default must fire only when BOTH
//     metadata fields are empty (mirroring the guard's own OR-condition), so an
//     advisor-supplied Description survives verbatim and is never clobbered.
```

### `go/acs/cycle1310/predicates_test.go:147` — above `func TestC1310_002_DriverlessMintRejected(t *testing.T) {`

```text
// TestC1310_002_DriverlessMintRejected is the negative predicate: an empty
// Dispatch.CLI mints a profile with no known driver — the #406 / instance-4
// failure — and must be rejected before anything is written.
```
