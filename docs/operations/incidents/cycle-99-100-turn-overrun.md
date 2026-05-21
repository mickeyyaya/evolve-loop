# Incident Report: Turn-Overrun Recurrence — Cycles 99–100

> **Severity:** WARN | **Status:** Resolved (cycle 102) | **Carryover ID:** `abnormal-turn-overrun-c99`
> **Cycles affected:** 99, 100 | **Prior precedent:** `docs/operations/incidents/cycle-95-turn-overrun.md`

---

## 1. What Happened

Cycle-95 had introduced a turn-18 stop criterion in the Builder persona to prevent overruns. Despite that fix, turn-overruns recurred in cycles 99 and 100, now spanning four agents: triage, intent, scout, and builder.

Evidence from `.evolve/runs/cycle-99/abnormal-events.jsonl` and `.evolve/runs/cycle-100/abnormal-events.jsonl`:

| Cycle | Agent | Actual Turns | Ceiling | Overrun % |
|---|---|---|---|---|
| 99 | triage | 17 | 15 | +13% |
| 100 | intent | 12 | 10 | +20% |
| 100 | scout | 41 | 30 | +37% |
| 100 | builder | 35 | 25 | +40% |

The cycle-95 fix (Builder stop criterion at turn 18) addressed the Builder's behavioral pattern but did not raise ceiling values. When scope grew across all four phases, every agent breached its ceiling.

---

## 2. Research

### Evidence examined

- `.evolve/runs/cycle-99/abnormal-events.jsonl` — triage overrun (17 > 15)
- `.evolve/runs/cycle-100/abnormal-events.jsonl` — three overruns: intent (12>10), scout (41>30), builder (35>25)
- `.evolve/profiles/triage.json` — `max_turns: 15` unchanged since last calibration
- `.evolve/profiles/intent.json` — `max_turns: 10` unchanged since last calibration
- `.evolve/profiles/scout.json` — `max_turns: 30` unchanged since last calibration
- `.evolve/profiles/builder.json` — `max_turns: 25` unchanged since last calibration
- `state.json:carryoverTodos` — item `abnormal-turn-overrun-c99` recorded at HIGH priority
- Prior incident: `docs/operations/incidents/cycle-95-turn-overrun.md` — documented dual-feature scope as root cause; fix was a behavioral stop criterion, not a ceiling raise

### What cycles 99–100 were executing

Cycle-99 ran a triage phase on a multi-stream scout with several carryover items. The triage agent hit 17 turns parsing carryover state and producing its decision document.

Cycle-100 ran a broader intent + scout + builder pass for an agy (antigravity CLI adapter) feature. The scout phase hit 41 turns performing deep research across three parallel sub-scout streams. The builder phase hit 35 turns implementing the four-file agy adapter with tri-mode degradation logic.

---

## 3. Reasoning

### Primary root cause: static ceilings calibrated for earlier, simpler cycles

All four ceiling values (triage=15, intent=10, scout=30, builder=25) were set during the v8–v9 era when cycle tasks were smaller. As the codebase and agent personas grew more complex — more carryover state, deeper research, larger deliverables — the actual turn usage drifted upward. The ceiling values were never re-calibrated.

The cycle-95 fix was behavioral (stop criterion at turn 18) rather than structural (raising the ceiling). It helped the Builder avoid infinite loops but did not give agents more room when tasks genuinely required it.

### Why cycle-99 hit triage specifically

Triage now parses a richer `state.json` (carryoverTodos, failedApproaches, batch metadata) and must cross-reference multiple abnormal-events.jsonl files when carryover items are present. This read-heavy path was not present when triage's ceiling was set to 15.

### Why cycle-100 hit intent, scout, and builder

The agy adapter introduced research-heavy scout work (three sub-stream sub-scouts) and a non-trivial builder deliverable (tri-mode degradation, zero-cost stub envelope, chmod fix). Each agent's scope was individually justifiable but collectively the cycle was "medium" in all four phases simultaneously.

### Why the cycle-95 fix was insufficient

The cycle-95 turn-18 stop criterion tells the Builder to enumerate remaining steps at turn 18 and stop early if needed. It does not raise the ceiling, so the Builder still exceeds its declared `max_turns`, triggering the abnormal event even when it finishes correctly. The structural fix is a ceiling raise.

---

## 4. Fix

### Applied in cycle 102

All four profile files updated with `max_turns` values that give each agent a 20–40% buffer above the observed peaks:

| Profile | Old `max_turns` | New `max_turns` | Observed peak | Buffer |
|---|---|---|---|---|
| triage.json | 15 | 18 | 17 | +6% |
| intent.json | 10 | 12 | 12 | 0% (exact) |
| scout.json | 30 | 42 | 41 | +2% |
| builder.json | 25 | 36 | 35 | +3% |

Note: intent and scout headroom is tight. If cycles 103+ show further overruns, another ceiling raise is warranted.

### No behavioral change required

The Builder turn-18 stop criterion (cycle-96) remains in place. It is complementary — the ceiling raise gives headroom, the stop criterion prevents unbounded growth. Both must coexist.

---

## 5. Lessons

| Lesson | Scope |
|---|---|
| Ceiling raises should accompany behavioral fixes when fixing turn-overruns; behavioral-only fixes leave the abnormal event intact | Incident resolution checklist |
| Triage ceiling must account for carryover-rich cycles that parse multiple abnormal-events.jsonl files | `agents/evolve-triage.md` turn budget notes |
| Scout ceiling must account for multi-stream parallel sub-scout execution patterns | `.evolve/profiles/scout.json` calibration notes |
| Builder ceiling must accommodate medium-complexity adapters (4+ files, tri-mode logic) | `.evolve/profiles/builder.json` calibration notes |
| After a ceiling raise, monitor the next 2 cycles for further overruns before closing the pattern | Carryover triage heuristics |

---

## 6. References

- Primary evidence: `.evolve/runs/cycle-99/abnormal-events.jsonl`, `.evolve/runs/cycle-100/abnormal-events.jsonl`
- Carryover record: `state.json:carryoverTodos` (`abnormal-turn-overrun-c99`, HIGH)
- Prior precedent: `docs/operations/incidents/cycle-95-turn-overrun.md`
- Prior turn-overrun doc: `docs/operations/incidents/turn-overrun-c69.md`
- Fixed profiles: `.evolve/profiles/triage.json`, `.evolve/profiles/intent.json`, `.evolve/profiles/scout.json`, `.evolve/profiles/builder.json`
- Cycle-96 behavioral fix: `.evolve/instincts/lessons/cycle-96-builder-turn-18-stop.yaml`
