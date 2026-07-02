<!-- challenge-token: a3dc9173039041f2 -->
<!-- ANCHOR:triage_decision -->
# Triage Decision — Cycle 449

cycle_size_estimate: medium
phase_skip: []

## top_n (commit to THIS cycle)
- salvage-core-leak-recovery-coverage: copy-adapt cycle-444 `leak_recovery_unit_test.go` into `go/internal/core/` + adapted core-floor predicates (`go/acs/cycle449/predicates_core_test.go`, floor 85.0); fix-and-cover any real defect surfaced in `leak_recovery.go` — priority=H, evidence=scout fresh cover-func (core 83.1%, leak_recovery.go 8 funcs at 0%; salvage audited PASS 0.9 in cycle 444), source=scout
- salvage-bridge-exhaustion-edges-coverage: copy-adapt cycle-444 `exhaustion_edges_test.go` into `go/internal/bridge/` + adapted bridge-floor predicates (`go/acs/cycle449/predicates_bridge_test.go`, floor 94.5); re-check assertions against post-#296/#297 drift — priority=H, evidence=scout fresh cover-func (bridge 93.5%; matchExhausted 66.7%, defaultKeychainProbe 27.3%), source=scout
- salvage-audit-ciparity-branches-coverage: copy-adapt cycle-444 `ciparity_branches_test.go` into `go/internal/phases/audit/` + adapted audit-floor predicates (`go/acs/cycle449/predicates_audit_test.go`, floor 96.0); preserve sysexec.RunFunc fail-at injection — priority=H, evidence=scout fresh cover-func (audit 92.6%; generateACSVerdict 58.3%), source=scout

## deferred (carry to NEXT cycle's carryoverTodos)
- router-llmroute-config-surgical-coverage: fresh surgical edge tests for router/llmroute/config (ApplyDriverBench 62.5%, parseMode 60.0%, chainCandidates edges) — priority=M, defer_reason=cycle-448 was gate-rejected for committing 4 coverage floors over the capacity cap 3; this is the only non-salvage task and scout's designated first-drop, so its three package floors defer to the next cycle
- latest-model-preference: model-selection policy for all CLIs (D1-D9, 4-tier top-default, newest-wins, conformance) — priority=M, defer_reason=operator-queued model-governance campaign work, out of scope for this cycle's coverage-salvage goal; picks up in the next convergence window (inbox, weight 0.95)
- advisor-policy-centralization: centralize advisor policy into .evolve/policy.json advisor{} + `evolve config reload` — priority=M, defer_reason=architecture item from design-review 2026-07-02, out of scope for the coverage goal (inbox, weight 0.93)
- advisor-emit-tier-dormant: model_routing=auto dormant — advisor never emits {cli,tier}; elicit + verify end-to-end — priority=M, defer_reason=routing-side bugfix queued behind the coverage batch; coordinate with latest-model-preference tier vocab (inbox, weight 0.90)
- egps-timeout-false-fail: EGPS generator timeout must be loud + distinguishable from missing predicates — priority=M, defer_reason=immediate mitigation already applied (policy acs.go_timeout_s=300) and this cycle's predicates are split per-package to stay cheap; the systemic fix (timeout diagnostics, retry-once) is a separate code change out of coverage scope (inbox, weight 0.85)
- bridge-integration-tests-flaky-ci: stabilize/quarantine environment-sensitive real-tmux CI tests — priority=M, defer_reason=proven flake, mitigated by re-run; not coverage work and touches integration tier the goal explicitly defers (inbox, weight 0.6)
- ship-phase-no-deliverable-contract: register a ship-phase deliverable contract so contract-gate stops failing open — priority=M, defer_reason=low-risk gate hole but out of coverage scope; queue for a hygiene cycle (inbox, weight 0.6)
- cycle-390-failed-triage: tree-diff guard main-tree leak record — priority=L, defer_reason=recovery machinery (leak_recovery.go) gets first real coverage via top_n item 1 this cycle; keep the record until that coverage ships
- cycle-399-failed-scout: same tree-diff-guard class as cycle-390 — priority=L, defer_reason=partially addressed by top_n item 1; process fix (workspace-local writes) already in effect
- cycle-393-failed-triage: triage-overpack review-gate rejection — priority=L, defer_reason=triage prompt-discipline record; kept visible because the pattern recurred in cycle 448 (this cycle complies: exactly 3 committed floors)
- cycle-448-failed-triage: triage-overpack review-gate rejection (4 floors > cap 3) — priority=L, defer_reason=directly heeded by this decision (3 floors committed, T4 deferred); keep one more cycle to confirm the pattern is broken
- cycle-437-failed-behavior-baseline: `launch exit=10 new-session` bridge flake — priority=M, defer_reason=recurring infra flake (5 hits in 10 cycles), needs a dedicated root-cause cycle after the coverage priority clears (scout B2)
- cycle-439-failed-tdd: same `launch exit=10` flake cluster — priority=M, defer_reason=see cycle-437 entry (B2 root-cause cycle)
- cycle-443-failed-scout: same `launch exit=10` flake cluster — priority=M, defer_reason=see cycle-437 entry (B2 root-cause cycle)
- cycle-445-failed-scout: same `launch exit=10` flake cluster — priority=M, defer_reason=see cycle-437 entry (B2 root-cause cycle)
- cycle-446-failed-test-amplification: same `launch exit=10` flake cluster — priority=M, defer_reason=see cycle-437 entry (B2 root-cause cycle)

