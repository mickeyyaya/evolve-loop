# Incident Report: Turn-Overrun — Cycle 69

> Severity: WARN | Status: Resolved | Cycle: 69 → 70

## 1. What Happened

Cycle 69 recorded two turn-overrun events in `.evolve/runs/cycle-69/abnormal-events.jsonl`:

| Phase | Actual Turns | Max Turns | Overshoot |
|---|---|---|---|
| Scout | 44 | 15 | 3× (29 excess turns) |
| Builder | 26 | 25 | 4% (1 excess turn) |

Both agents completed their tasks correctly. Cycle 69 shipped as **PASS** (commit `92193ee`). The overrun is a calibration gap, not a reliability or integrity gap.

## 2. Root Cause

### Scout 3× overshoot (primary concern)

Cycle 69's Scout performed a comprehensive P1–P8 roadmap audit spanning multiple docs and knowledge-base files before selecting P4 (anchored intent injection). The 15-turn ceiling was calibrated for a **narrow targeted-lookup task** (e.g., "read state.json → propose next P-item"). The actual task was a **multi-stream breadth-first audit**: enumerate token-economics-2026.md (the full roadmap table), cross-check P5 status against `lesson-template.yaml`, read multiple knowledge-base research dossiers, and verify which P-items had been shipped.

The Scout's exhaustive reading pattern — reading entire referenced docs rather than stopping after capturing sufficient evidence — caused the overrun. This is a **prompt/calibration gap**, not a logic error.

### Builder marginal overshoot (minor, expected)

Cycle 69's Builder implemented P4 across 4 files (intent persona, role-context-builder.sh, token-economics doc, knowledge-base research dossier). 26 turns for 4 new/modified files is at the structural ceiling. The STOP CRITERION and hard exit trigger were present but the builder reached 26 turns because the final `build-report.md` write pushed one turn past the 25-turn hard ceiling. This is within tolerable calibration range.

## 3. Reasoning

The Scout's 15-turn ceiling was set assuming sequential single-file reads. A multi-stream audit that reads 8+ files across 4+ streams naturally requires 30–50 reads. Two fixes are possible:

1. **Raise max_turns for scout** — solves the symptom, increases cost on simple cycles
2. **Add stop_criterion guidance** — trains scout to stop each stream after sufficient evidence, not after exhaustive reading

Option 2 is preferred: it keeps the 15-turn ceiling appropriate for narrow tasks while guiding scouts on multi-stream tasks to bound their per-stream reads. The `stop_criterion` field in `scout.json` provides this guidance.

## 4. Fix

**Cycle 70 (this fix):**

1. Added `stop_criterion` to `.evolve/profiles/scout.json`: explicit guidance that multi-stream breadth-first audits should read 1–2 files per stream, 3–4 for the primary stream, and stop once a task proposal is ready.

2. Added `turn_budget_guidance` to `.evolve/profiles/builder.json`: structured checkpoint at turn 15 with enumeration protocol. Builder persona updated with Budget Checkpoint Protocol section.

**No ceiling change required.** The 15-turn ceiling is appropriate for typical scout tasks. Multi-stream audit tasks should scope more tightly, not get a higher ceiling.

## 5. Lessons

| Lesson | Applies To |
|---|---|
| Stop each stream after sufficient evidence — not after exhaustive reading | Scout persona, scout.json stop_criterion |
| A multi-stream breadth-first audit requires a proportionally larger turn budget than a targeted lookup | Scout task sizing, orchestrator prompt |
| Builder hitting 25-turn ceiling by 1 turn is calibration variance, not a failure | No action — within tolerance |
| Overrun ≠ integrity failure; cycle shipped PASS with correct artifacts | Trust-kernel invariants confirmed |

## 6. References

- Abnormal events: `.evolve/runs/cycle-69/abnormal-events.jsonl`
- Scout report (cycle 70): `.evolve/runs/cycle-70/scout-report.md` §"Stream A — Carryover Todos"
- Fix: `.evolve/profiles/scout.json:stop_criterion`, `.evolve/profiles/builder.json:turn_budget_guidance`, `agents/evolve-builder.md` §"Budget Checkpoint Protocol"
- Cycle 69 PASS commit: `92193ee`
