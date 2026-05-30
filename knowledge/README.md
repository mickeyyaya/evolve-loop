# evolve-loop knowledge base

The curated, future-facing knowledge base for `evolve-loop` — distilled from ~148 self-improvement
cycles into durable structure. If you are new here, **start with
[00-overview/system-in-one-page.md](00-overview/system-in-one-page.md)**.

This base is designed so a developer with zero prior context can reconstruct *what the system is*,
*why it is shaped this way*, *how each subsystem works*, and *what was already tried and rejected* —
without reading the raw cycle history (which has been retired; see
[_migration/history-recovery.md](_migration/history-recovery.md)).

## Map

| Area | Read it for |
|---|---|
| **[00-overview/](00-overview/)** | The mental model in one page + a [glossary](00-overview/glossary.md) of every load-bearing term. |
| **[architecture/](architecture/)** | How the system is built and **why**, grounded in the Go code (`go/internal/<pkg>`). Five docs: [phase-pipeline](architecture/phase-pipeline.md), [trust-kernel-and-egps](architecture/trust-kernel-and-egps.md), [routing-and-advisor](architecture/routing-and-advisor.md), [bridge-and-adapters](architecture/bridge-and-adapters.md), [state-and-ledger](architecture/state-and-ledger.md). |
| **[evolution/](evolution/)** | What ~148 cycles **taught**: the [bash→Go port](evolution/bash-to-go-port.md), the [ADR decision digest](evolution/decision-digest.md), [rejected approaches](evolution/rejected-approaches.md) (anti-knowledge — don't repeat these), and the [compound-improvement arc](evolution/compound-improvement-arc.md). |
| **[incidents/](incidents/)** | A synthesized [pattern library](incidents/pattern-library.md) — recurring failure modes, root causes, and the guards that now prevent them, with a triage table. |
| **[reference/](reference/)** | Lookup tables: the [env-var reference](reference/env-vars.md) and the [CLI capability matrix](reference/cli-capability-matrix.md). |
| **[_migration/](_migration/)** | The consolidation record: the [unmerged-work ledger](_migration/unmerged-work-ledger.md) (what was merged/dropped) and [history-recovery](_migration/history-recovery.md) (the safety net). |

## Reading paths

- **New contributor:** overview → architecture/phase-pipeline → architecture/trust-kernel-and-egps → glossary as needed.
- **Debugging a failure:** incidents/pattern-library (triage table) → the relevant architecture doc → evolution/rejected-approaches.
- **Changing behavior:** reference/env-vars → the relevant architecture doc → evolution/decision-digest (check the rationale before reversing a decision).

## Governance

This is **synthesized, not archival** — each doc rewrites raw history into durable lessons. When a new
durable lesson is learned, refine the relevant doc in place rather than appending dated dumps. The raw
cycle artifacts (`.evolve/runs/`, `ledger.jsonl`, per-cycle retrospectives) were gitignored runtime
state and have been retired; their *lessons* live here.