## dropped (rejected with reason)
- cycle-366-failed-ship: SELF_SHA_TAMPERED ship failure — reason=stale; root-caused as plugin-install drift with a documented self-heal (`make build` → `evolve reset-sha -operator`, ADR-0065); no code change pending
- cycle-384-failed-ship: SELF_SHA_TAMPERED ship failure — reason=stale; same class and same documented self-heal as cycle-366
- cycle-428-failed-ship: SELF_SHA_TAMPERED ship failure — reason=stale; same class and same documented self-heal as cycle-366
- cycle-372-failed-scout: codex launch rc=1 — reason=stale; fixed by boot/trust-prompt delivery (#277) + provider-routing playbook (#199); >70 cycles old, non-recurring since
- cycle-373-failed-scout: codex launch rc=1 — reason=stale; same fixed era as cycle-372
- cycle-374-failed-scout: codex launch rc=1 — reason=stale; same fixed era as cycle-372
- cycle-375-failed-scout: codex launch rc=1 — reason=stale; same fixed era as cycle-372
- cycle-376-failed-scout: codex launch rc=1 — reason=stale; same fixed era as cycle-372
- cycle-377-failed-scout: codex launch rc=1 — reason=stale; same fixed era as cycle-372
- cycle-378-failed-scout: codex launch rc=1 — reason=stale; same fixed era as cycle-372
- cycle-380-failed-scout: codex-tmux session killed rc=85 — reason=stale; fixed by per-run tmux socket (#275) + tickDuringBoot (#277)
- cycle-396-failed-scout: scout did not materialize evals — reason=stale; fixed by the workspace-local eval materialization path (cycle-449 scout wrote all 4 evals before finalize)
- cycle-397-failed-scout: scout did not materialize evals — reason=stale; same fixed class as cycle-396
- cycle-398-failed-scout: scout did not materialize evals — reason=stale; same fixed class as cycle-396
- cycle-401-failed-scout: scout did not materialize evals — reason=stale; same fixed class as cycle-396
- cycle-402-failed-scout: scout did not materialize evals — reason=stale; same fixed class as cycle-396
- cycle-403-failed-scout: scout did not materialize evals — reason=stale; same fixed class as cycle-396
- cycle-404-failed-scout: scout did not materialize evals — reason=stale; same fixed class as cycle-396
- cycle-405-failed-scout: scout did not materialize evals — reason=stale; same fixed class as cycle-396
- cycle-406-failed-scout: scout did not materialize evals — reason=stale; same fixed class as cycle-396
- cycle-407-failed-scout: scout did not materialize evals — reason=stale; same fixed class as cycle-396
- cycle-409-failed-scout: scout did not materialize evals — reason=stale; same fixed class as cycle-396
- cycle-400-failed-adversarial-review: bridge exit=81 session-killed — reason=stale; fixed by per-run tmux socket (#275) + tickDuringBoot (#277)
- cycle-408-failed-triage: bridge exit=81 session-killed — reason=stale; same fixed era as cycle-400
- cycle-410-failed-triage: bridge exit=81 session-killed — reason=stale; same fixed era as cycle-400

## inbox (pre-checks, idempotency)
- skip_shipped: cycle-audit-cycle-scoped-ci-gap — shipped as PR #294 (`d6480463`, ADR-0069 cycle CI-parity gate); memory ci_parity_gate confirms it closes exactly this item's ask
- skip_shipped: driver-quota-hang-wedge — shipped as PR #295 (`aaaa0cb1`, ADR-0070 SignalCenter exhaustion signal) + PR #296 (`26986cf7`, fast-fail on the ~2s poll); wait loop now returns ExitUnknownPrompt(85) on exhaustion, the exact fix this item specifies
- ingested-and-deferred: latest-model-preference, advisor-policy-centralization, advisor-emit-tier-dormant, egps-timeout-false-fail, bridge-integration-tests-flaky-ci, ship-phase-no-deliverable-contract (see ## deferred)
- schema note: the four 2026-07-02 items are architect-brief JSONs (title/description, lowercase or absent `priority`) rather than inject-task.sh schema; treated leniently as operator-queued MEDIUM/weight-ranked rather than rejected on a field technicality — rejecting operator briefs on schema shape would lose intent (surfaced per Rule 3, lenient reading chosen)
- lifecycle: inbox files left in place for the ship-phase inbox-lifecycle hook (consumes triage-decision.json); triage does not move main-tree files (cycle-390 tree-diff-guard lesson)
- skip_rejected: none · escalate_block: none

## carryoverTodos warnings (if any)
- none at defer_count ≥ 3 (all 34 records carry no defer_count field; cycles_unpicked=0)
- operator visibility: cycle-393 + cycle-448 are the same triage-overpack gate rejection two occurrences apart — this decision commits exactly 3 coverage floors (at cap) to break the pattern; recommend operator review if it recurs

## Rationale
Cycle 448 — same goal — was gate-rejected for committing 4 coverage floors against the capacity cap of 3, so this cycle commits exactly the three cycle-444 salvage tasks (core 85.0, bridge 94.5, audit 96.0 floors; ~125K of the 200K budget) and defers the only non-salvage task (router/llmroute/config) with its floors intact for next cycle. The salvage set is the highest-value/lowest-risk unit: all three test files passed audit 0.9 + adversarial review in cycle 444 and died only on the since-fixed EGPS timeout, and scout re-verified symbol drift is nil. Blocker-solo does not trigger: 448's failure was a triage-discipline defect fixed by this decision itself, not by any code item in the backlog. Size is medium (3 items, multi-file but coherent, copy-adapt of pre-audited tests); per-package predicate files keep EGPS runtime small per the cycle-352 lesson.
