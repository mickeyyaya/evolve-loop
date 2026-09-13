# Build Explanation — Cycle 1637

## Build Binding
- Cycle: 1637
- Base SHA: df11933d25193a7b4ae63aeb18ea58ef543567ab

## Summary
Triage can carry an evidence-cited unified solution through its decision boundary while preserving each selected inbox item as an independent commitment. The continuation also closes two reachability gaps: an unscoped item cannot be unified with a campaign-scoped item, and items already claimed by the current cycle remain visible to validation.

## Rationale
The implementation keeps qualitative synthesis in the triage agent and validates only structural claims in Go. Reusing the existing triage decision, router conditions, build-planner phase, and campaign planner avoids a parallel orchestration path. Treating the empty campaign as a real partition and loading only the current cycle's claimed directory are the smallest changes that align validation with the existing inbox lifecycle.

## Changed Areas
- `.evolve/inbox/2026-07-21T02-00-00Z-triage-unified-solution-synthesis.json` — removes the root inbox copy after the unified-synthesis item is consumed so it cannot be selected again.
- `.evolve/inbox/2026-07-30T23-05-00Z-overlay-family-name-transport-ambiguity.json` — removes the root inbox copy after the companion routing item is consumed so it cannot be selected again.
- `.evolve/inbox/consumed/2026-07-21T02-00-00Z-triage-unified-solution-synthesis.json` — preserves the complete unified-synthesis item with ship-consumption metadata for lifecycle history and auditability.
- `.evolve/inbox/consumed/2026-07-30T23-05-00Z-overlay-family-name-transport-ambiguity.json` — preserves the complete companion routing item with ship-consumption metadata for lifecycle history and auditability.
- `agents/evolve-triage.md` — documents when triage may author a unified commitment, retains each member in `top_n`, and defines the existing small/large planning routes (`agents/evolve-triage.md:83-90`, `agents/evolve-triage.md:180-183`).
- `go/internal/campaign/campaign.go` — projects validated large commitments onto the existing campaign DAG while preserving member dependencies and independent acceptance contracts (`go/internal/campaign/campaign.go:54-89`).
- `go/internal/inboxbatch/unified.go` — defines and validates the typed commitment; campaign comparison now treats the empty campaign as a distinct scope (`go/internal/inboxbatch/unified.go:15-23`, `go/internal/inboxbatch/unified.go:69-80`).
- `go/internal/phases/buildplanner/buildplanner.go` — consults the validated triage digest through the central router policy before skipping build planning (`go/internal/phases/buildplanner/buildplanner.go:59-67`).
- `go/internal/phases/triage/triage.go` — invokes unified-commitment processing at the production triage decision boundary and preserves fail-open diagnostics (`go/internal/phases/triage/triage.go:295-305`).
- `go/internal/phases/triage/unified.go` — validates claims against both root inbox items and items claimed by this cycle, then emits only a validated routing projection or campaign plan (`go/internal/phases/triage/unified.go:46-60`, `go/internal/phases/triage/unified.go:81-97`).
- `go/internal/router/condition.go` — exposes unified size and member count to the existing declarative condition evaluator (`go/internal/router/condition.go:83-86`).
- `go/internal/router/digest.go` — reads the validated projection from `triage-decision.json` and rejects malformed projections before they reach routing (`go/internal/router/digest.go:47-52`, `go/internal/router/digest.go:117-130`).
- `go/internal/router/signals.go` — carries the validated unified size and member count in the triage routing signal (`go/internal/router/signals.go:107-116`).
- `skills/plan-review/SKILL.md` — keeps the generated plan-review input contract synchronized by naming `triage-decision.json` as an input (`skills/plan-review/SKILL.md:43-52`).

## Design Decisions
The triage agent authors the root-cause judgment, while Go checks completeness, known membership, evidence, campaign scope, deliverable kind, and membership in `top_n`. Invalid commitments fail open to independent work and cannot leave a trusted projection behind. Small commitments reuse plan-review and build-planner; large commitments reuse the campaign DAG. The rejected alternative was adding semantic root-cause inference to deterministic inbox batching, because that would confuse structural grouping with qualitative judgment.

## Verification
The cycle-1637 ACS suite exercises the real triage caller and proves mixed unscoped/scoped membership is rejected, current-cycle claimed items validate, other-cycle and processed items do not, landing is transactional, and only validated projections route. Package tests, `go vet`, race tests over `internal/llmroute`, `internal/router`, `internal/core`, `internal/inboxbatch`, and `internal/phases/triage`, and API coverage over the touched exported surfaces are green. Review evidence includes `skills/plan-review/SKILL.md:51` and every implementation seam cited in Changed Areas.

## Compatibility
Ordinary decisions without `unified_commitment` remain unchanged. All-unscoped and all-same-campaign commitments remain valid, and the claimed-item lookup is bounded to `processing/cycle-1637` rather than other cycles or processed history.

## Limitations
Structural validation cannot decide whether a proposed root-cause hypothesis is insightful; plan-review remains the qualitative safeguard. Loader sanitization warnings are still not promoted into the unified-commitment diagnostic stream.
