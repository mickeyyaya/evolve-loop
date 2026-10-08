# Concept Overview — What Evolve Loop Is

> Read this first if you have never seen evolve-loop run a cycle. It explains the mental model: cycles, agents, artifacts, and what "self-evolving" actually means in this codebase.
> Audience: new operators and contributors. The reference docs in [docs/architecture/](../architecture/) assume that you know everything in this file.

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

Evolve Loop runs your codebase through an autonomous improvement cycle. It finds work, implements it, audits its own output, ships only what passes, and learns from failures. Every cycle is an OS-enforced sequence of 8 phases. The model cannot skip phases, edit out of scope, or forge verdicts. The trust kernel (shell hooks + sandboxes + a tamper-evident ledger) blocks that structurally.

If you have used `/goal` in Claude Code 2.1.139+, this is the next layer down. It is a full pipeline with adversarial review and error recovery, and it learns durably across runs.

---

## What a Cycle Is

A cycle is one pass through 8 phases (plus a meta-cycle every 5 cycles). Every phase produces a markdown artifact in `.evolve/runs/cycle-N/`, and the next phase must read that artifact.

| # | Phase | What happens | Output artifact |
|---|---|---|---|
| 1 | **Intent** | Operator's vague goal → 8-field structured intent + AwN classifier + ≥1 challenged premise | `intent.md` |
| 2 | **Calibrate** | Cycle-state initialized; HEAD recorded; baselines captured | `cycle-state.json` |
| 3 | **Scout** | Find work (codebase + carryover + instincts); cite research; propose tasks | `scout-report.md` |
| 4 | **Triage** | Bound the scope of this cycle: top_n / deferred / dropped | `triage-decision.md` |
| 5 | **Plan-Review** (opt-in) | 4-lens CEO/Eng/Design/Security review of the plan | `plan-review.md` |
| 6 | **Build** | Implement in an isolated git worktree; write EGPS predicates | `build-report.md`, `acs/cycle-N/*.sh` |
| 7 | **Audit** | Adversarial cross-check; run ACS predicate suite; emit verdict JSON | `audit-report.md`, `acs-verdict.json` |
| 8 | **Ship** | If `red_count==0`, commit + push through `legacy/scripts/lifecycle/ship.sh` | git commit on `main` |
| 9 | **Memo / Retro** | PASS → memo (carryover capture); FAIL/WARN → retrospective (lesson extraction) | `carryover-todos.json` OR `retrospective-report.md` + `lessons/<id>.yaml` |

Since v22.27.0, a code cycle also runs **Code-Review** between Build and Audit. It runs in shadow, writes `code-review-report.md` and is not a gate.

Then `gate_cycle_complete` archives the workspace to `.evolve/history/cycle-N/`.

The 8 phases are mandatory, in order. The kernel-managed helper `legacy/scripts/lifecycle/cycle-state.sh advance <phase> <agent>` records every phase change in `cycle-state.json`. `phase-gate-precondition.sh` (PreToolUse hook) structurally blocks a skipped phase, and phases out of order.

---

## Who Does What — The Agents

An "agent" in evolve-loop is **one persona, one perspective, one output format** (see [tri-layer.md](../architecture/tri-layer.md)). Personas are markdown files in `agents/`. They do not invoke each other. The slash command (or the orchestrator of `/evo:loop`) puts them in sequence.

| Agent | Persona file | Role | Read what | Write what |
|---|---|---|---|---|
| **Scout** | `agents/evolve-scout.md` | Discovery + plans | codebase, `state.json:carryoverTodos[]`, `instinctSummary[]` | `scout-report.md` |
| **Triage** | `agents/evolve-triage.md` | Cycle-scope bouncer | `scout-report.md` | `triage-decision.md` |
| **Plan-Reviewer** (opt-in fan-out) | `agents/plan-reviewer.md` | 4-lens review | `scout-report.md`, `intent.md` | `plan-review.md` (aggregate of 4 worker artifacts) |
| **Builder** | `agents/evolve-builder.md` | Implementation | `scout-report.md`, `triage-decision.md` | code edits in worktree, `build-report.md`, `acs/cycle-N/*.sh` predicates |
| **Tester** (opt-in, v10.3.0+) | `agents/evolve-tester.md` | Predicate authorship | `build-report.md` | `acs/cycle-N/*.sh` (split from the deliverables of Builder) |
| **Code-Reviewer** (code cycles, shadow, v22.27.0+) | `agents/evolve-code-reviewer.md` | Independent review on the shared quality index | the build and its diff | `code-review-report.md` |
| **Auditor** | `agents/evolve-auditor.md` | Adversarial cross-check | everything above + `git diff HEAD` | `audit-report.md`, `acs-verdict.json` |
| **Memo** (PASS cycles only) | `agents/evolve-memo.md` | Carryover capture | scout/triage outputs | `carryover-todos.json`, `memo.md` |
| **Retrospective** (FAIL/WARN only) | `agents/evolve-retrospective.md` | Lesson extraction | artifacts of the failed cycle | `retrospective-report.md`, `.evolve/instincts/lessons/<id>.yaml` |
| **Orchestrator** | `agents/evolve-orchestrator.md` | Phase sequencer (not a write-heavy role) | all phase outputs | `orchestrator-report.md` |

