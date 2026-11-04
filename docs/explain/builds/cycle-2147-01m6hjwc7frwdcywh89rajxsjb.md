# Build Explanation — Cycle 2147

## Build Binding
- Cycle: 2147
- Base SHA: 23d09f7c638b71e7c9906fcf6b0f76ff387c4570

## Summary
A version bump inside one model lineage now reuses the prior tier pick. The refresh makes no classifier call for it.

## Rationale
The reuse gate matched only an identical candidate fingerprint. A bump from gpt-5.5 to gpt-5.6 changed the fingerprint and paid a classifier call. The classifier decides which lineage serves a tier. Go already decides the newest version of a lineage, through PromoteLatest. When the lineage set is unchanged, the prior decision is still valid and only the version needs promotion.

## Changed Areas
- `go/internal/modelquery/query.go` — adds carryAcrossBump, isSoundPrior, priorDerivedFromItsOffering and sameLineages. liveTiers calls carryAcrossBump after the exact-fingerprint check and before the classifier.
- `go/internal/modelquery/query_carry_invalidation_test.go` — covers a policy change, an unchanged non-default policy and a prior hash of unknown origin across a bump.
- `go/internal/modelquery/query_lineage_carry_test.go` — covers a swapped lineage of equal count and a carried tier model that the CLI no longer offers.
- `go/internal/modelquery/refresh_reuse_test.go` — TDD-phase unit tests for bump, replacement, new lineage, vanished lineage and unsound prior.
- `go/internal/modelquery/effort_live_test.go` — the live integration tests install an exec launcher, because no composition root wires one in the package test binary.
- `go/acs/cycle2147/predicates_test.go` — acceptance predicates 001 to 008 for the task.
- `.evolve/evals/model-currency-c2-lineage-keyed-reuse.md` — eval graders for the task.
- `docs/architecture/model-discovery-and-catalog.md` — adds the carry rule and its prior-hash condition to the reuse section.
- `docs/architecture/packages/internal-modelquery.md` — records the lineage carry invariant.

## Design Decisions
The carry reuses LineageKey, GroupByLineage and PromoteLatest. It adds no new exported symbol and no flag. An unsound prior (detect-sourced, tier-incomplete, or without a recorded offering) never carries. The carry also requires the prior hash to equal the fingerprint of the prior offering under the current decision version, policy and tiers. A decision-version bump or a policy change therefore classifies again and the new hash is never stamped on a stale decision. A carried tier model that is not in the current offering falls through to the classifier, so a stale id never ships.

## Verification
The modelquery tests pass and every function of query.go has 100.0% coverage. The cycle 2147 predicates pass under the acs build tag.

## Compatibility
Fingerprint, decisionVersion surface files and the catalog schema are unchanged. The result carries the new fingerprint, so the next refresh reuses by exact match.

## Limitations
The carry trusts the prior classification for the lineages it already knew. A model that changes its tier role without a lineage change is not reclassified until the offering gains or loses a lineage.
