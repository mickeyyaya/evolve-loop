# ADR-0158: A repair round runs only for a finding that the lane can fix, and the auditor applies a small prescribed fix

- **Status:** Proposed (2026-10-27). It amends [ADR-0155](0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md) decision 10, [ADR-0157](0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md) decisions 8 to 10 and §5 of the [retry-ladder spec](../retry-ladder.md). The amended files get their "Amended by ADR-0158" note when the operator accepts this ADR.
- **Plan:** [cost-aware-repair-2026-10](../../plans/cost-aware-repair-2026-10.md). **Research:** [cost-aware-repair-2026-10](../../research/cost-aware-repair-2026-10.md). **Case:** [cycle 2035](../../incidents/2026-10-27-cycle-2035-repair-loop.md). **Design:** [retry-ladder.md](../retry-ladder.md) §5.
- **Relates to:**
  - [ADR-0126](0126-every-iterative-loop-converges-or-escalates.md): a loop converges or escalates. This ADR escalates at once when convergence is not possible.
  - [ADR-0142](0142-a-capability-failure-climbs-one-bounded-retry-ladder.md): the ladder keeps the climb. `audit_fix` is a rung of its decider, and its handler plugs into `core.RungHandler` ([#1173](https://github.com/mickeyyaya/evolve-loop/pull/1173)).
  - [ADR-0155](0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md): I-11 stays. The auditor that writes a patch does not judge it.
  - [ADR-0157](0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md): a patch on a fence path is a held proposal.
- **Evidence:**
  - Cycle 2035 spent 65% of its 160.1 LLM minutes after an audit that named a protected blocker (F1.1, F1.2, F1.3).
  - The repair decision reads only the failure class, and it restarts tdd and build for any fix (F1.4, F1.5).
  - Triage admitted the same item ten times, for 628 LLM minutes (F1.12).
  - The audits missed two HIGH regressions and three empty tests that one diff review found (F1.9, F1.10).
  - Practice stops on a failed precondition, budgets the retries and lets a second person review a small applied patch (P2.4, P2.6, P2.8, P2.13).

## Context

The operator said on 2026-10-27: "I need to balance to cost and output otherwise, this project is no use." Cycle 2035 shows the waste. The first audit named the blocker and its fix: a console item on a protected surface. The pipeline still ran two repair rounds of tdd and build, and they did not converge.

Two gaps cause this. The repair decision has no input that says "the lane cannot fix this". The only repair is a full rebuild, also for a fix of a few lines. The audit also spent its rounds on the gate results and missed the defects in the diff.

## Decision

### Lane fixability

1. Before triage admits an item and before each repair round, sort each finding: lane or hand-off.
2. A hand-off finding cites a fence path (`guards.IsProtectedSurface`) or names an open console item.
3. A hand-off finding that blocks stops the cycle at once. No repair round starts.
4. A finding blocks if a gate fails on it, the audit proves it red, or it is HIGH or above.
5. The stop seals the cycle `handed-off` with the reason `handoff_blocker`, the salvage pointer, the findings and the console item.
6. Triage defers a lane item while a console item that its acceptance needs is open.

### The audit-fix rung

7. `ChooseAuditFix` judges only the lane findings. A hand-off finding that does not block goes to the console item.
8. When it chooses `audit_fix`, the auditor applies the patch in its loaded session.
9. The patch changes only the files that the prescriptions cite, inside the `audit_fix` bounds.
10. A fresh seat of another family reviews only the patch and the prescriptions, at the deep tier.
11. The handoff floor and the ship importer backstop run again on the patched tree.
12. If the review or a gate fails, one escalation runs: a raised build with the L6 digest.
13. If the escalation fails, the cycle stops and hands off, as decision 5 says.

### The spend limit of a cycle

14. Each cycle has a spend limit of LLM minutes, fresh tokens, cache-read tokens and audit rounds.
15. The orchestrator checks the spend limit at each phase boundary, from `llm-calls.ndjson`.
16. At the limit, the cycle stops and hands off, as decision 5 says.
17. The lane deadline stays a hang guard. Its percentile counts only the times of shipped lanes.

### Cheap checks first

18. The build session runs the handoff floor checks and gets their findings before it closes.
19. A guard refuses a new or changed test function with an empty body.
20. The audit gets the gate results as facts. It does not judge them again.
21. The audit reviews the final diff for regressions and asks of each guard fix: "is this fix honest?"

### Supersede

22. When the console claims the item of a live cycle, the cycle stops at its next phase boundary.
23. That stop seals the cycle `superseded`, with the salvage pointer.

### Stages and proof

24. Each component starts at `shadow`. At `shadow` it records its effect and changes no flow.
25. A component moves to `enforce` after one wave at `shadow` and an operator flip.
26. The flip needs zero false stops: no `would` stop on a cycle that then shipped.

## Policy keys

The defaults live in `go/internal/policy`, because the checked-in `.evolve/policy.json` is on the fence (`integrity_surface.go:54`). Strict policy applies: each key has a typed reader with a range check. A value outside its range, or an unknown stage, refuses the load with the key name, as `validateRetryLadder` does (`go/internal/policy/workflow_retry_ladder.go:41`). Each key gets a test in `go/cmd/evolve/policy_readers_strict_test.go` and a row in [policy-config.md](../policy-config.md).

| Key | Values | Default | Decisions |
|---|---|---|---|
| `workflow.repair.fixability.stage` | `off`, `shadow`, `enforce` | `shadow` | 1 to 6 |
| `workflow.retry_ladder.audit_fix.stage` | `off`, `shadow`, `enforce` | `shadow` | 7 to 13 |
| `workflow.retry_ladder.audit_fix.max_files` | 0 to 50 | 3 | 9 (from #1173) |
| `workflow.retry_ladder.audit_fix.max_lines` | 0 to 2000 | 40 | 9 (from #1173) |
| `workflow.retry_ladder.audit_fix.max_code_lines` | 0 to 2000 | 10 | 9 (from #1173) |
| `workflow.retry_ladder.audit_fix.max_tokens` | 0 to 64000 | 6000 | 9 (from #1173) |
| `workflow.retry_ladder.audit_fix.no_new_production_logic` | `true`, `false` | `true` | 9 (from #1173) |
| `workflow.cycle_spend.stage` | `off`, `shadow`, `enforce` | `shadow` | 14 to 16 |
| `workflow.cycle_spend.max_llm_minutes` | 10 to 600 | 90 | 14 |
| `workflow.cycle_spend.max_fresh_tokens` | 100000 to 20000000 | 2500000 | 14 |
| `workflow.cycle_spend.max_cache_read_tokens` | 1000000 to 500000000 | 40000000 | 14 |
| `workflow.cycle_spend.max_audit_rounds` | 1 to 4 | 2 | 14 |
| `fleet.orchestrator.deadline_basis` | `all`, `shipped` | `all` | 17 |
| `workflow.builder_selfcheck.stage` | `off`, `shadow`, `enforce` | `shadow` | 18 |
| `workflow.supersede.stage` | `off`, `shadow`, `enforce` | `shadow` | 22, 23 |

- The block is `workflow.cycle_spend`, not `workflow.cycle_budget`. That key already exists: it is the stage of the loop end decision (`go/internal/policy/workflow.go:14`, package `cyclebudget`).
- "Fresh tokens" are input, output and cache-write tokens. Cache-read tokens have their own limit, because they cost less and grow with the context.
- Each default is 1.5 to 2 times the largest value of a shipped cycle in waves 141 and 142 (research F1.15). The largest values are 47.0 LLM minutes (cycle 2033), 1.70 M fresh tokens and 26.7 M cache-read tokens (cycle 2036).
- At `deadline_basis` `all`, the loop log prints both deadlines. The operator sets `shipped` after the shadow wave.
- Decisions 19 to 21 have no key. The guard of decision 19 starts with a baseline of today's offenders, so it is safe at once. Decisions 20 and 21 are persona and prompt data.
- The one escalation of decision 12 is a fixed rule, not a key.
- The `audit_fix` rung reads its own stage. Thus it can run at `enforce` while `workflow.retry_ladder.stage` stays at `shadow`.

## Metrics that prove it

| Metric | Source | Before | Target |
|---|---|---|---|
| LLM minutes for each shipped cycle | `llm-calls.ndjson` and the ship records of a wave | 101.2 (waves 141, 142) | 75 or less |
| Fresh and cache-read tokens for each shipped cycle | the same | not measured for each wave | report, then set |
| Audit rounds for each cycle | `audit_dispatches` in `cycle-state.json` | 4 in cycle 2035 | 2 or fewer |
| Convergence rate | shipped cycles / started cycles | 5 of 8 in waves 141 and 142 | no fall |
| Repair rounds that start with a hand-off finding that blocks | the fixability signal | 2 in cycle 2035 | 0 |
| False stops | `would` stops on a cycle that then shipped | not measured | 0 |

## Amendments

### ADR-0155

- **Decision 10 (I-13).** "Two failed repairs escalate" stays as the upper bound. A hand-off finding that blocks escalates after zero repairs. The spend limit adds a limit for each cycle beside the budgets for each window.
- **I-11.** It stays as written. The auditor that applies a patch does not judge it. A fresh seat of another family does (decision 10).
- **Decision 15 (surface tiers).** A finding on a T0 path is a hand-off finding (decision 2).

### ADR-0157

- **Decisions 8 to 10.** When a cycle works a held proposal, a fence path in a finding is not a hand-off finding. The `audit_fix` patch on that path joins the held proposal. It lands only after `evolve landing approve`, the operator verb of ADR-0157 (`go/cmd/evolve/cmd_landing.go`).
- When the cycle is not a held proposal, a fence path stays a hand-off finding.

### Retry-ladder spec §5

- The rule "the seat of the auditor never does the fix" changes. In an `audit_fix` round, the auditor applies the patch, and a fresh judge reviews it.
- `retryladder.Climbs` stays false for the auditor. The audit fix is not a climb.
- The order of the rules of `ChooseAuditFix` gets one rule first: a hand-off finding that blocks returns `handoff_blocker`, and the round stops.
- `AllPrescribed` counts only the lane findings.
- "1 correction, then escalate" (#1169) stays.

## Consequences

Good:

- The estimate of the plan is that cycle 2035 stops at triage after 12.3 LLM minutes, not 160.1. The wave cost goes from 101.2 to 71.7 LLM minutes for each shipped cycle.
- A small prescribed fix costs one patch and one review, not a tdd and build round of about 50 LLM minutes.
- The audit reads the diff, where F1 and F2 were.
- The empty-test path to pass a guard is closed.

Bad:

- More cycles end `handed-off`, so the console gets more items. In cycle 2035 the console needed 44 minutes for the work.
- The auditor writes code. The bounds, the second seat and the gate rerun are the controls. The patch is a new risk surface.
- Eleven items to build. Four of them are console-manual, and one of those wires the four pure lane packages into the cycle.
- The spend limit stops a good cycle that runs long. The salvage keeps its work.

What changes this decision: if the shadow wave records a false stop, revisit decisions 3 and 4. If `audit_fix` records `would` on fewer than 1 cycle in 20, revisit decisions 7 to 13.

## Rejected

| Option | The fact that rules it out |
|---|---|
| More repair rounds or a larger retry budget | Self-repair gains decay within a few tries (P2.11, P2.14). Cycle 2035 had no path to converge (F1.3). |
| A higher model for each repair round | A stronger model does not reach a protected file (F1.3). It costs more for each round (P2.10). |
| The auditor applies and also approves its patch | Meta saw reviews take more than 5% longer when the reviewer saw AI patches (P2.2). SLSA asks two parties for the final revision (P2.4). |
| A spend limit alone | It saves 70 LLM minutes in cycle 2035, but it also stops good cycles, and it does not tell the console why (F1.15). |
| A shorter lane deadline as the cost stop | The deadline comes from lane times and grows with each failure (F1.11). Time is not tokens. |
| Remove the audit for small changes | Deterministic gates do not find the regressions F1 and F2 (F1.9). The audit changes its focus instead. |
