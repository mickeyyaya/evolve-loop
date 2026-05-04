---
description: The macro — runs the full Scout → Plan-Review → TDD → Build → Audit → Ship → Retro lifecycle with trust-kernel-enforced phase ordering.
---

# /loop

Auto-orchestrated full lifecycle. Runs each phase in sequence; the trust kernel (sandbox + ledger SHA + phase-gate) enforces phase ordering at the script layer. This is **Pattern 5** from `docs/architecture/tri-layer.md` — the auto-orchestrated macro that's safe specifically because of the kernel.

## When to use

- Autonomous mode — when bypass-permissions is on and tasks should run end-to-end
- Multi-cycle dispatch (e.g., 30 cycles via `/loop` skill)
- Routine cycles where human checkpoints aren't needed

## When NOT to use

- One-off discovery (use `/scout` instead)
- When you want to inspect each phase's output before advancing (use individual commands)
- High-risk architectural cycles (use `/scout → /plan-review → /tdd → ...` step by step)

## Execution

The `/loop` skill (existing, in `skills/evolve-loop/`) drives the sequence. With Sprint 1+2+3+4 active:

```
/intent (single — structures user goal; v8.19.1+ always-on for /evolve-loop)
  ↓ awn_class != IBTC AND >=1 challenged_premise
/scout (fan-out: 3 workers)
  ↓
/plan-review (fan-out: 4 lenses) [if EVOLVE_PLAN_REVIEW=1]
  ↓ Verdict: PROCEED
/tdd (single)
  ↓
/build (single, in worktree)
  ↓
/audit (fan-out: 4 sub-auditors)
  ↓ Verdict: PASS
/ship (atomic)
  ↓
/retro (fan-out: 3 sub-reflectors)
```

## How intent integrates (autonomy-preserving)

When you type `/evolve-loop "your goal"`, the skill prepends `EVOLVE_REQUIRE_INTENT=1` to the dispatcher invocation. The intent persona structures your goal into `intent.md` (8 fields + AwN classifier + ≥1 challenged premise), the kernel verifies structure, then Scout proceeds against the structured intent rather than the raw goal text. **No human checkpoint, no pause** — the autonomy invariant is preserved end-to-end.

Direct CLI invocations (e.g., `bash scripts/evolve-loop-dispatch.sh ...`) remain opt-in via `EVOLVE_REQUIRE_INTENT=1` — only the slash-command path defaults intent on. This keeps existing test infrastructure unchanged.

## Why this is safe (vs. addyosmani's anti-pattern C)

addyosmani's `references/orchestration-patterns.md` lists "sequential orchestrator that paraphrases" as Anti-pattern C. `/loop` avoids the anti-pattern because:

1. **No paraphrasing at handoff** — phases pass artifacts (scout-report.md, audit-report.md) bound by SHA256 in the ledger, not summaries
2. **Trust kernel enforces ordering** — `phase-gate-precondition.sh` blocks out-of-order calls; no orchestrator drift possible
3. **Each phase has a hard verdict** — PASS/FAIL/PROCEED/REVISE/ABORT — not a fuzzy summary

This is why evolve-loop earns Pattern 5 (auto-orchestrated macro) where most projects should stick with Pattern 4 (user-driven sequence).

## See also

- `skills/evolve-loop/SKILL.md` (the macro implementation)
- `docs/architecture/tri-layer.md` (Pattern 5 justification)
- `CLAUDE.md` "Autonomous Execution" section
