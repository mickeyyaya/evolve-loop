<!-- challenge-token: edb33ef304c8badf -->
<!-- ANCHOR:triage_decision -->
# Triage Decision — Cycle 448

cycle_size_estimate: medium
phase_skip: []

## top_n (commit to THIS cycle)
- salvage-cycle444-coverage-tests: restore + adapt the four cycle-444 salvaged test files (exhaustion-edge, leak-recovery, CI-parity-branch, cycle448 floor predicates) with coverage floors core ≥85.0%, audit ≥96.0%, bridge ≥94.5% — priority=H, evidence=cycle-444 post-mortem; salvage files verified on disk, no filename collisions with main; targets re-verified alive after merges 295-297, source=scout
- core-model-routing-branch-coverage: net-new adversarial branch tests lifting core total coverage from the measured 82.9% baseline to ≥86.0% across the degrade, advisory-log-only, auto-apply-soft-overlay, plan-clamp and advisorLaunch-all-fail arms; sequenced after the salvage task (same package, dependsOn=salvage-cycle444-coverage-tests) — priority=H, evidence=goal-named branch list; hypothesis H3; beyond-ask B1 error-wrap assertion, source=scout

## deferred (carry to NEXT cycle's carryoverTodos)
- router-llmroute-config-surgical-coverage: surgical edge tests raising router to ≥96.0%, llmroute to ≥96.5%, config to ≥96.5% coverage (below-floor rejection negative + nil/whitespace/dup OOD cases; carries the cycle-444 +2 carryover boost forward) — priority=M, defer_reason=committed-floor capacity cap is 3 (= ceil(1.25×K), K=2 over the last 5 shipped floor cycles) and the two committed tasks already occupy core/audit/bridge; different packages, parallel-safe, first pick next cycle
- cycle-437-failed-behavior-baseline: review single-occurrence behavior-baseline failure — priority=L, defer_reason=phase ran clean in cycles 440 and 442; revisit only on recurrence
- cycle-446-failed-test-amplification: review most recent unexplained phase failure — priority=M, defer_reason=this cycle is test-heavy, so the Auditor should watch the test-amplification phase live; promote to a task if it recurs in 448
- egps-timeout-false-fail (inbox, weight 0.85): systemic fix for the EGPS generator-timeout class that false-FAILed cycles 444 and 447 (loud DeadlineExceeded diagnostic naming the knob, retry-once on warmed build cache, regression test) — priority=M, defer_reason=the instance mitigation is already applied and verified (acs go_timeout_s raised 60→300 in the operator policy file); this cycle CONSUMES that mitigation via the salvage task; the hardening is queued model-governance campaign work
- advisor-policy-centralization (inbox, weight 0.93): centralize all advisor authority axes into the operator SSOT with reload semantics — priority=M, defer_reason=queued model-governance campaign work (design-review approved 2026-07-02); architecture change out of scope for this cycle's tests-only goal
- latest-model-preference (inbox, weight 0.95): latest-preferred, family-pure, 4-tier model-selection pipeline (D1-D9) — priority=M, defer_reason=large multi-slice feature that needs its own scout split (requires-split if forced into one cycle); queued model-governance campaign work per operator direction
- advisor-emit-tier-dormant (inbox, weight 0.90): advisor never emits {cli,tier} so model_routing=auto is dormant; elicit + replay-test + observability — priority=M, defer_reason=queued routing-side bugfix; must coordinate with the latest-model-preference tier vocabulary to avoid conflict
- bridge-integration-tests-flaky-ci (inbox, weight 0.6): stabilize or quarantine environment-sensitive real-tmux CI integration tests — priority=L, defer_reason=proven flake, not a regression; unrelated to this cycle's goal; candidate for a dedicated CI-hygiene cycle

