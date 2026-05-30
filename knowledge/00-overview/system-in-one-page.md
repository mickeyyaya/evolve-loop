# Evolve Loop — The System in One Page

> Start here. This page gives you the mental model, the cycle lifecycle, and the
> invariants that everything else depends on. If you read only one file, read this one.
> Terms in `code font` are defined in [glossary.md](glossary.md).

---

## The Mental Model

**Evolve Loop is a self-evolving development pipeline that runs your codebase
through autonomous improvement cycles — and structurally prevents the LLM driving
it from faking the result.**

Hold three ideas in your head and the rest follows:

1. **The system IS a Go binary (`evolve`, v13.0.0).** You run `evolve loop` and it
   orchestrates everything: sequencing phases, dispatching LLM subagents through a
   CLI-agnostic `bridge`, enforcing the trust kernel, and shipping commits. (The
   pipeline was originally a tree of bash scripts; that `legacy/scripts/` tree was
   removed in the Go-only consolidation — the Go binary is now the sole runtime, with
   no bash fallback. See [evolution/bash-to-go-port](../evolution/bash-to-go-port.md).)

2. **A cycle is an experiment, not a build step.** Each `cycle` is one pass through
   an ordered sequence of `phase`s. The LLM picks the work; the framework constrains
   *how* the work proceeds — the way scientific method constrains an experimenter.
   Every phase writes a markdown artifact the next phase must read, and every
   handoff is verified by SHA against a tamper-evident `ledger`.

3. **Verdicts are exit codes, not opinions.** A cycle ships only when its `EGPS`
   predicates — executable bash scripts whose *exit code is the verdict* — all pass
   (`red_count == 0`). An LLM can write a glowing audit report; it cannot make a
   RED predicate turn GREEN. This is the single most important design decision in
   the system.

What makes it *self-evolving*: when a cycle fails, a retrospective extracts a
durable, evidence-bound `lesson` (a YAML file). The next cycle's `Scout` reads it.
Cycle N's mistake teaches cycle N+200. This is the Reflexion loop (Shinn et al.,
2023) wired through verifiable storage — weights never change; the *context the
model sees next time* does.

---

## The Cycle Lifecycle

One cycle is an ordered walk through these phases. The trust kernel enforces the
order at the OS layer — phases cannot be skipped, reordered, or shortcut via prompt
text.

```
INTENT → SCOUT → TRIAGE → [PLAN-REVIEW] → [TDD] → BUILD → AUDIT → SHIP → MEMO/RETRO
```

| Phase | Role | Produces |
|---|---|---|
| **Intent** | Structure the vague goal into 8 fields + ≥1 challenged premise | `intent.md` |
| **Scout** | Find work (codebase + carryover + instincts); cite research; propose tasks | `scout-report.md` |
| **Triage** | Bound this cycle's scope: `top_n` / deferred / dropped | `triage-decision.md` |
| **Plan-Review** *(opt-in)* | 4-lens CEO/Eng/Design/Security review of the plan | `plan-review.md` |
| **TDD** *(default-on)* | Write RED predicates *before* code (separates author from implementer) | `acs/cycle-N/*.sh` |
| **Build** | Implement the minimum to turn predicates GREEN, in an isolated `worktree` | `build-report.md` |
| **Audit** | Adversarial cross-check; run the predicate suite; emit the verdict | `audit-report.md`, `acs-verdict.json` |
| **Ship** | If `red_count == 0`, commit + push through the `ship-gate` | a commit on `main` |
| **Memo / Retro** | PASS → capture carryover; FAIL/WARN → extract `lesson`s | `carryover-todos.json` *or* `lessons/<id>.yaml` |

Key properties of the walk:

- **Single-writer discipline.** Only one phase mutates the tree at a time. Builder,
  Intent, TDD, and the Orchestrator are excluded from parallel fan-out so writes
  never race.
- **Different-family audit.** The `Auditor` runs on a different model family from the
  `Builder` by default (e.g. Builder=Sonnet, Auditor=Opus). Same-model judges are
  sycophantic; cross-family breaks that.
- **Promotion on ship.** A shipped cycle's predicates promote to the
  `regression-suite` and become permanent guards every future cycle must keep GREEN.
- **Meta-cycle.** Every few cycles, a higher-order pass reflects on accumulated
  instincts and proposes structural changes.

How the walk is driven (v13.0.0): a static state machine is the default driver, but
the `PhaseAdvisor` / `dynamic routing` kernel can compute a whole-cycle plan and
drive run/skip decisions — always clamped by a non-configurable **integrity floor**:
`ship ⇒ build ∧ audit ∧ (tdd unless trivial)`. No routing decision can reach ship
without a real build and a real audit.

---

## The Trust Kernel (Why You Can Merge Without Re-Reading Every Diff)

The threat model is not malicious humans — it's the **LLM itself** taking the
path of least resistance to "looks done." Three composed tiers defend against it
(full treatment in [../architecture/](../architecture/)):

