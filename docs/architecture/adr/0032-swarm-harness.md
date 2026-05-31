# ADR-0032: Multi-tmux-LLM-CLI Subagent Swarm Harness

**Status:** Accepted + IMPLEMENTED (v1→v4 shipped on `feat/swarm-harness`, HEAD `32bae0f`). The full harness — planner+validator (v1), isolation/registry/reaper (v2), parallel dispatcher (v3), merge-train/synthesis fan-in + `swarmRunner` Decorator WIRED into the orchestrator runners map (v4) — is live behind `EVOLVE_SWARM_STAGE` (default `shadow` = byte-identical N=1 delegate).
**Date:** 2026-05-31
**Related:** ADR-0023 (live injection), ADR-0024 (phase advisor), ADR-0027 (worktree provisioning / commit-as-evidence), ADR-0031 (tmux recipe engine + capability catalog); `docs/architecture/sequential-write-discipline.md`.

## Context

evolve-loop runs **one** LLM agent per phase. The orchestrator's single dispatch point is
`go/internal/core/orchestrator.go` (`runner.Run(ctx, phaseReq)`), and `sequential-write-discipline.md`
forbids parallel writers because the ledger hash-chain (`prev_hash`+`entry_seq`) and the `.git` index
are single-writer.

We want a phase to optionally dispatch **N heterogeneous workers** (each its own CLI/model) that
collaborate on a decomposable task, for throughput on large phases — without weakening the invariants
that make the system safe.

## Decision

Introduce a reusable **swarm primitive** (`go/internal/swarm/`) any phase can opt into, attached via
a **Decorator** at the existing dispatch seam (the `runner.Run` call site stays byte-identical). A new
**Planner** phase (`swarm-plan`) partitions the task; a deterministic **Validator** enforces the
safety gate; workers run in isolated branch+worktree+sandbox; results fan in — by a **serialized
merge-train** (writers) or **synthesis** (readers); the orchestrator reduces N results to ONE
`PhaseResponse` and ONE ledger entry.

### The central rule: the WRITER/READER asymmetry

Writers and readers partition along **different axes** with **opposite safety gates**:

| | **WRITER swarm** (e.g. build) | **READER swarm** (e.g. scout/audit/research) |
|---|---|---|
| Partition axis | exclusive **file ownership** (correctness) | investigative **aspect** (efficiency) |
| Disjointness | **STRICT — required** | best-effort; **overlap allowed** |
| Not cleanly partitionable | **strongly recommend NOT to swarm → fall back to N=1** | still swarm (overlap = wasted tokens, never corruption) |
| Isolation | own branch + worktree + sandbox | isolated context + own workspace (no branch/worktree) |
| Fan-in | serialized git **merge-train** into an integration branch | **synthesis/concat** of summaries |

This is the load-bearing decision. It is grounded in how the surveyed systems behave (see the research
doc): Anthropic's multi-agent research system is read-heavy *because* read overlap is harmless and
write overlap is a correctness failure; Gemini CLI's own docs warn against parallel writers; no
surveyed OSS tool (claude-squad, uzi, container-use, Codex Cloud) reliably auto-merges *conflicting*
writers — they isolate by branch and defer to humans. So a writer swarm is only safe on **provably
disjoint** work; everything else falls back to a single writer (N=1), which is byte-identical to
today's behavior.

### Why collaborative-integration, not competition

Workers do NOT each attempt the whole task so a judge can pick a winner. Instead the Planner splits the
task into independent sub-tasks; each worker owns its slice; a serialized merge-train assembles them
into one integration branch (each worker resolves its own merge conflicts against the current
integration tip). This matches the user's intent (real throughput on a decomposable task) and keeps the
single-writer rule intact: parallelism lives only in *disjoint worktrees*; the *only* serialized
section is the merge — exactly where the single-writer rule already applies.

### Guaranteed teardown (both layers)

1. **Live, structured concurrency:** a `SessionRegistry` tracks every dispatched session; the dispatch
   scope blocks until all workers are reaped (process-group kill via `Setpgid`+`kill(-pgid)` → tmux
   `kill-session` → `#{pane_dead}` confirm). **Orphan-on-cancel hardened:** the dispatcher pins a
   deterministic tmux session name (`swarm-c<cycle>-<workerID>` via the shared `bridge.NamedSessionName`
   formula) and REGISTERS it *before* `Launch`, so a worker cancelled mid-spawn is still reaped by name
   (no reliance on the launch returning a session identity). Headless workers create no session and die
   by ctx-cancel.
2. **Crash-safe:** a persistent on-disk manifest + an `evolve swarm reap` sweep kills orphans even
   after a hard SIGKILL of the parent (the one case in-process defer cannot cover).

### Design patterns

- **Proposer–Validator** — the LLM proposes the partition; deterministic native code (`swarm.Validate`)
  enforces disjointness. Never trust an LLM for a correctness-critical invariant.
- **Decorator** — `swarmRunner` wraps any `core.PhaseRunner`; the orchestrator is unchanged.
- **Strategy** — fan-in reducer (merge-train vs synthesis) selected by `Mode`.
- **Template Method (reuse)** — the planner phase is a `runner.BaseRunner` clone of `buildplanner`.
- **Dependency Injection** — the dispatcher takes injected ports (Bridge, WorktreeProvisioner,
  GitMerger, Tmux, Clock), matching `engine.go`/`orchestrator.go`.

## Rollout (shadow → advisory → enforce)

| Stage | `EVOLVE_SWARM_STAGE` | Behavior |
|---|---|---|
| **v1 (this)** | `shadow`/off (default) | Planner + Validator exist + are unit-tested; pure partition logic (`swarm.Validate`, `TopoOrder`, `ParsePlan`). **No dispatch** — build/scout run N=1, pipeline byte-identical to today. |
| v2 | — | Worktree/branch isolation + `SessionRegistry` + `Reaper` + `evolve swarm reap` (fakes only). |
| v3 | `advisory` | Real parallel `Dispatcher` (workers launch); integration computed but not authoritative. |
| v4 | `enforce` | `MergeTrain` authoritative (writers) / synthesis (readers); proven on build + scout. |

Every stage is independently shippable through the commit-gate + 2-cycle E2E, and reverts to N=1 on any
planning/validation failure.

## Consequences

**Positive:** parallel throughput on decomposable phases; cross-family worker diversity; the
single-writer invariants are preserved (not bypassed); strictly additive (worst case = today).

**Negative / costs:** multi-agent ≈ 15× tokens (mitigated: planner recommends-against-swarm unless it
finds real breadth; per-worker + total budget caps); more moving parts (mitigated: small focused units,
exhaustive unit tests on the pure core, the crash-safe reaper, and the debugging runbook in
`docs/architecture/swarm-harness.md`).

## Alternatives rejected

- **Judge-select competition** (N workers each do the whole task, pick the best) — wastes tokens on
  redundant full attempts and discards work; the user explicitly chose collaborative integration.
- **Best-effort repair of overlapping writer partitions** — too clever; an LLM-proposed "repair" of a
  correctness-critical disjointness violation is itself untrustworthy. v1 rejects-to-N=1 instead.
- **Concurrent merges with retry-on-lock** — fragile `.git` lock thrash; the serialized merge-train is
  deterministic and debuggable.