The routing prefers a **different model family for the Auditor and the Builder**. The Auditor runs Claude Opus on Claude Code. In this repository, the `cli_routing` table runs the Builder on agy first. `evolve cli-routing explain builder` prints the chain and the model at the tier of the builder. This is intentional: same-model judges are sycophantic. See [`docs/architecture/multi-llm-review.md`](../architecture/multi-llm-review.md).

---

## What Artifacts You'll See

After a cycle, open `.evolve/runs/cycle-N/`. You see these files:

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
| `acs/cycle-N/*.sh` | builder/tester | Executable predicates: the actual evidence of the verdict |

After a successful ship, the predicates promote to `acs/regression-suite/cycle-N/` and become permanent regression guards. Future cycles must keep them GREEN.

---

## What Self-Evolving Means Here

Here, self-evolving is a specific technical claim, not a sales claim. It means: **failures in cycle N produce machine-readable lessons that cycle N+1 reads and acts on.**

In detail:

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

This is the Reflexion loop (Shinn et al. 2023, "Reflexion: Language Agents with Verbal Reinforcement Learning"). [self-evolution.md](self-evolution.md) gives the detailed mechanism.

Two important properties:

1. **Lessons are durable across cycles.** They are in `state.json` (tracked) plus YAML files (tracked).
   A cycle 60 failure can teach a cycle 200 Scout.
2. **Lessons are evidence-bound.** Each lesson cites the cycle that produced it. You cannot fabricate
   a lesson: the merge script verifies that the YAML file exists on disk.

The cycle-61 → cycle-63 incident in this repo is a worked example. Cycle 61 failed in 7 distinct ways, and the retrospective extracted lessons. Then cycles 1-9 of v10.7.0 fixed all 7 structurally. See [`docs/incidents/cycle-61.md`](../incidents/cycle-61.md).

---

## How This Differs From a Makefile or a CI Pipeline

Both Makefiles and CI pipelines run steps in a sequence. But:

| Property | Makefile | CI pipeline | evolve-loop |
|---|---|---|---|
| Steps written by | Human | Human | **LLM** (each cycle picks its own tasks) |
| Step boundary enforcement | Build dependencies | Workflow YAML | **OS shell hooks (PreToolUse)** |
| Verdict source | exit code | Test runner | **EGPS predicate exit codes** (deterministic; no LLM judge) |
| Tamper resistance | None | Git history | **SHA-chained ledger** with `prev_hash` |
| Lessons from failures | None | None | **Reflexion-style YAML lessons → next-cycle instincts** |
| Recovery from mid-run crash | Re-run target | Re-run job | **Checkpoint-resume**: per-cycle worktree preserved, state survives |

evolve-loop is closer to a **research lab notebook**. Every cycle is an experiment, every experiment has a ledger entry, every failure produces a lesson, and the lab gets smarter over time. That LLMs do the work is incidental to the framework. The job of the framework is to constrain LLMs, the same way as the scientific method constrains experimenters.

---

## What to Read Next

After this overview, read in the order of your curiosity:

| If you want to understand... | Read |
|---|---|
| Why it learns from failures | [self-evolution.md](self-evolution.md) |
| How it stops LLMs that try to game the system | [trust-architecture.md](trust-architecture.md) |
| How errors recover | [error-recovery.md](error-recovery.md) |
| How every phase is pluggable + how to route different LLMs per-phase | [pluggability.md](pluggability.md) |
| How it compares to other tools, for example /goal, self-improving-agent and superpowers | [../comparisons/long-running-claude-skills.md](../comparisons/long-running-claude-skills.md) |
| The per-phase mechanics | [../architecture/phase-architecture.md](../architecture/phase-architecture.md) |
| To actually run a cycle | [../getting-started/your-first-cycle.md](../getting-started/your-first-cycle.md) |
