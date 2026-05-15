# Concept Overview — What Evolve Loop Is

> Read this first if you've never seen evolve-loop run a cycle. It explains the mental model: cycles, agents, artifacts, and what "self-evolving" actually means in this codebase.
> Audience: new operators and contributors. Reference docs in [docs/architecture/](../architecture/) assume you know everything in this file.

## Table of Contents

1. [The 30-Second Pitch](#the-30-second-pitch)
2. [What a Cycle Is](#what-a-cycle-is)
3. [Who Does What — The Agents](#who-does-what--the-agents)
4. [What Artifacts You'll See](#what-artifacts-youll-see)
5. [What Self-Evolving Means Here](#what-self-evolving-means-here)
6. [How This Differs From a Makefile or a CI Pipeline](#how-this-differs-from-a-makefile-or-a-ci-pipeline)
7. [What to Read Next](#what-to-read-next)

---

## The 30-Second Pitch

Evolve Loop runs your codebase through an autonomous improvement cycle: it finds work, implements it, audits its own output, ships only what passes, and learns from failures. Every cycle is an OS-enforced sequence of 8 phases. The model can't skip phases, edit out of scope, or forge verdicts — the trust kernel (shell hooks + sandboxes + a tamper-evident ledger) blocks that structurally.

If you've used `/goal` in Claude Code 2.1.139+, this is the next layer down: a full pipeline with adversarial review, error recovery, and durable learning across runs.

---

## What a Cycle Is

A cycle is one pass through 8 phases (plus a meta-cycle every 5 cycles). Every phase produces a markdown artifact in `.evolve/runs/cycle-N/` that the next phase must read.

| # | Phase | What happens | Output artifact |
|---|---|---|---|
| 1 | **Intent** | Operator's vague goal → 8-field structured intent + AwN classifier + ≥1 challenged premise | `intent.md` |
| 2 | **Calibrate** | Cycle-state initialized; HEAD recorded; baselines captured | `cycle-state.json` |
| 3 | **Scout** | Find work (codebase + carryover + instincts); cite research; propose tasks | `scout-report.md` |
| 4 | **Triage** | Bound this cycle's scope: top_n / deferred / dropped | `triage-decision.md` |
| 5 | **Plan-Review** (opt-in) | 4-lens CEO/Eng/Design/Security review of the plan | `plan-review.md` |
| 6 | **Build** | Implement in an isolated git worktree; write EGPS predicates | `build-report.md`, `acs/cycle-N/*.sh` |
| 7 | **Audit** | Adversarial cross-check; run ACS predicate suite; emit verdict JSON | `audit-report.md`, `acs-verdict.json` |
| 8 | **Ship** | If `red_count==0`, commit + push via `scripts/lifecycle/ship.sh` | git commit on `main` |
| 9 | **Memo / Retro** | PASS → memo (carryover capture); FAIL/WARN → retrospective (lesson extraction) | `carryover-todos.json` OR `retrospective-report.md` + `lessons/<id>.yaml` |

Then `gate_cycle_complete` archives the workspace to `.evolve/history/cycle-N/`.

The 8 phases are mandatory in order. Every phase change is recorded in `cycle-state.json` via the kernel-managed helper `scripts/lifecycle/cycle-state.sh advance <phase> <agent>`. Skipping a phase or running them out of order is structurally blocked by `phase-gate-precondition.sh` (PreToolUse hook).

---

## Who Does What — The Agents

An "agent" in evolve-loop is **one persona, one perspective, one output format** — see [tri-layer.md](../architecture/tri-layer.md). Personas are markdown files in `agents/`. They don't invoke each other; the slash command (or `/evolve-loop`'s orchestrator) sequences them.

| Agent | Persona file | Role | Read what | Write what |
|---|---|---|---|---|
| **Scout** | `agents/evolve-scout.md` | Discovery + planning | codebase, `state.json:carryoverTodos[]`, `instinctSummary[]` | `scout-report.md` |
| **Triage** | `agents/evolve-triage.md` | Cycle-scope bouncer | `scout-report.md` | `triage-decision.md` |
| **Plan-Reviewer** (opt-in fan-out) | `agents/plan-reviewer.md` | 4-lens review | `scout-report.md`, `intent.md` | `plan-review.md` (aggregate of 4 worker artifacts) |
| **Builder** | `agents/evolve-builder.md` | Implementation | `scout-report.md`, `triage-decision.md` | code edits in worktree, `build-report.md`, `acs/cycle-N/*.sh` predicates |
| **Tester** (opt-in, v10.3.0+) | `agents/evolve-tester.md` | Predicate authorship | `build-report.md` | `acs/cycle-N/*.sh` (split from Builder's deliverables) |
| **Auditor** | `agents/evolve-auditor.md` | Adversarial cross-check | everything above + `git diff HEAD` | `audit-report.md`, `acs-verdict.json` |
| **Memo** (PASS cycles only) | `agents/evolve-memo.md` | Carryover capture | scout/triage outputs | `carryover-todos.json`, `memo.md` |
| **Retrospective** (FAIL/WARN only) | `agents/evolve-retrospective.md` | Lesson extraction | failed cycle's artifacts | `retrospective-report.md`, `.evolve/instincts/lessons/<id>.yaml` |
| **Orchestrator** | `agents/evolve-orchestrator.md` | Phase sequencer (not a write-heavy role) | all phase outputs | `orchestrator-report.md` |

The Auditor runs on a **different model family from the Builder** by default (Builder=Sonnet, Auditor=Opus or Haiku per profile). This is intentional: same-model judges are sycophantic. See [`docs/architecture/multi-llm-review.md`](../architecture/multi-llm-review.md).

---

## What Artifacts You'll See

Open `.evolve/runs/cycle-N/` after a cycle and you'll see:

| File | Producer | Purpose |
|---|---|---|
| `intent.md` | intent agent | Structured goal (8 fields) |
| `scout-report.md` | scout | Selected tasks + research + decision trace |
| `triage-decision.md` | triage | top_n[] + deferred[] + dropped[] |
| `build-report.md` | builder | Files changed + AC claims |
| `audit-report.md` | auditor | Verdict + per-AC verification + defects |
| `acs-verdict.json` | run-acs-suite.sh | Binary PASS/FAIL from predicate exit codes |
| `orchestrator-report.md` | orchestrator | Phase outcome table + CLI Resolution (auto-rendered) |
| `*-usage.json` | adapter | Cost + tokens per phase |
| `*-stdout.log`, `*-stderr.log` | adapter | Raw LLM stream output |
| `acs/cycle-N/*.sh` | builder/tester | Executable predicates — the verdict's actual evidence |

After a successful ship: predicates promote to `acs/regression-suite/cycle-N/` and become permanent regression guards. Future cycles must keep them GREEN.

---

## What Self-Evolving Means Here

Self-evolving here is a specific technical claim, not marketing. It means: **failures in cycle N produce machine-readable lessons that cycle N+1 reads and acts on.**

Concretely:

```
Cycle N audit FAILs
  ↓
record-failure-to-state.sh appends to state.json:failedApproaches[]   (Argyris single-loop)
  ↓
retrospective agent fires (auto-on per v8.45.0+)
  ↓
Lessons emitted as YAML files: .evolve/instincts/lessons/cycle-N-<slug>.yaml  (Argyris double-loop)
  ↓
merge-lesson-into-state.sh appends to state.json:instinctSummary[]
  ↓
Cycle N+1 Scout reads state.json
  ↓
Scout's prompt context includes the new instincts
  ↓
Builder avoids the failure pattern OR retrospective marks it `systemic: true` and triggers preventive refactor
```

This is the Reflexion loop (Shinn et al. 2023, "Reflexion: Language Agents with Verbal Reinforcement Learning"). The detailed mechanism is in [self-evolution.md](self-evolution.md).

Two important properties:

1. **Lessons are durable across cycles.** They live in `state.json` (tracked) plus YAML files (tracked). A cycle 60 failure can teach a cycle 200 Scout.
2. **Lessons are evidence-bound.** Each lesson cites the cycle that produced it. You can't fabricate a lesson — the merge script verifies the YAML file exists on disk.

The cycle-61 → cycle-63 incident in this repo is a worked example: cycle 61 failed in 7 distinct ways, retrospective extracted lessons, and cycles 1-9 of v10.7.0 fixed all 7 structurally. See [`docs/incidents/cycle-61.md`](../incidents/cycle-61.md).

---

## How This Differs From a Makefile or a CI Pipeline

Both Makefiles and CI pipelines run sequenced steps. But:

| Property | Makefile | CI pipeline | evolve-loop |
|---|---|---|---|
| Steps written by | Human | Human | **LLM** (each cycle picks its own tasks) |
| Step boundary enforcement | Build dependencies | Workflow YAML | **OS shell hooks (PreToolUse)** |
| Verdict source | exit code | Test runner | **EGPS predicate exit codes** (deterministic; no LLM judge) |
| Tamper resistance | None | Git history | **SHA-chained ledger** with `prev_hash` |
| Learning from failures | None | None | **Reflexion-style YAML lessons → next-cycle instincts** |
| Recovery from mid-run crash | Re-run target | Re-run job | **Checkpoint-resume**: per-cycle worktree preserved, state survives |

evolve-loop is closer to a **research lab notebook**: every cycle is an experiment, every experiment has a ledger entry, every failure produces a lesson, and the lab gets smarter over time. The fact that LLMs do the work is incidental to the framework — the framework's job is to constrain LLMs the way scientific method constrains experimenters.

---

## What to Read Next

After this overview, read in order of curiosity:

| If you want to understand... | Read |
|---|---|
| Why it learns from failures | [self-evolution.md](self-evolution.md) |
| How it prevents LLMs from gaming the system | [trust-architecture.md](trust-architecture.md) |
| How errors recover | [error-recovery.md](error-recovery.md) |
| How every phase is pluggable + how to route different LLMs per-phase | [pluggability.md](pluggability.md) |
| How it compares to /goal, self-improving-agent, superpowers, etc. | [../comparisons/long-running-claude-skills.md](../comparisons/long-running-claude-skills.md) |
| The per-phase mechanics | [../architecture/phase-architecture.md](../architecture/phase-architecture.md) |
| To actually run a cycle | [../getting-started/your-first-cycle.md](../getting-started/your-first-cycle.md) |
