# Build Explanation — Cycle 1633

## Build Binding
- Cycle: 1633
- Base SHA: b496a8dc0c475bf562a19f5fcfc2fe1d93ad6fe5

## Summary
Triage can now publish a validated, evidence-cited unified commitment while retaining each selected inbox item as an independently accepted task. Small commitments activate the existing deep-planning phases; large commitments become verified campaign plans.

## Rationale
Keeping synthesis as an optional triage judgment and validating it in deterministic Go preserves the existing structural inbox classifier. The implementation reuses the phase registry, router, build-planner, and campaign DAG instead of introducing another planner or grouping rule.

## Changed Areas
- `.evolve/evals/triage-unified-solution-synthesis.md` — carries the TDD-owned executable acceptance matrix for validation, fail-open behavior, routing, and campaign projection (lines 1-38).
- `agents/evolve-triage.md` — teaches triage when a unified commitment is credible, preserves independent selection, and documents the decision JSON shape (lines 83-90 and 180-184).
- `docs/architecture/phase-registry.json` — makes small validated commitments conditionally require plan-review and build-planner, and exposes `triage-decision.json` to both phases (lines 86-92, 468-481, and 527-539).
- `go/acs/cycle1633/predicates_test.go` — carries the TDD-owned 17-predicate contract without Builder modification (lines 1-734).
- `go/internal/inboxbatch/unified.go` — defines and validates evidence-cited commitments, including duplicate, unknown, incomplete, and heterogeneous rejection plus the shared size boundary (lines 1-93).
- `go/internal/inboxbatch/apicover_named_test.go` — executes and asserts every new inboxbatch export for the API-coverage floor (lines 41-62).
- `go/internal/phases/triage/triage.go` — invokes unified-commitment processing from the production phase classifier (lines 295-302).
- `go/internal/phases/triage/unified.go` — validates claims against live inbox items and `top_n`, fails open with a diagnostic, writes only validated routing projection, and emits large campaign plans (lines 1-116).
- `go/internal/router/signals.go` — carries validated commitment size and member count in typed triage signals (lines 108-117).
- `go/internal/router/digest.go` — reads the authoritative decision projection while retaining the independent committed-task count and degrading safely on malformed data (lines 42-130).
- `go/internal/router/condition.go` — resolves the new typed signal fields for phase-registry conditions (lines 83-86).
- `go/internal/phases/buildplanner/buildplanner.go` — evaluates the central routing policy with the real triage digest before deciding to skip (lines 62-69).
- `go/internal/campaign/campaign.go` — converts a validated large commitment into the existing verified dependency DAG while retaining per-member acceptance (lines 54-91).
- `go/internal/campaign/apicover_named_test.go` — names and executes the campaign projection export with acceptance and dependency assertions (lines 29-54).
- `skills/plan-review/SKILL.md` — keeps generated plan-review input documentation synchronized with the registry (lines 47-54).

## Design Decisions
The overlay is both qualitative and deterministic: triage authors the root-cause hypothesis and evidence, while Go alone decides whether the structure is trustworthy enough to influence routing. Invalid or spoofed projections are removed, independent `top_n` survives unchanged, and large work reuses the ADR-0054 campaign representation rather than adding a parallel execution format.

## Verification
The staged ACS package passes all 17 predicates. Scoped package tests and race tests cover the real triage classifier, router digest/condition path, build-planner skipper, and campaign planner; the enforced API-coverage run reports 149 exported symbols covered with zero uncovered or false-green symbols.

## Compatibility
Existing decisions without `unified_commitment` follow the unchanged path. Mechanical inbox batching retains its three structural rules, and malformed or unsupported commitments cannot set routing signals.

## Limitations
Deterministic validation establishes completeness, membership, evidence presence, campaign consistency, and deliverable-kind consistency; it cannot judge whether the prose root-cause hypothesis is semantically insightful, so plan-review remains the qualitative safeguard.