## dropped (rejected with reason)
- cycle-366-failed-ship: SELF_SHA_TAMPERED integrity failure — reason=stale-class-fixed (ADR-0065 per-phase integrity + documented self-heal; ships green since cycle 440)
- cycle-384-failed-ship: SELF_SHA_TAMPERED integrity failure — reason=stale-class-fixed (same ADR-0065 class as cycle-366)
- cycle-428-failed-ship: native ship failure — reason=stale-class-fixed (same ADR-0065 integrity class; v21.6.0 released cleanly since)
- cycle-372-failed-scout: codex rc=1 launch failure — reason=stale-class-fixed (codex native install chain PRs 281-283, v21.4.x; no recurrence since cycle 380)
- cycle-373-failed-scout: codex rc=1 launch failure — reason=stale-class-fixed (same codex install class)
- cycle-374-failed-scout: codex rc=1 launch failure — reason=stale-class-fixed (same codex install class)
- cycle-375-failed-scout: codex rc=1 launch failure — reason=stale-class-fixed (same codex install class)
- cycle-376-failed-scout: codex rc=1 launch failure — reason=stale-class-fixed (same codex install class)
- cycle-377-failed-scout: codex rc=1 launch failure — reason=stale-class-fixed (same codex install class)
- cycle-378-failed-scout: codex rc=1 launch failure — reason=stale-class-fixed (same codex install class)
- cycle-380-failed-scout: codex-tmux session killed, exit 85 — reason=stale-class-fixed (same codex install class; last occurrence of it)
- cycle-390-failed-triage: tree-diff guard leak — reason=stale-class-fixed (lease fencing PRs 273-275 + worktree isolation fixes; >55 cycles old, no recurrence)
- cycle-393-failed-triage: capacity-cap overpack rejection — reason=stale (the lesson is APPLIED in this very report: 3 committed floors, remainder deferred)
- cycle-408-failed-triage: review-gate rejection — reason=stale-class-fixed (same triage-gate era; superseded by v21.4.x gate fixes)
- cycle-410-failed-triage: review-gate rejection — reason=stale-class-fixed (same triage-gate era)
- cycle-396-failed-scout: review-gate eval-materialization rejection — reason=stale-class-fixed (claude-tmux prompt delivery PR 277 + per-run tmux socket; no recurrence in cycles 411-448)
- cycle-397-failed-scout: review-gate eval-materialization rejection — reason=stale-class-fixed (same prompt-delivery class)
- cycle-398-failed-scout: review-gate rejection — reason=stale-class-fixed (same prompt-delivery class)
- cycle-399-failed-scout: review-gate rejection — reason=stale-class-fixed (same prompt-delivery class)
- cycle-401-failed-scout: review-gate rejection — reason=stale-class-fixed (same prompt-delivery class)
- cycle-402-failed-scout: review-gate rejection — reason=stale-class-fixed (same prompt-delivery class)
- cycle-403-failed-scout: review-gate rejection — reason=stale-class-fixed (same prompt-delivery class)
- cycle-404-failed-scout: review-gate rejection — reason=stale-class-fixed (same prompt-delivery class)
- cycle-405-failed-scout: review-gate rejection — reason=stale-class-fixed (same prompt-delivery class)
- cycle-406-failed-scout: review-gate rejection — reason=stale-class-fixed (same prompt-delivery class)
- cycle-407-failed-scout: review-gate rejection — reason=stale-class-fixed (same prompt-delivery class)
- cycle-409-failed-scout: review-gate rejection — reason=stale-class-fixed (same prompt-delivery class)
- cycle-400-failed-adversarial-review: adversarial-review phase failure — reason=stale-class-fixed (same era and gate class as the 396-409 group)
- cycle-439-failed-tdd: bridge launch failure — reason=stale-class-fixed (exhaustion-wall class fixed by PRs 295 and 296, regression-tested)
- cycle-443-failed-scout: bridge launch/exhaustion failure — reason=stale-class-fixed (same exhaustion class; this cycle's salvage task additionally pins it with tests)
- cycle-445-failed-scout: bridge launch/exhaustion failure — reason=stale-class-fixed (same exhaustion class)
- cycle-447-address-audit-findings (dossier): EGPS false-FAIL salvage of cycle-447 work — reason=already-shipped (merged to main as PR 297, commit 9305f8db)

## carryoverTodos warnings (if any)
- (none) — no carryover item has defer_count ≥ 3; all 33 entries have cycles_unpicked=0

## inbox pre-checks (idempotency)
- skip_shipped: cycle-audit-cycle-scoped-ci-gap — landed as PR 294 / ADR-0069 (commit d6480463): cycle audit now runs the exact repo-wide CI checks on the worktree
- skip_shipped: driver-quota-hang-wedge — landed as PR 295 / ADR-0070 (commit aaaa0cb1, completed by PR 296 commit 26986cf7): mid-phase exhaustion is a SignalCenter signal, fast-fails on the ~2s poll
- skip_shipped: ship-phase-no-deliverable-contract — landed as commit 75cc8719: explicit NoArtifact ship contract registered (contract_registry.go:174), contract gate no longer fails open for ship
- skip_rejected: (none)
- escalate_block: (none)
- ingested and bucketed above: egps-timeout-false-fail, advisor-policy-centralization, latest-model-preference, advisor-emit-tier-dormant, bridge-integration-tests-flaky-ci (all → deferred)

## Rationale
The goal directs salvage-first coverage hardening and the two committed tasks deliver exactly that: the intact cycle-444 salvage (33 tests, ~1300 lines, verified collision-free with its floor predicates) plus the net-new branch tests for the goal's biggest measured gap. The third scouted task is deferred solely because of the triage capacity clamp: committed floors are capped at 3 (K=2 from the shipped-cycle throughput window) and the committed pair already occupies core/audit/bridge — the cycle-393 overpack rejection is the precedent, and deferral is the designed relief valve. Blocker-solo (Core Principle 5) was explicitly evaluated and does NOT fire: the previous cycles' deterministic gate defect (EGPS 60s generator timeout) had its root knob already fixed and verified by the operator (go_timeout_s=300), the operator's cycle goal post-dates those failures and directs this exact salvage, and the remaining systemic hardening is deferred as egps-timeout-false-fail rather than silently dropped. Three inbox items were verified already-shipped on main and are skipped idempotently; the four queued model-governance items stay pending for their own campaign cycles per operator direction.
