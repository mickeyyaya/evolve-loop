# Incident Report: Turn-Overrun — Cycle 95

> **Severity:** WARN | **Status:** Resolved (cycle 99) | **Carryover ID:** `abnormal-turn-overrun-c95`
> **Cycle:** 95 | **Deferred:** 2 cycles (96, 98) before resolution in cycle 99

---

## 1. What Happened

Cycle 95 recorded three abnormal events in `.evolve/runs/cycle-95/abnormal-events.jsonl`:

| Event | Timestamp | Details |
|---|---|---|
| `turn-overrun` | 2026-05-20T05:53:06Z | Builder: actual_turns=35, max_turns=25 (40% overrun) |
| `stall-detected` | 2026-05-20T06:02:44Z | idle_s=250, threshold_s=240; last file: orchestrator-stdout.log |
| `ship-refused` | 2026-05-20T07:20:53Z | git HEAD moved since audit (audited=89f2d08c, current=ad07d259) |

The turn-overrun was logged by `subagent-run.sh` with remediation hint: "Review builder-report.md for scope; split task or tighten STOP CRITERION." Cycle 95 ultimately **shipped as PASS** (commit records auditor mastery gate + O-1 fast-fail counter scope fix).

A separate anomaly during the same 24-hour window: 26 `subscription-auth-mode MISCONFIGURED` events appeared in `.evolve/abnormal-events.jsonl` spanning 2026-05-19T07:31Z to 2026-05-20T05:35Z. These had `cycle: 0`, indicating they were emitted during the pre-cycle auth probe (`doctor-subscription-auth.sh`), not during cycle-95 execution itself.

---

## 2. Research

### Evidence examined

- `.evolve/runs/cycle-95/abnormal-events.jsonl` — primary evidence (3 events listed above)
- `.evolve/instincts/lessons/cycle-95-auditor-mastery-gate.yaml` — describes cycle-95 deliverables: subagent-run.sh +75/-22 lines (mastery gate), `agents/evolve-auditor.md` +2 lines (preamble note), ACS predicates for both features
- `.evolve/abnormal-events.jsonl` (project-root) — 26 MISCONFIGURED events confirmed at `cycle: 0`
- Prior incident: `docs/operations/incidents/turn-overrun-c69.md` — Scout overrun (3× ceiling) resolved by `stop_criterion` in `scout.json`; Builder marginal overrun (1 turn) accepted as calibration variance
- `scripts/dispatch/subagent-run.sh` — turn counter is derived from Claude Code's reported `num_turns`; counter is NOT reset between retries within a phase invocation

### What cycle-95 delivered

Cycle-95's Builder implemented two features in one pass:

1. **P2 — Auditor mastery gate:** `subagent-run.sh` gained a mastery-conditional model-tier selector (75 lines added, 22 removed) reading `state.json:mastery.consecutiveSuccesses` to choose Sonnet vs Opus at auditor launch.
2. **O-1 (bundled carryover):** Per-agent vs per-workspace fast-fail counter scope clarification (additional lines across `subagent-run.sh` + documentation).

Combined scope: a 97-line net delta to `subagent-run.sh` plus persona file updates and two ACS predicates.

---

## 3. Reasoning

### Primary root cause: dual-feature scope exceeded single-cycle Builder budget

The 25-turn Builder ceiling was calibrated for single-feature cycles (~20-30 LOC core change). Cycle-95 bundled two independent features (P2 + O-1 carryover) into one Builder pass. The `subagent-run.sh` delta alone (+75/-22 = net 53 new lines) places it in the upper tier of medium-complexity changes. When ACS predicates, persona preamble edits, and documentation updates are added, the effective scope was comparable to a "medium" cycle that should have used ~30 turns.

Quantitatively: at ~1.4 turns/file-edit typical, a cycle editing subagent-run.sh (1 file), evolve-auditor.md (1 file), 2 ACS predicates, and build-report.md = 5+ files × ~1.4 + overhead ≈ 30–35 turns. The 25-turn ceiling was optimistic for this scope.

### Secondary event: stall-detected (causally linked)

The stall at turn ~35 (idle_s=250 > threshold_s=240) is consistent with the Builder hitting the turn ceiling and awaiting orchestrator intervention. The orchestrator stdout log was the last active file, suggesting the orchestrator was processing the Builder's final output when the stall timer fired. Not a separate failure — a downstream consequence of the overrun.

