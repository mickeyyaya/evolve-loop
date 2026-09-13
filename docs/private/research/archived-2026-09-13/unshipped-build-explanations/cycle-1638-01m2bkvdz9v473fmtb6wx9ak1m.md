# Build Explanation — Cycle 1638

## Build Binding
- Cycle: 1638
- Base SHA: 4c58eb6d1da8f5f7a26d677c0263e39b9e7842d8

## Summary
Validated unified commitments now give both planning phases deep-tier authority and pin those phases for both small and large commitments. Plan review also receives an explicit rubric for rejecting a group of unrelated patches presented as one general solution.

## Rationale
The existing validated `triage.unified_size` projection is the smallest trustworthy signal for this behavior. Reusing the integrity-floor clamp, declarative conditional-mandatory registry, and production-loaded plan-review persona avoids a second routing path or another public configuration surface.

## Changed Areas
- `.evolve/inbox/2026-07-21T02-00-00Z-triage-unified-solution-synthesis.json` — removes the selectable root copy after the unified-synthesis item enters its consumed lifecycle state.
- `.evolve/inbox/2026-07-30T23-05-00Z-overlay-family-name-transport-ambiguity.json` — removes the selectable root copy after the companion routing item enters its consumed lifecycle state.
- `.evolve/inbox/consumed/2026-07-21T02-00-00Z-triage-unified-solution-synthesis.json` — preserves the complete unified-synthesis record with landing metadata for auditability.
- `.evolve/inbox/consumed/2026-07-30T23-05-00Z-overlay-family-name-transport-ambiguity.json` — preserves the completed companion routing record with landing metadata for auditability.
- `agents/evolve-triage.md` — defines how triage authors one evidence-cited commitment while keeping every member bound to `top_n`.
- `agents/plan-reviewer.md` — requires REVISE or ABORT for patch bundles that fail the single-source, immutability, and KISS general-solution rubric.
- `docs/architecture/phase-registry.json` — pins plan-review and build-planner for every validated commitment size through the existing conditional-mandatory rules.
- `go/internal/campaign/campaign.go` — projects validated large commitments into the existing dependency-ordered campaign plan with member-specific acceptance.
- `go/internal/inboxbatch/unified.go` — defines structural validation for evidence-cited members and treats scoped and unscoped campaigns as heterogeneous.
- `go/internal/phases/buildplanner/buildplanner.go` — consumes central router policy and reports digest degradation instead of silently self-skipping on incomplete signals.
- `go/internal/phases/triage/triage.go` — invokes unified-commitment processing at the production triage decision boundary and retains fail-open diagnostics.
- `go/internal/phases/triage/unified.go` — validates against root and current-cycle claimed items before emitting a trusted projection or campaign plan.
- `go/internal/router/condition.go` — exposes the validated unified size and member count through the existing condition evaluator.
- `go/internal/router/digest.go` — derives routing signals only from a triage-produced validated projection, clearing forged or malformed values.
- `go/internal/router/floor.go` — raises existing plan-review and build-planner entries to deep tier for a validated commitment and records each raise as a clamp.
- `go/internal/router/signals.go` — carries the validated commitment size and member count through the typed routing signal.
- `skills/plan-review/SKILL.md` — keeps the plan-review input contract synchronized with the triage decision artifact it evaluates.

## Design Decisions
The triage model retains qualitative root-cause judgment, while deterministic Go validates membership and projects only the resulting trusted size. A non-empty validated size pins both planning phases through existing configuration and raises their existing plan entries at the sole integrity-floor clamp. The alternative of adding a second flag or a new phase-specific routing API was rejected because it would duplicate the signal and split authority.

## Verification
Cycle-1638 tests prove the deep-tier clamps, the absent-commitment negative, both size pins, production prompt loading, degraded-digest diagnostics, agreement between both routing authorities, and complete explanation coverage. The same diff originally authored contradictory cycle-1633 and cycle-1638 expectations; its TDD repair reconciles both to the size-independent planning requirement.

## Compatibility
Cycles with no validated commitment keep the advisor-selected planning tiers and do not gain conditional planning phases. Small commitments preserve their existing planning path, and large commitments retain campaign generation while adding the required design review and build planning.

## Limitations
The change does not alter campaign wave sizing or dependency ordering; it only ensures that both commitment sizes receive planning before execution.
