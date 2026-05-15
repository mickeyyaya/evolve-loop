# Self-Evolution — How Failures Teach the Next Cycle

> The technical mechanism by which evolve-loop becomes incrementally better across cycles. Grounded in Argyris & Schon (1978) double-loop learning and Shinn et al. (2023) Reflexion. No magic — just durable, evidence-bound lessons that next-cycle Scout reads.
> Audience: people who've read [overview.md](overview.md) and want to understand WHY the system gets smarter.

## Table of Contents

1. [The Core Loop](#the-core-loop)
2. [Single-Loop vs Double-Loop Learning](#single-loop-vs-double-loop-learning)
3. [The Reflexion Connection](#the-reflexion-connection)
4. [Lesson YAML Format](#lesson-yaml-format)
5. [Instinct Lifecycle](#instinct-lifecycle)
6. [How Cycle N+1 Reads the Lesson](#how-cycle-n1-reads-the-lesson)
7. [Worked Example: Cycle 7 → Cycle 59 Lesson Generalization](#worked-example-cycle-7--cycle-59-lesson-generalization)
8. [Anti-Patterns: What Self-Evolution Is NOT](#anti-patterns-what-self-evolution-is-not)
9. [References](#references)

---

## The Core Loop

When cycle N's audit returns FAIL or WARN, the framework does NOT just record the bad outcome — it extracts a structured lesson that the next cycle's Scout will see.

```
┌──────────────────────────────────────────────────────────────────┐
│ Cycle N: Audit verdict = FAIL/WARN                               │
└──────────────────────┬───────────────────────────────────────────┘
                       ↓
        record-failure-to-state.sh $WORKSPACE $VERDICT
                       │
                       ↓
     ┌──────────────────────────────────────┐
     │ state.json:failedApproaches[]        │   ← single-loop (raw)
     │  - cycle, verdict, errorCategory     │
     │  - failedStep, lessonIds, systemic   │
     │  - expiresAt (30d default; ageable)  │
     └──────────────────────────────────────┘
                       ↓
       subagent-run.sh retrospective N $WORKSPACE
                       │
                       ↓
     ┌──────────────────────────────────────┐
     │ retrospective-report.md              │   ← prose narrative
     │ handoff-retrospective.json           │   ← machine handoff
     │ .evolve/instincts/lessons/<id>.yaml  │   ← durable lesson(s)
     └──────────────────────────────────────┘
                       ↓
         merge-lesson-into-state.sh $WORKSPACE
                       │
                       ↓
     ┌──────────────────────────────────────┐
     │ state.json:instinctSummary[]         │   ← double-loop (structured)
     │  - id, pattern, confidence, type     │
     │  - errorCategory                     │
     └──────────────────────────────────────┘
                       ↓
            ───── Cycle N+1 begins ─────
                       ↓
        scout reads state.json before generating scout-report.md
                       ↓
        instincts appear in Scout's prompt context
                       ↓
        Scout's "Selected Tasks" reflect the instinct
        OR Scout adds an instinct-based deferral to "Carryover Decisions"
```

The mechanism is documented in [`../architecture/retrospective-pipeline.md`](../architecture/retrospective-pipeline.md).

---

## Single-Loop vs Double-Loop Learning

Argyris & Schon (1978) distinguished two kinds of organizational learning:

| Loop | What it does | Example | evolve-loop mechanism |
|---|---|---|---|
| **Single-loop** | Detect error → correct action | "Tests failed → re-run with more verbose output" | `state.json:failedApproaches[]` (raw record of what went wrong) |
| **Double-loop** | Detect error → question the assumptions that led to it → change the policy | "Tests failed because the assertion logic was wrong → change how we write assertions" | `state.json:instinctSummary[]` (structured pattern + prevention rule) |

Pre-v8.45.0, evolve-loop only had the single-loop. Cycles failed → entries piled up in `failedApproaches[]` → next cycle saw the noise but had no actionable rule to follow. v8.45.0 made the retrospective auto-fire on FAIL/WARN, completing the double-loop.

The cycle-59 retrospective output (preserved in `.evolve/instincts/lessons/cycle-59-acs-predicates-worktree-invisible.yaml`) is a perfect example of double-loop output: it doesn't say "predicate 031 RED'd"; it says "ACS predicates written inside a worktree are invisible to the project-root ACS suite. Always copy current-cycle predicates to PROJECT_ROOT before audit." The next cycle's Builder gets a rule, not a war story.

---

## The Reflexion Connection

Shinn et al. (2023) introduced **Reflexion** — language agents that improve through verbal self-critique stored in long-term memory. The architecture:

```
Actor → Action → Environment → Evaluator → Self-Reflection → Memory → next Actor invocation
```

evolve-loop is a multi-agent Reflexion variant:

| Reflexion | evolve-loop equivalent |
|---|---|
| Actor | Builder (writes code + predicates) |
| Action | Worktree edits, predicate writes |
| Environment | git + filesystem + ACS predicate suite |
| Evaluator | Auditor + `acs-verdict.json` exit codes |
| Self-Reflection | Retrospective subagent (different model from Builder; auto-fires on FAIL/WARN) |
| Long-term memory | `state.json:instinctSummary[]` + `.evolve/instincts/lessons/<id>.yaml` |
| Next Actor invocation | Cycle N+1 Scout reads the memory |

The crucial difference from a simple Reflexion implementation: **the verdict is not a model claim** (Reflexion's Evaluator is typically an LLM). evolve-loop's verdict is `acs-verdict.json:red_count == 0`, derived from bash exit codes. The Auditor model writes prose but cannot game the predicate suite — that's the EGPS layer described in [trust-architecture.md](trust-architecture.md) and [`../architecture/egps-v10.md`](../architecture/egps-v10.md).

---

## Lesson YAML Format

Every lesson is one file at `.evolve/instincts/lessons/<id>.yaml` with this schema:

```yaml
id: cycle-N-<slug>
cycle: N
timestamp: "YYYY-MM-DD"
classification: code-audit-fail | code-audit-warn | unknown-classification
pattern: "one-line root-cause pattern for future matching"
lesson: "what should be done differently"
prevention: "concrete step to prevent recurrence"
instinct:
  - "one instinct per bullet"
priority: HIGH | MEDIUM | LOW
```

The `instinct[]` bullets are what Scout will see verbatim in its next-cycle prompt context. They should be imperative and testable, not vague:

| GOOD (Scout can act on this) | BAD (Scout can't tell when to apply this) |
|---|---|
| "Before audit, copy `worktree/acs/cycle-N/*.sh` to `PROJECT_ROOT/acs/cycle-N/` so the suite can see them." | "Be careful about file locations." |
| "When adding to `phase-registry.json:audit.inputs.state_fields[]`, grep every regression predicate that calls `check-phase-inputs.sh audit` and update fixtures." | "Watch out for breaking changes." |

The retrospective persona ([`agents/evolve-retrospective.md`](../../agents/evolve-retrospective.md)) is prompted to produce the imperative form. The merge script verifies the YAML schema before adding to `state.json`.

---

## Instinct Lifecycle

Lessons are durable but not forever. Each instinct has a lifecycle:

| Stage | Mechanism | When |
|---|---|---|
| **Birth** | Retrospective writes YAML + merge script appends to `state.json:instinctSummary[]` | Cycle N audit FAIL/WARN |
| **Application** | Cycle N+1 (and later) Scout reads `state.json` and includes instinct in prompt | Every subsequent cycle |
| **Reinforcement** | If a similar failure recurs, `failedApproaches[]` gets a new entry but `instinctSummary[]` entry's `confidence` may increment | Recurrence |
| **Generalization** | A retrospective may explicitly broaden an existing instinct's `pattern` field if its framing was too narrow | Operator-initiated or retrospective-recommended |
| **Aging-out** | `failedApproaches[]` entries have `expiresAt` (default 30 days); reaper script removes stale ones | Default cron |
| **Promotion** | High-confidence instincts that fire across many cycles may become permanent project rules (e.g., added to CLAUDE.md) | Operator-initiated |

Currently 19 active lesson YAMLs in `.evolve/instincts/lessons/` (as of v10.6+):

```
cycle-2-acceptance-literal-compliance.yaml
cycle-2-hybrid-masquerade.yaml
cycle-2-test-registration.yaml
cycle-7-ephemeral-worktree-artifact.yaml
cycle-7-ghost-complete-phase.yaml
cycle-8-acceptance-reframe-recurrence.yaml
cycle-8-triage-structural-enforcement.yaml
cycle-18-tautological-fixture.yaml
cycle-24-builder-uncommitted-worktree-edit.yaml     ← Cycle 64 re-triggered this in 2026-05-15
cycle-24-causal-claim-unverified-script.yaml
cycle-24-enforcement-step-before-producer-updated.yaml
cycle-29-builder-wrong-directory.yaml
cycle-36-dead-code-variable-scope.yaml
cycle-38-section-pattern-mismatch.yaml
cycle-40-ac-numerical-baseline.yaml
cycle-40-ac-taxonomy-anchor.yaml
cycle-40-builder-context-crosscheck.yaml
cycle-59-acs-predicates-worktree-invisible.yaml
cycle-59-registry-state-fields-fixture-impact.yaml
```

That `cycle-24-builder-uncommitted-worktree-edit.yaml` entry is what predicted (in 2026, cycle 24) the exact failure pattern that recurred in cycle 64's gemini re-test on 2026-05-15. The framework knew the failure mode by name. Next cycle's Builder should now apply it.

---

## How Cycle N+1 Reads the Lesson

The next cycle's Scout is dispatched with a prompt that includes a section like:

```
## Active Instincts (from state.json:instinctSummary[])

These are pattern-rules extracted from prior cycle failures. Apply them when
selecting tasks, planning the build, or estimating risk:

- [cycle-24-builder-uncommitted-worktree-edit] pattern="Builder edits a file in
  the worktree but does not run git add/commit; worktree cleanup discards the
  edit before ship.sh sees it." prevention="After every edit in the worktree,
  Builder must run `git -C $WORKTREE add <file> && git -C $WORKTREE commit -m
  '...'` before invoking ship.sh." confidence=0.92

- [cycle-59-acs-predicates-worktree-invisible] pattern="ACS predicates written
  inside the worktree branch are invisible to the project-root run-acs-suite.sh
  invocation; this_cycle_count=0 ⇒ predicate 031 RED." prevention="Builder must
  copy acs/cycle-N/*.sh to PROJECT_ROOT/acs/cycle-N/ BEFORE auditor runs."
  confidence=0.95
```

Scout's `## Carryover Decisions` section in `scout-report.md` may explicitly cite these instincts when deciding to defer a task ("instinct cycle-24-... warns this would recur — deferring until we add a structural commit-check").

The prompt building is in `scripts/dispatch/role-context-builder.sh`. Token cost per cycle for the instinct section is bounded — long-lived instincts that haven't fired recently get demoted out of the visible list to keep Scout's context focused.

---

## Worked Example: Cycle 7 → Cycle 59 Lesson Generalization

This is the most-cited example in the project's own retrospective work.

**Cycle 7** failed because `.evolve/` runtime artifacts (gitignored) were invisible to phase-gate after worktree cleanup. The lesson was filed as `cycle-7-ephemeral-worktree-artifact.yaml` with pattern: "*.evolve/* runtime artifacts in the worktree don't survive cleanup; phase-gate must reach them before cleanup fires.*"

**Cycle 59** failed in what looked like the same class of bug, but the lesson didn't catch it. Why? Because cycle-7's pattern was framed too narrowly: it talked about `.evolve/` runtime artifacts (gitignored), but cycle-59 was about `acs/` predicates (TRACKED — not gitignored) that lived in the worktree branch and weren't yet on `main`.

Cycle-59's retrospective explicitly recommended:

> The cycle-7 instinct existed but its framing was too narrow to catch this case. Expanding the instinct scope is the primary retrospective output.

So cycle-59 wrote a NEW lesson (`cycle-59-acs-predicates-worktree-invisible.yaml`) AND recommended the cycle-7 lesson be generalized to "any worktree-branch artifact not yet on main, gitignored or not."

This is the meta-pattern: **lessons themselves are first-class objects that can be refined over time.** The retrospective persona is explicitly prompted to check whether an existing instinct partially covers the new failure and, if so, broaden the existing one rather than write a redundant new one. See the [feedback memory note `feedback_skill_file_structure`](#) for the broader rule.

---

## Anti-Patterns: What Self-Evolution Is NOT

| Claim | Why it's wrong here |
|---|---|
| "The model improves its weights" | No — weights are frozen. The framework changes the *context* the model sees on the next run. |
| "The framework is conscious / agentic in a deep sense" | No — it's a deterministic pipeline that happens to be driven by LLM-generated content. The Reflexion-style loop is a mechanical pattern, not emergent cognition. |
| "Every cycle gets better" | No — cycles can regress. A bad lesson can mislead next-cycle Scout (which is why retrospective YAMLs require explicit prevention steps, not vague advice). |
| "It self-improves without human review" | Partially — for the routine learning loop, yes; but high-impact instincts are usually promoted to CLAUDE.md or shipped as structural fixes (predicates, gates) by humans before they become enforceable. |
| "It's like AutoML" | No — the framework is selecting and structuring *human-style* engineering work (refactors, bug fixes, doc updates), not searching hyperparameter spaces. |

---

## References

| Source | Relevance |
|---|---|
| Argyris, C. & Schon, D. (1978) *Organizational Learning: A Theory of Action Perspective* | The single-loop / double-loop distinction underpinning the failedApproaches / instinctSummary split. |
| Shinn, N. et al. (2023) "Reflexion: Language Agents with Verbal Reinforcement Learning" arXiv:2303.11366 | The actor / evaluator / self-reflection / memory pattern. evolve-loop's variant separates each role into a different agent + ledger-backed memory. |
| [`../architecture/retrospective-pipeline.md`](../architecture/retrospective-pipeline.md) | The script-level contract for `record-failure-to-state.sh` and `merge-lesson-into-state.sh`. |
| [`../architecture/phase-architecture.md`](../architecture/phase-architecture.md) — RETROSPECTIVE section | Per-phase mechanics of the retrospective subagent. |
| [`../incidents/cycle-61.md`](../incidents/cycle-61.md) §Retrospective + §Operator Lessons | Worked example: 8 bugs in cycle 61 → 8 structural fixes shipped via cycles 1-9 of v10.7.0. |
