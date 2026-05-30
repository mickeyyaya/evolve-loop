# State & Ledger — the durable substrate

> Everything the pipeline knows that must survive a crash lives in four places:
> `state.json` (the cross-cycle envelope), `cycle-state.json` (the per-cycle
> recovery anchor), `ledger.jsonl` (the hash-chained audit trail), and the
> per-cycle git worktree (the isolated source tree). This document describes their
> schemas, the resume/reset machinery, the unfinished-cycle guard, and *why* the
> state is laid out this way. Current Go design (v13.0.0).

Related: [phase-pipeline.md](phase-pipeline.md) ·
[trust-kernel-and-egps.md](trust-kernel-and-egps.md) ·
[routing-and-advisor.md](routing-and-advisor.md) ·
[glossary](../00-overview/glossary.md)

---

## 1. The `.evolve/` surface

All durable state lives under `.evolve/`, read and written exclusively through the
`core.Storage` and `core.Ledger` ports (`go/internal/core/ports.go`), implemented
by the storage and ledger adapters. The orchestrator never touches the filesystem
directly — which is what makes `RunCycle` unit-testable with in-memory fakes.

| File | Lifetime | Owner | Role |
|---|---|---|---|
| `state.json` | persistent | `Storage` | cross-cycle envelope (cycle counter, batch cost, failure history) |
| `cycle-state.json` | per-cycle | `Storage` | the in-flight cycle's recovery anchor |
| `ledger.jsonl` + `ledger.tip` | append-only | `Ledger` | hash-chained audit trail |
| `.lock` | per-run | `Storage` | the single cycle lock |
| `worktrees/cycle-N/` | per-cycle | `WorktreeProvisioner` | isolated source tree for tdd/build |
| `runs/cycle-N/` | per-cycle | phases | the workspace (artifacts, predicates, decisions) |

---

## 2. `state.json` — the cross-cycle envelope

`core.State` (`ports.go`) is a **subset view** of `state.json` — it models only the
orchestrator-load-bearing fields:

```go
type State struct {
    LastCycleNumber int             // the integer cycle counter
    CurrentBatch    BatchAccrual    // cumulative USD cost for this dispatcher run
    FailedAt        []FailedRecord  // failedApproaches[] — the failure-adapter's input
    CarryoverTodos  []CarryoverTodo // operator-queued work surfaced cycle-to-cycle
    SetupCompletedAt string         // first-run onboarding marker
}
```

> **Critical subtlety:** because `State` is a *subset*, a naïve `WriteState` would
> *drop* any unmodeled key (e.g. `expected_ship_sha`, the self-SHA TOFU pin from
> [trust-kernel-and-egps.md](trust-kernel-and-egps.md) §5). So fields outside the
> orchestrator's concern are mutated through a **full-fidelity map** (`reset.go`'s
> `readJSONMapFile` / `writeJSONMapFileAtomic`; ship's `statefile.go`), never
> through the typed `WriteState`. This is the same trap documented across
> `reset.go` and `ship/statefile.go`.

`FailedRecord` (`ports.go`) is the on-disk shape of `failedApproaches[]` — each
non-PASS cycle outcome, carrying `verdict`, `classification`, `expiresAt`, and the
audit-report SHA/HEAD/tree binding. This array is the **sole input** to the
deterministic failure-adapter that decides retro's RETRY/BLOCK/PROCEED branch (see
[phase-pipeline.md](phase-pipeline.md) §5). `expiresAt` implements a 30-day
retention window so old audit failures don't block forever.

`LastCycleNumber` is the spine of cycle identity: `RunCycle` computes
`cycle = LastCycleNumber + 1`, runs the cycle, and advances `LastCycleNumber = cycle`
only on completion (`ship.sh`/native ship also advances it after a successful ship).
A *finished* cycle therefore has `cycle-state.json:cycle_id == LastCycleNumber`; an
*unfinished* one is ahead — the basis of the guard in §6.

---

## 3. `cycle-state.json` — the per-cycle recovery anchor

`core.CycleState` (`ports.go`) is the transient per-cycle record, written
**incrementally** after every phase transition so a crash leaves an inspectable
trail:

```go
type CycleState struct {
    CycleID         int      // == LastCycleNumber+1 while in flight
    Phase           string   // the phase currently running
    CompletedPhases []string // append-only; drives resume + routing digest
    ActiveWorktree  string   // the per-cycle worktree path (gates source writes)
    WorkspacePath   string   // .evolve/runs/cycle-N
    IntentRequired  bool     // gates the start→intent vs start→scout edge
}
```

`ActiveWorktree` is load-bearing for the trust kernel: the **role-gate**
(`guards/role.go`) allows a source write only when the phase is a source-writing
phase AND the path is under `cs.ActiveWorktree`. `CompletedPhases` feeds both the
resume replay (§5) and the routing digest (`router.Digest`, see
[routing-and-advisor.md](routing-and-advisor.md)).

---

## 4. `ledger.jsonl` — the hash-chained audit trail

The ledger is an append-only, tamper-evident log. Each `core.LedgerEntry`
(`ports.go`) carries `prev_hash` + `entry_seq`; the `FileLedger` adapter
(`go/internal/adapters/ledger/ledger.go`) implements the chain:

- **Append**: read the current `ledger.tip` (`<seq>:<sha>`); set the new entry's
  `PrevHash` to the prior tip hash and `EntrySeq = prevSeq + 1` (genesis seeds
  `PrevHash = ZeroSeed`, `EntrySeq = 0`); write the line; compute
  `newHash = sha256(line)`; rewrite `ledger.tip` to `<entry_seq>:<newHash>`.
- **Verify**: walk every line and assert each `prev_hash` equals the SHA of the
  preceding line. A single edited or removed entry breaks the chain *at that point
  and every entry after it* (`core.ErrLedgerChainBroken`).

**Why a hash chain:** the ledger *is* the evidence the ship-gate reads. Ship's
audit-binding (see [trust-kernel-and-egps.md](trust-kernel-and-egps.md) §4) walks
this file backwards for the auditor's binding entry; if an attacker could silently
rewrite a past entry to fake a PASS-binding, audit-binding would be defeated. The
chain makes any rewrite detectable. The `chain` guard exposes `Verify` via
`evolve guard chain`.

### The on-disk ledger is never rewritten

A corollary: even to fix a malformed legacy entry, the bytes are **never** rewritten
— doing so would cascade hash breaks through every subsequent entry. The classic
case is the `cycle` field: manual operator entries historically wrote
`"cycle": "manual-release-v10.16.0"` (a string), which broke the int-typed walker.
Rather than rewrite history, `LedgerEntry.UnmarshalJSON` (`ports.go`) is a defensive
unmarshaler: a string `cycle` lands in `CycleLabel` with `Cycle = 0`; an int (or
whole-number float) lands in `Cycle`; a fractional float errors. Manual entries are
*required* to use `"cycle": 0` + `"cycle_label": "<semantic>"` going forward — the
numeric `cycle` field is reserved for the integer cycle sequence.

Entries record `role`, `kind` (`phase`, `agent_subprocess`, `routing_decision`,
`phase_plan`, `phase_skipped`, `reset`, …), and binding fields (`git_head`,
`tree_state_sha`, `worktree_tree_sha`, `artifact_sha256`, `challenge_token`). The
**per-cycle verifier** (`go/internal/ledgerverify`) walks the chain once per cycle
to assert the pipeline ran end-to-end (scout + builder + auditor each have an
exit-0 entry, plus intent/memo when required) — folding the two ledger vocabularies
(bash `agent_subprocess`/`builder` and Go `phase`/`build`) onto canonical buckets so
both verify identically (the cycle-137 fix).

---

## 5. Checkpoint & resume

`cycle-state.json` can carry a `checkpoint` block (written at
`EVOLVE_CHECKPOINT_AT_PCT`, default 95% of cumulative cost — a pre-emptive pause
before the budget is exhausted). Resume is `core/resume.go`:

- `LoadResumeState` reads the `checkpoint`, validates that **git HEAD has not
  drifted** since the pause (a mismatch is `ErrStaleCheckpoint` unless
  `EVOLVE_RESUME_ALLOW_HEAD_MOVED=1` downgrades it to WARN) and that the
  **worktree still exists**, then returns a `ResumePoint`.
- `RunCycleFromPhase` replays from `resumeFromPhase` onward. Critically, it does
  **not** increment `LastCycleNumber` (it operates on the cycle already in flight)
  and does **not** re-acquire the lock (the checkpoint was written under lock). It
  sets `EVOLVE_RESUME_MODE=1` in the env overlay.

