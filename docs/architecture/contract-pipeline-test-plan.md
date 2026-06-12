# Contract-Pipeline Test Plan (generation / check / verify)

> Produced 2026-06-12 from an architecture review of contract resolution
> (the unification that made the runner's reconcile-on-timeout default
> catalog-aware) plus a full coverage analysis of the three pipeline legs.
> P0 items are IMPLEMENTED in the same change; P1/P2 are the backlog.
> Companion design: ADR-0034 (deliverable contract), ADR-0035 (phase
> descriptor), ADR-0038 (phase plugins), ADR-0045 (correction ladder).

## The pipeline under test

| Leg | What it is | Code home |
|---|---|---|
| A — Generation | Built-in registry, spec-derived contracts (`FromSpec` ← phase.json), prompt projection (`RenderContractBlock` injected at bridge.go:228), `evolve phases create` minting | `go/internal/phasecontract`, `go/cmd/evolve/cmd_phases_create.go` |
| B — Check | Agent self-check `evolve phase verify` (exit 0/1/2/10), persona/profile coherence lint | `go/cmd/evolve/cmd_phase_verify.go`, `go/internal/phasecoherence` |
| C — Verify | Host contract gate (Reviewer + breaker), breaker-neutral salvage verifier, `VerifyCatalogAware` + runner reconcile default, correction-retry redispatch, challenge token | `go/internal/deliverable`, `go/internal/phases/runner` |

**The one invariant everything hangs on:** all four resolution consumers
(gate, salvage rung, self-check CLI, reconcile default) share ONE policy —
`phasespec.MergedCatalog` → `phasecontract.NewCatalogResolver` →
`deliverable.VerifyWith`. The 2026-06-12 review found and fixed the last
fork (reconcile default was builtin-only: a user/minted phase whose artifact
survived a bridge timeout synthesized FAIL).

## Test plan

Legend: P0 = cross-leg consistency property, implemented 2026-06-12.
P1/P2 = backlog (good cycle goals).

| # | Leg | Property | Type | Location | Pri | Status |
|---|-----|----------|------|----------|-----|--------|
| 1 | A↔C | Every contract in `Contracts()`: `RenderContractBlock` names every enforced section / JSON key / verdict + the self-check instruction (agents are never gated on a requirement they weren't shown) | property | `phasecontract/properties_test.go` | P0 | ✅ |
| 2 | C | `BuiltinResolver` and `CatalogResolver` return byte-identical contracts for every built-in phase (two strategies, one policy) | property | `phasecontract/properties_test.go` | P0 | ✅ |
| 3 | A→B | Roundtrip: `phases create` → `phase verify` resolves the minted phase (missing artifact = exit 1 not 10; conforming artifact = exit 0) | integration | `cmd_phases_create_test.go` | P0 | ✅ |
| 4 | B | CLI exit 1 (confirmed violation, not ambiguity) for stray-in-worktree artifact | CLI unit | `cmd_phase_verify_test.go` | P0 | ✅ |
| 5 | C | Gate at StageEnforce FAILS OPEN (approve, breaker untouched) when no contract resolves — the soak-observed `[contract-gate] ship: ambiguity, failing open` line, pinned | unit | `reviewer_spec_test.go` | P0 | ✅ |
| 6 | A | `phases create` with `kind=native, outputs={}`: envelope must not advertise a phantom contract (`FromSpec` called unguarded by `SynthesizesContract` at cmd_phases_create.go:167) | unit | `cmd_phases_create_test.go` | P1 | ☐ |
| 7 | A↔C | Registry↔profile artifact-name drift property extended to every future contract (exists for 6 spine phases) | property | `contract_registry_test.go` | P1 | partial |
| 8 | B | `phase verify` exit 2 on corrupt user phase.json for a USER phase request; exit 0 for a BUILT-IN request under the same corruption (degrade contract) | CLI unit | `cmd_phase_verify_test.go` | P1 | ☐ |
| 9 | A↔B | Bridge-injected contract block parsed section-by-section equals what `VerifyWith` enforces for the same user spec (not substring spot-checks) | unit | `bridge_contract_spec_test.go` | P1 | ☐ |
| 10 | C | Correction ladder drives a user-phase contract violation: redispatch fires carrying the violation verbatim as the correction directive | integration | `core/` correction tests | P1 | ☐ |
| 11 | C | `verifyJSON` rejects a top-level JSON array → `CodeInvalidJSON` | unit | `deliverable_test.go` | P1 | ☐ |
| 12 | C | `checkStray` no-op when `Worktree == Workspace` | unit | `deliverable_test.go` | P1 | ☐ |
| 13 | A | Mint-promotion envelope carries no phantom `required_sections` | unit | `cmd_phases_create_test.go` | P2 | ☐ |
| 14 | B | Three-way property: registry `ArtifactName` == persona `output-format` token == profile `output_artifact` basename, for every paired phase | property | `phasecoherence/` | P2 | ☐ |
| 15 | C | Challenge-token property over every `RequireChallengeToken` contract (currently example-based on build only) | property | `challenge_token_test.go` | P2 | ☐ |
| 16 | B | Built-in phase still verifies (exit 0) when a corrupt user spec exists in the catalog | CLI unit | `cmd_phase_verify_test.go` | P2 | ☐ |

## Why property loops over examples

Items 1/2/7/14/15 are invariants over a SET (all contracts, all paired
phases). A loop fails automatically when a new phase violates the invariant;
a hand-picked example silently goes stale — the same rot class as the
file-wide ACS count assertions (fixed by `acsassert.CountInGoFunc`, same day).

## Bugs these tests retro-cover

- **Reconcile default builtin-only** (fixed 2026-06-12): caught by
  `TestNew_DefaultVerifyFnIsCatalogAware` (runner) + plan #2/#3 would have
  caught it as a class.
- **tester profile `output_artifact` = builder's artifact** (fixed
  2026-06-12, persona-lint commit): caught by
  `TestArtifactNameMatchesProfileOutput`; plan #14 generalizes it to every
  phase including user phases.

## Design-review residue (accepted)

- `VerifyCatalogAware` precondition: `roots.EvolveDir` must be
  `<projectRoot>/.evolve` (documented in the func comment; every production
  constructor builds that shape).
- Catalog-load failure on the reconcile path now WARNs to stderr before
  degrading to builtin-only (was silent — a degrade there can flip a user
  phase's outcome).
- Pre-loaded (gate/salvage, loud at composition root) vs lazy (CLI/reconcile,
  degrade-with-WARN) catalog loading is a justified seam difference, not a
  policy fork: the what-resolves-to-what answer is identical.
- Follow-up chore (reviewer MEDIUM): three near-identical seed-project test
  fixtures (`phasespec/mergedcatalog_test.go`, `deliverable/catalogaware_test.go`,
  `runner/runner_reconcile_test.go`) → extract a shared internal testfixture
  builder so the three suites describe the same scenario.
