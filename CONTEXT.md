# CONTEXT.md — Evolve Loop Canonical Glossary

## Language

**batch** — One dispatcher invocation spanning N cycles until budget exhausts or batch_cap fires. _Avoid_: run, session, job.

**carryover** — A queued todo in `state.json:carryoverTodos[]`; not started, not failed. _Avoid_: backlog item, deferred task.

**cycle** — One Scout→Build→Audit→Ship→Learn iteration; the unit of work. _Avoid_: run, iteration, invocation.

**gate** — A phase-transition enforcement point blocking out-of-order subagent dispatch. _Avoid_: guard, check.

**instinct** — A learned rule written by retrospective and persisted so subsequent cycles read it. _Avoid_: memory, lesson file, knowledge.

**kernel hook** — A guard script at deterministic pipeline boundaries enforcing integrity. _Avoid_: hook, guard, check.

**ledger** — Append-only tamper-evident log of subagent invocations with SHA256 hash-chain. _Avoid_: log, history, audit trail.

**memo** — Post-PASS role that writes a cycle summary and emits carryover todos for the next cycle. _Avoid_: summary, handoff, recap.

**persona** — The full markdown specification defining a role's behavior, tools, and output format. _Avoid_: agent file, prompt, character.

**phase** — An ordered, gated step within a cycle. _Avoid_: stage, step, turn.

**pipeline** — The ordered sequence of phases in a cycle, enforced by kernel hooks. _Avoid_: workflow, process, loop body.

**retrospective** — Post-FAIL/WARN role that writes a structured lesson merged into instinct state. _Avoid_: retro, postmortem, review.

**role** — The functional job a subagent performs (scout / builder / auditor / orchestrator / retrospective / triage / memo / intent). _Avoid_: agent type, worker, persona.

**sandbox** — OS-level process isolation wrapping each subagent invocation. _Avoid_: container, jail, isolation layer.

**triage** — Post-scout phase selecting top-N tasks from the scout report and carryover queue. _Avoid_: prioritizer, filter, planning step.

**verdict** — The auditor's output: PASS, WARN, or FAIL; drives the ship/retrospective branch. _Avoid_: score, result, outcome.

**worktree** — A git worktree provisioned per cycle where the builder writes exclusively. _Avoid_: branch checkout, scratch area.

**Project** — A directory containing a `.evolve/` subdirectory; the unit of evolve-loop isolation. _Avoid_: workspace, repo.

**Project hash** — The 8-char SHA256 prefix of `EVOLVE_PROJECT_ROOT`; namespaces shared resources across concurrent Projects. Never hardcoded. _Avoid_: project ID, identifier.

**Worktree base** — The per-project temporary directory holding per-cycle git worktrees, derived from the Project hash. _Avoid_: temp dir, scratch dir.

**Concurrent isolation** — The property that multiple Projects run simultaneously without interfering with each other's `.evolve/` tree, ledger, memory bucket, or worktrees. _Avoid_: parallelism, multi-tenancy.

## Relationships

A **batch** contains N **cycles**. Each cycle traverses a **pipeline** of **phases**, each executed by a **role** defined by its **persona**. **Kernel hooks** enforce transitions at each **gate**.

Scout produces a report; builder edits a **worktree**; auditor issues a **verdict**. On PASS, **memo** queues **carryovers**. On FAIL/WARN, **retrospective** writes an **instinct** read by the next cycle.

The **ledger** records invocations (immutable). `state.json` holds mutable project state. They are distinct.

Multiple **Projects** run concurrently; each gets its own **Project hash** → **Worktree base** → `.evolve/` tree → memory bucket.

## Flagged ambiguities

**phase vs role** — A phase is a gated pipeline stage; a role is the job a subagent performs. They share names: the *scout* role runs during the *research* phase.

**ledger vs state.json** — Ledger records invocations (immutable). `state.json` holds project state (mutable). Different integrity functions; do not conflate.

**carryover vs failedApproach** — Carryover: queued intention. FailedApproach: recorded failure. They age out differently and are never merged.

**gate vs kernel hook** — All gates are kernel hooks; not all kernel hooks are gates. Role-gate and ship-gate are hooks, not phase-transition gates.

**plugin root vs project root** — Plugin root is read-only (evolve-loop install). Project root is writable (user project). Scripts must never write to plugin root.