### Third event: ship-refused (independent, not caused by overrun)

The HEAD-mismatch ship refusal (audited=89f2d08c, current=ad07d259) occurred ~1.5 hours after the turn-overrun. This indicates the orchestrator made additional commits after the Auditor completed its binding (most likely a memo phase or a manual operator edit). This is a known race condition documented in `scripts/lifecycle/ship.sh` — audit binding captures HEAD at audit time; if HEAD advances before `ship.sh` runs, ship is refused. The turn-overrun is not a contributing cause.

### MISCONFIGURED auth events: environmental, not causal

The 26 MISCONFIGURED events at `cycle: 0` occurred before cycle-95 launched (spanning ~22 hours preceding the 05:53Z overrun event). They represent repeated `doctor-subscription-auth.sh` invocations during a period when `ANTHROPIC_API_KEY` was absent and `~/.claude.json` OAuth was not yet confirmed active. They did NOT consume Builder turns (they ran outside any phase invocation). The coincidence in timing is explained by the operator debugging auth during the same session window.

---

## 4. Fix

### Immediate (no code change required — pattern already partially addressed)

Cycle-96 (`cycle-96-builder-turn-18-stop.yaml` lesson) shipped a Builder stop criterion at turn 18: enumeration protocol requiring the Builder to assess remaining scope and either checkpoint or split. This directly addresses the failure mode — a Builder that reaches turn 18 on a dual-feature scope can flag the overrun risk before hitting the ceiling.

The cycle-95 overrun predates the turn-18 stop criterion; the stop criterion is the correct structural fix and is now in place.

### Recommended: scope guard in triage for dual-feature bundles

When the triage phase detects two independent features bundled as carryover + primary task, it should emit a `cycle_size_estimate: medium` even if each feature individually would be `small`. The current triage sizing logic appears to evaluate tasks independently, not additively. A future cycle should audit `agents/evolve-triage.md` for this gap.

### Carryover resolution

Carryover item `abnormal-turn-overrun-c95` (HIGH priority, deferred cycles 96 and 98) is **resolved** by this incident analysis. Root cause identified (dual-feature scope), structural fix already shipped (cycle-96 turn-18 stop criterion). No further action required unless the triage additive-scope gap is promoted to a future cycle task.

---

## 5. Lessons

| Lesson | Scope |
|---|---|
| Dual-feature bundles (primary + carryover) must trigger `medium` triage sizing even if each feature is individually `small` | `agents/evolve-triage.md` additive-scope heuristic |
| The cycle-96 turn-18 stop criterion is the correct preventive control; confirm it is present and tested before deferring turn-overrun carryovers | Builder persona + ACS regression suite |
| MISCONFIGURED auth events at `cycle: 0` are environmental noise from `doctor-subscription-auth.sh` — they do not consume phase turns and should not be treated as causally linked to overruns | Incident triage heuristics |
| `stall-detected` after a turn-overrun is a downstream symptom, not an independent root cause | Abnormal-event classifier |
| `ship-refused` due to HEAD movement is independent of turn-budget issues; check for post-audit commits before attributing to Builder | Ship-refusal triage playbook |

---

## 6. References

- Primary evidence: `.evolve/runs/cycle-95/abnormal-events.jsonl`
- Project-level anomaly log: `.evolve/abnormal-events.jsonl` (26 MISCONFIGURED events, cycle=0)
- Cycle-95 lesson (shipped deliverables): `.evolve/instincts/lessons/cycle-95-auditor-mastery-gate.yaml`
- Cycle-96 fix (turn-18 stop criterion): `.evolve/instincts/lessons/cycle-96-builder-turn-18-stop.yaml`
- Prior turn-overrun precedent (cycle-69): `docs/operations/incidents/turn-overrun-c69.md`
- Builder turn-budget guidance: `.evolve/profiles/builder.json:turn_budget_guidance`
- Ship-gate HEAD-mismatch behavior: `scripts/lifecycle/ship.sh` (ship-binding invariant)
- Carryover record: `.evolve/runs/cycle-95/carryover-todos.json` (`abnormal-turn-overrun-c95`, HIGH)