**Why HEAD validation:** resuming a cycle whose audit was bound to an older tree
would let stale work ship. The checkpoint pins the tree it paused on; if `main`
moved, the checkpoint is stale by construction.

---

## 6. Reset & the unfinished-cycle guard

The complement of resume is `core/reset.go:SealCycle`, which **abandons** a stuck
cycle while preserving its full history:

1. The workspace is **moved** (never deleted) to `<workspace>.reset-<UTCnano>/`,
   with a verbatim `cycle-state.snapshot.json` + a `reset-manifest.json` (why/when/
   what) written *into* it **before** the rename — so the archive is complete the
   instant it appears (no split-across-two-dirs partial failure).
2. `LastCycleNumber` advances to the sealed cycle's number, so the number is
   **never reused** (the next cycle is N+1). Done via the full-fidelity map so
   unmodeled state survives.
3. An auditable, hash-chained `kind:reset` ledger entry is appended
   (`cycle: 0` + `cycle_label: reset-seal-cycle-N`).
4. `cycle-state.json` is **removed** — the abandon commit point that disarms the
   phase-gate precondition and lets a fresh cycle start clean.

A destructive rename is refused if `workspace_path` escapes both the evolve dir and
the project root (`pathWithin`) — defense against a corrupt state file pointing at
an arbitrary directory.

### The guard

A fresh `evolve loop` **refuses** to start (exit 2, `stop_reason=unfinished_cycle`)
when it detects an unfinished cycle — `cycle-state.json` ahead of `lastCycleNumber`,
or unreadable (truncated by a SIGKILL'd dispatcher). It prints the resume‖reset
fork (`go/cmd/evolve/cmd_loop.go:unfinishedCycle`). **Why:** silently starting fresh
would clobber the stuck cycle's history. The operator resolves it with
`evolve loop --resume` (continue) or `evolve cycle reset` (seal it). The escape
hatch `EVOLVE_FORCE_FRESH=1` restores the prior silent-clobber (history NOT sealed)
— operator-only.

---

## 7. Per-cycle git worktrees

Source-writing phases run in an **isolated per-cycle worktree** so a failed or buggy
cycle never mutates the live working tree. `core.WorktreeProvisioner`
(`core/worktree.go`) is the seam:

- `Create(projectRoot, cycle)` runs `git worktree add --detach <base>/cycle-N HEAD`,
  base = `EVOLVE_WORKTREE_BASE` or `<root>/.evolve/worktrees`. Idempotent (an
  existing worktree is reused). The path lands in `cycle-state.json:active_worktree`.
- `Cleanup` removes it on cycle exit (best-effort), *after* ship has merged the
  worktree → main.

> **Why this exists** (ADR-0027): the v11 Go port initially *dropped* worktree
> provisioning, which left `ActiveWorktree` empty and the role-gate's only
> source-write allowance (`phase==build && ActiveWorktree!=""`) permanently
> *unsatisfiable* — no phase could write code. Provisioning was restored behind an
> injected seam so `RunCycle` stays unit-testable without real git.

The isolation has three payoffs: (1) the live `main` tree is never half-written
mid-cycle; (2) a `SKIPPED_UNKNOWN` cycle's changes are discarded simply by deleting
the worktree; (3) the orchestrator's tree-diff leak guard
(`guards/treediff`) can detect any write that *escaped* the worktree into the main
tree by snapshotting `git diff --name-only HEAD` before/after each source phase —
because each worktree is a separate working dir, a leak shows up in the main tree's
dirty set and aborts the cycle.

---

## 8. The single-writer discipline

All of the above rests on one invariant: **exactly one writer per artifact at a
time.** The cycle lock (`Storage.AcquireLock`) serializes cycles; the worktree
isolates source writes; the role-gate confines each phase's writes; the ledger is
append-only with a hash chain. Even when fan-out is enabled
(`EVOLVE_FANOUT_ENABLED`), Builder/Intent/Orchestrator/TDD are excluded from
parallelism (`parallel_eligible`) because they are single-writers. The durable
substrate is designed so that, at any instant, there is one authoritative writer for
each piece of state — which is what makes a crashed cycle always recoverable to a
consistent point.