| Tier | Mechanism | Posture |
|---|---|---|
| **1 — Structural integrity** | `phase-gate`, `role-gate`, `ship-gate`, `audit-binding`, SHA-chained `ledger` | **Always on. No bypass.** Catches phase-skipping, out-of-scope writes, unrouted commits, ledger tampering. |
| **2 — OS isolation** | `sandbox-exec` (macOS) / `bwrap` (Linux), per-cycle git `worktree` | Always present; path + mode auto-detected. Auditor/Evaluator run `read_only_repo`. |
| **3 — Workflow defaults** | Intent capture, adversarial audit, mutation testing, fan-out, Triage | Operator-tunable. Catches ambiguity and laziness — quality failures, not integrity breaches. |

The governing principle: **graceful adaptation, not graceful degradation.** When
the inner sandbox can't fire (e.g. nested-Claude), Tier 1 still enforces. There is
no configuration in which integrity protection is fully off.

---

## The Invariants (Memorize These)

These are the load-bearing rules. Violating any of them is a CRITICAL fault.

1. **Ship requires `red_count == 0`.** The `acs-verdict.json` predicate count, not
   the prose verdict, gates the commit.
2. **`audit-binding`: the audit is bound to a tree SHA.** Ship verifies the audited
   `git diff HEAD` tree still matches before committing. You cannot audit one tree
   and ship another.
3. **All commits route through `evolve ship`.** Bare `git commit`/`git push` is
   denied by `ship-gate`. The `commit-gate` requires a fresh review attestation
   bound to the staged tree SHA.
4. **Phases run in order, enforced by the OS.** The kernel reads `cycle-state.json`
   and on-disk artifacts; it does not trust the model's claim of progress.
5. **The `ledger` is append-only and hash-chained.** Each entry's `prev_hash`
   chains to the prior one; tampering invalidates every later entry.
6. **Every cycle number is real.** Claiming a cycle number without running
   Scout → Build → Audit → Ship → Learn is fabrication, the gravest violation.
7. **Lessons are evidence-bound.** A `lesson` is only merged into state after the
   merge script confirms its YAML file exists on disk. You cannot fabricate a lesson.
8. **In autonomous mode, "don't ask the user" never means "skip integrity checks."**
   Maximum velocity, zero shortcuts.

---

## Error Recovery (Work Survives Failure)

Long cycles fail routinely — quota walls, 529s, context limits. Four independent
layers preserve work at escalating cost (full treatment in [error-recovery](../../docs/concepts/error-recovery.md)):

1. **`failedApproaches[]`** — near-free single-loop record of what went wrong.
2. **Retrospective `lesson`s** — mid-cost double-loop root-cause + prevention rule.
3. **Checkpoint-resume** — heavy; preserves the full mid-cycle worktree + state so
   `evolve loop --resume` picks up exactly where a quota wall hit.
4. **Worktree preservation** — passive last-ditch; the worktree just isn't deleted.

The contract: a subscription user should never lose more than one phase of work to
a mid-cycle quota exhaustion.

---

## Pluggability (Three Independent Axes)

The framework separates *what work happens* from *who does it* from *what model
runs the who*:

- **Persona** — the role's prompt + output format, in `agents/<role>.md`.
- **Skill** — the imperative workflow inside a persona, in `skills/<name>/SKILL.md`.
- **LLM** — the CLI + model driving the persona, routed per-phase via
  `.evolve/llm_config.json` through a CLI `adapter` (Claude / Gemini / Codex).

You can route Scout to Gemini, Builder to Claude Sonnet, and Auditor to Claude Opus
in one config file — and the `capability catalog` plus tmux `recipe engine` let
non-native CLIs participate through driven terminal sessions. A custom persona still
inherits Tier 1 enforcement: you cannot author "a Scout that also commits to main."

---

## You Are Here — Map of the Knowledge Base

This page is the front door. From here:

| If you want to understand… | Go to |
|---|---|
| **Any specific term** | [glossary.md](glossary.md) |
| **How a phase, gate, or subsystem actually works** | `knowledge/.../architecture/` |
| **Why it learns across cycles, mechanically** | [self-evolution](../../docs/concepts/self-evolution.md) → `knowledge/.../evolution/` |
| **How the LLM is prevented from gaming verdicts** | [trust-architecture](../../docs/concepts/trust-architecture.md) |
| **How failures don't lose work** | [error-recovery](../../docs/concepts/error-recovery.md) |
| **How to route different models per phase** | [pluggability](../../docs/concepts/pluggability.md) |
| **Worked failures and what they taught the system** | `knowledge/.../incidents/` (e.g. cycle-61, cycle-11) |
| **Env vars, ship classes, operator commands** | `knowledge/.../reference/` + [CLAUDE.md](../../CLAUDE.md) |
| **To actually run a cycle right now** | [your-first-cycle](../../docs/getting-started/your-first-cycle.md) |

The one-line summary to carry with you: **Evolve Loop constrains LLMs the way the
scientific method constrains experimenters — every cycle is an experiment, every
experiment has a ledger entry, every failure produces a lesson, and only
exit-code-proven work ships.**
