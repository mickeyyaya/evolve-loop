# Build Explanation — Cycle 1647

## Build Binding
- Cycle: 1647
- Base SHA: 23f1073dc9e27309c0267d986faa290f8423059a

## Summary
This continuation preserves the unified-commitment planning changes inherited from cycle 1638 and closes the routing-overlay ambiguity by documenting its selector precedence and proving both advisor-path transport outcomes through the production runner.

## Rationale
The surviving task record at `.evolve/inbox/consumed/2026-07-30T23-05-00Z-overlay-family-name-transport-ambiguity.json:1` requires bare family names to preserve an existing transport while explicit driver names remain authoritative. The three-rung resolver behavior was already correct, so documenting that decision at the package boundary and exercising the existing runner path is the smallest change; normalizing advisor input again or adding another routing abstraction would duplicate authority.

## Changed Areas
- `.evolve/inbox/2026-07-21T02-00-00Z-triage-unified-solution-synthesis.json` — removes the selectable root copy after the inherited unified-synthesis item entered its consumed lifecycle state.
- `.evolve/inbox/2026-07-30T23-05-00Z-overlay-family-name-transport-ambiguity.json` — removes the selectable root copy after this routing item entered its consumed lifecycle state.
- `.evolve/inbox/consumed/2026-07-21T02-00-00Z-triage-unified-solution-synthesis.json` — preserves the inherited unified-synthesis record with landing metadata for auditability.
- `.evolve/inbox/consumed/2026-07-30T23-05-00Z-overlay-family-name-transport-ambiguity.json` — preserves the routing task and its complete acceptance criteria after consumption.
- `agents/evolve-triage.md` — defines how triage authors one evidence-cited unified commitment while keeping every member bound to `top_n`.
- `agents/plan-reviewer.md` — requires revision or rejection when unrelated patches are presented as one general solution.
- `docs/architecture/phase-registry.json` — pins plan-review and build-planner for validated small and large unified commitments through the central phase registry.
- `go/internal/campaign/campaign.go` — projects validated large commitments into the existing dependency-ordered campaign plan with member-specific acceptance.
- `go/internal/inboxbatch/unified.go` — validates evidence-cited members and distinguishes scoped from unscoped campaign membership.
- `go/internal/llmroute/llmroute.go` — states the exact-entry, bare-family, and explicit-driver overlay precedence in the package documentation where callers discover the contract.
- `go/internal/phases/buildplanner/buildplanner.go` — consumes central router policy and reports digest degradation rather than silently skipping planning.
- `go/internal/phases/runner/model_routing_overlay_test.go` — drives advisor overlays through `BaseRunner.Run` and proves both transport preservation and explicit-driver fallback order.
- `go/internal/phases/triage/triage.go` — invokes unified-commitment processing at the production triage boundary while preserving fail-open diagnostics.
- `go/internal/phases/triage/unified.go` — validates root and current-cycle claimed items before emitting a trusted projection or campaign plan.
- `go/internal/router/condition.go` — exposes validated unified size and member count through the existing condition evaluator.
- `go/internal/router/digest.go` — derives routing signals only from a triage-produced validated projection and clears forged or malformed values.
- `go/internal/router/floor.go` — raises plan-review and build-planner entries to deep tier for a validated commitment and records each clamp.
- `go/internal/router/signals.go` — carries validated commitment size and member count through the typed routing signal.
- `skills/plan-review/SKILL.md` — keeps the plan-review input contract synchronized with the triage decision artifact it evaluates.

## Design Decisions
The inherited unified-commitment implementation keeps qualitative synthesis in triage and deterministic validation in Go. For the current routing task, `llmroute.ApplySoftOverlay` remains the single behavior authority: exact chain entries win first, bare names select the first same-family entry without changing transport, and hyphen-qualified names request that concrete driver. The runner test uses existing test fixtures and the real `BaseRunner.Run` entry point rather than a new helper or a resolver-only duplicate.

## Verification
The focused runner regression covers the advisor projection at `go/internal/phases/runner/routing.go:71` separately from contract escalation, with literal `claude-p`, `codex`, and `claude-tmux` inputs. Cycle-1647 ACS checks also execute both runner outcomes, package documentation, race checks, API coverage, and artifact tracking. The predicate packages added by this base-bound diff cover the unified-commitment paths, including the central registry behavior at `docs/architecture/phase-registry.json:88-89`; the source acceptance remains auditable in `.evolve/inbox/consumed/2026-07-21T02-00-00Z-triage-unified-solution-synthesis.json:1`.

## Compatibility
Existing exact driver entries and fallback order remain unchanged. Bare family overlays now have a discoverable contract but no new runtime behavior, and phases without unified commitments retain their prior routing and planning behavior.

## Limitations
This change does not redesign CLI naming, add a configuration surface, or choose among multiple same-family candidates beyond preserving their resolved order. It also does not alter the inherited campaign wave sizing or dependency ordering.
