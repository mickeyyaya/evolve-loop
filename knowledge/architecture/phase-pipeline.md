# Phase Pipeline — the cycle lifecycle

> One *cycle* is one trip through the orchestrator state machine: Scout discovers
> work, the pipeline turns RED tests GREEN, an adversarial auditor judges the
> result, and Ship commits it — or the cycle ends without shipping. This document
> describes the **current Go design** (v13.0.0). Every claim is traceable to a
> `go/internal/<pkg>` path or an ADR.

Related: [trust-kernel-and-egps.md](trust-kernel-and-egps.md) ·
[routing-and-advisor.md](routing-and-advisor.md) ·
[state-and-ledger.md](state-and-ledger.md) ·
[bridge-and-adapters.md](bridge-and-adapters.md) ·
[glossary](../00-overview/glossary.md)

---

## 1. The orchestrator is a pure driver

The whole pipeline lives in one function: `Orchestrator.RunCycle`
(`go/internal/core/orchestrator.go`). It is deliberately I/O-free at its own
layer — every side effect is delegated through an injected **port**:

| Port | Interface | Owns |
|---|---|---|
| `Storage` | `core.Storage` (`ports.go`) | `state.json`, `cycle-state.json`, the `.lock` |
| `Ledger` | `core.Ledger` (`ports.go`) | the hash-chained `ledger.jsonl` |
| `PhaseRunner` | `core.PhaseRunner` (`phase.go`) | actually *running* one phase |
| `WorktreeProvisioner` | `core.WorktreeProvisioner` (`worktree.go`) | the per-cycle git worktree |
| `Observer` | `core.Observer` (`observer.go`) | the per-phase stall detector |
| `DeliverableReviewer` | `core.DeliverableReviewer` (`reviewer.go`) | a pre-ledger quality gate |

The orchestrator is wired with these via functional options
(`WithRouting`, `WithPlanner`, `WithCatalog`, `WithObserver`, `WithReviewer`,
`WithWorktreeProvisioner` — all in `orchestrator.go`). The composition root
(`go/cmd/evolve/cmd_loop.go`) is the *only* place that reads env vars and
chooses concrete implementations. **Why:** the orchestrator never branches on a
feature flag; it depends on interfaces, so the same `RunCycle` is exercised
byte-for-byte in unit tests with fakes and in production with real git.

### The loop

`RunCycle` (orchestrator.go:403) does this, under a single cycle lock:

1. `ReadState` → `cycle = LastCycleNumber + 1`.
2. Archive any *polluted* workspace from a killed prior attempt at the same
   cycle number (orchestrator.go:`archivePollutedWorkspace`). Without this,
   leftover `scout-report.md` from a SIGKILL'd run makes Scout short-circuit on
   stale artifacts and steer the cycle to the wrong task (cycle-108 incident).
3. Provision the per-cycle worktree (`worktree.Create`). TDD/Build write source
   *here*, isolated from the live tree; every other phase only writes its
   artifact to the absolute workspace.
4. Mint a per-cycle **challenge token** (8-byte hex, `crypto/rand`) once, and
   thread it to every phase via `Context["challengeToken"]` + a
   `challenge-token.txt` sidecar. Phases use it as a tamper-evidence nonce in
   their ledger entries (see [state-and-ledger.md](state-and-ledger.md)).
5. Walk the state machine (a bounded `for safety := 0; safety < 32` loop). Each
   iteration: compute `next`, optionally let the router override it, run the
   phase, review its deliverable, run the tree-diff leak guard, append a ledger
   entry, persist `cycle-state.json`.
6. After the loop: `finalizeOutcome` disambiguates the cycle label, then
   `LastCycleNumber = cycle` is committed to `state.json`.

State is written **incrementally** after every phase, so a crash leaves an
inspectable trail that `--resume` can recover (see §8).

---

## 2. The phase graph

The canonical transition table is the `StateMachine`
(`go/internal/core/statemachine.go`). It is the runtime authority for two
questions: *is this edge legal?* (`CanTransition`) and *given a verdict, what
runs next?* (`Next`).

```
start ──┬─→ intent ──→ scout            (intent only if EVOLVE_REQUIRE_INTENT)
        └─→ scout
scout ──┬─→ triage ──→ tdd
        └─→ tdd | build                 (trivial-cycle skip edges)
tdd → build-planner → build → audit     (build-planner skipped unless enabled)
audit ──┬─→ ship    (PASS or WARN — EGPS v10 accepts WARN as a soft-pass)
        └─→ retro   (FAIL)
retro ──┬─→ ship    (recovered)
        ├─→ tdd     (RETRY, per failure-adapter)
        └─→ end     (BLOCK)
ship  ──┬─→ end
        └─→ debugger  (structured ShipError → advisor-recommended recovery)
debugger → {ship | audit | build | tdd | end}
```

The phase identities are `core.Phase` constants (`phase.go`): `start`, `intent`,
`scout`, `triage`, `tdd`, `build-planner`, `build`, `audit`, `ship`, `retro`,
`debugger`, `end`. Each phase emits one of four **verdicts** (`phase.go`):
`PASS`, `FAIL`, `WARN`, `SKIPPED`. These match the EGPS gate vocabulary — see
[trust-kernel-and-egps.md](trust-kernel-and-egps.md).

The first edge (`start → intent | scout`) is gated by *intent_required*, not by a
verdict: `NextFromStart(intentRequired)`. The one truly verdict-driven edge is
`audit → ship | retro`. The `retro → {tdd, ship, end}` and
`debugger → {…}` branches are **decision-driven**: the state machine returns
`end` by default and the orchestrator overrides via `scheduledNext`, reading the
deterministic failure-adapter (§5) or the debug-decision.

---

## 3. Per-phase contracts

Every subagent-dispatching phase shares one Template-Method skeleton:
`runner.BaseRunner` (`go/internal/phases/runner/runner.go`). A phase supplies a
small `Hooks` value (`PhaseName`, `AgentPromptName`, `ArtifactFilename`,
classification) and `BaseRunner.Run` does the identical surrounding work: profile
lookup → prompt composition → bridge dispatch → artifact read → verdict
classification → `PhaseResponse` packaging. **Why:** ~70 LoC of boilerplate per
phase collapses to ~5, and the external contract (`BridgeRequest` shape,
`PhaseResponse` fields) is fixed so per-phase integration tests survive refactors.

| Phase | Pkg | Deliverable | Contract |
|---|---|---|---|
| **Intent** | `phases/intent` | `intent.md` (or `intent-delta.md`) | Pre-Scout: structures a vague goal into intent before any subagent budget is spent. Opt-in (`EVOLVE_REQUIRE_INTENT=1`). |
| **Scout** | `phases/scout` | `scout-report.md` | Fans out into codebase / research / eval-design sub-scouts and merges. Selects the cycle's task by priority: new features > bug fixes > security. Reads the challenge token for its tamper-evident report. |
| **Triage** | `phases/triage` | triage notes | Layer-C triage on every cycle (soft-WARN if skipped). Disable: `EVOLVE_TRIAGE_DISABLE=1`. |
| **TDD** | `phases/tdd` | RED predicates / tests | Writes behavioral predicates **before** Build (EGPS Tester layer, `EVOLVE_TEST_PHASE_ENABLED=1`). Source-writing → runs in the worktree. |
| **Build-planner** | `phases/buildplanner` | `build-plan.md` | Advisory (ADR-0019). Shadow / advisory / enforce rollout via `EVOLVE_BUILD_PLANNER`. |
| **Build** | `phases/build` | code + `acs/cycle-N/*.sh` predicates | Single-writer; turns RED → GREEN. Source-writing → runs in the worktree. Writes the executable EGPS predicates the auditor will run. |
| **Audit** | `phases/audit` | `audit-report.md` + `acs-verdict.json` | Adversarial: requires positive evidence for PASS; defaults to Opus (different model family from Build's Sonnet) to break same-model sycophancy. Runs the predicate suite → `red_count`. |
| **Ship** | `phases/ship` | a commit on `main` | **Pure executor** — verifies + commits; cannot reject (audit already did). See §6. |
| **Retro** | `phases/retro` | retrospective + lesson | Runs on FAIL/WARN. Its successor is chosen by the failure-adapter. |
| **Debugger** | (recovery) | debug-decision | Optional. Diagnoses a structured `ShipError` and emits `RESHIP`/`RERUN_PHASE`/`BLOCK`. Never on the mandatory spine. |

`PhaseRequest` / `PhaseResponse` (`phase.go`) are the typed envelopes. Notable
fields: `PhaseResponse.Signals` is a namespaced bus (`<phase>.<key>`) the router
consumes instead of re-parsing markdown; `PhaseResponse.CommitSHA` anchors a
phase's deliverable to a commit (ADR-0027 commit-as-evidence).

### Source-writing phases run in the worktree

`worktreePhase(next)` (orchestrator.go) reports whether a phase writes source
(built-in `tdd`/`build`, or a user phase whose spec sets `writes_source`). Those
phases run with `cwd = active_worktree`; the **role-gate** (`guards/role.go`)
denies any Edit/Write outside the workspace or that worktree. **Why:** the live
`main` tree is never mutated mid-cycle, so a half-finished cycle is always
discardable by deleting the worktree.

---

## 4. Pure-function phases and the signal bus

The phases themselves are effectful (they shell out to LLM CLIs), but the
*decision* logic around them is pure:

- **Routing** (`go/internal/router`) is a pure function of `RouteInput` →
  `RouterDecision` (router.go). The caller does all I/O and hands in plain
  values; `Route` is deterministic.
- **The integrity floor** (`router/floor.go:ClampPlanToFloor`) returns a *new*
  plan plus the clamps applied — input unmutated.
- **The failure-adapter** (`go/internal/failureadapter`) computes
  PROCEED/RETRY/BLOCK from cycle history as a pure function.

This purity is the testability guarantee (ADR-0004, pure-function phases): the
hard logic — what to skip, when to block, how to clamp — is covered by table
tests with no git, no network, no LLM.

---

## 5. The retro branch — deterministic recovery

After Retro runs, the orchestrator does **not** ask an LLM what to do next. It
calls `decideAfterRetro` (orchestrator.go), which:

- On retro `PASS` → `ship` ("retrospective recovered the cycle"); no adapter call.
- Otherwise → consults `failureadapter.Decide(history)` over
  `state.json:failedApproaches[]`:
  - `RETRY_WITH_FALLBACK` → `tdd` (re-attempt with a fallback env overlay),
  - `BLOCK_*` → `end` (cycle history forbids further work),
  - `PROCEED` → `end` (no recovery, no block — exit cleanly).

The chosen branch is re-validated against the state machine
(`CanTransition(PhaseRetro, branch)`) before it runs. **Why deterministic:**
Core Rule 5 — retry/block decisions are mechanical (error codes, retention
windows), not judgment calls, so they belong in code, not a prompt. The reason
string is recorded in `CycleResult.RetroDecision` for the audit trail.

---

## 6. Ship is a pure executor (ShipError protocol)

The ship phase (`phases/ship`) **has no power to reject a cycle's changes** —
that decision was already made by Audit. Ship verifies the *process* is right
and executes (commit → ff-merge → push → release). When it cannot execute, it
returns a structured `core.ShipError` (`core/shiperror.go`) carrying a precise
`Code`, a severity `Class`, the `Stage` it failed at, and a `Debug` map.

**Why structured:** the old boundary collapsed ~50 distinct ship failure points
into one generic `ship: exit=N`, so the orchestrator couldn't tell a transient
push race from a stale audit-binding from a real tampering breach (cycle-148/150/151
incident family). Now each is a distinct `ShipErrorCode` with its own `Class`:

- `transient` — a relaunch may succeed (push race, network).
- `precondition` — an upstream precondition is stale but re-establishable
  (`AUDIT_BINDING_HEAD_MOVED`, tree mismatch).
- `integrity` — a genuine breach (`SELF_SHA_TAMPERED`, `INTEGRITY_TREE_DRIFT`);
  defaults to BLOCK.
- `config` — operator error (`INVALID_CLASS`, `COMMIT_GATE_MISSING`).

The orchestrator (not ship) decides what to do: route to the **debugger** phase,
re-run an upstream phase, or block. `ShipError` lives in `core` (not `ship`) so
the orchestrator can `errors.As` it without an import cycle. Ship's verification
stages (self-SHA TOFU, audit-binding, EGPS gate, commit-gate) are detailed in
[trust-kernel-and-egps.md](trust-kernel-and-egps.md).

---

## 7. Self-heal, observer, and the deliverable review gate

Three resilience layers wrap each `runner.Run` call:

- **Per-phase retry on artifact-timeout.** A bridge `ErrArtifactTimeout`
  (exit=81 — "agent produced no artifact within the wait window") is recoverable;
  the orchestrator relaunches the phase up to `phaseMaxAttempts = 2` times *on
  that sentinel only*. Every other error aborts the cycle immediately
  (orchestrator.go:711).
- **Stall observer** (ADR-0030). `observer.Start(ctx, phase, req)` runs a
  background watcher per phase that watches the agent's stdout-log; if it stops
  growing past `EVOLVE_OBSERVER_STALL_S` (600s) it emits a stall event (and, if
  `EVOLVE_OBSERVER_ENFORCE=1`, SIGTERMs the subagent). Auto-spawned when
  `EVOLVE_OBSERVER_AUTOSPAWN != "0"`. The default `noopObserver` is byte-identical
  to the pre-ADR-0030 cycle.
- **Deliverable review gate** (Workstream E2). For any non-`SKIPPED` verdict, the
  orchestrator calls `reviewer.Review` **before** the ledger append, so a reject
  aborts the cycle *without* recording the phase as a success. Default
  `noopReviewer` approves everything.

A **tree-diff leak guard** (`guards/treediff`) snapshots the main-tree dirty set
before each source-writing phase and re-checks after — any newly-dirty MAIN-tree
path is a write that escaped the worktree sandbox, and aborts the cycle before the
ledger records success.

---

## 8. Resume and reset

`cycle-state.json` is the per-cycle recovery anchor.

- **Resume** (`core/resume.go`). `LoadResumeState` reads the `checkpoint` block,
  validates that git HEAD hasn't drifted (`EVOLVE_RESUME_ALLOW_HEAD_MOVED=1`
  downgrades to WARN) and that the worktree still exists, then
  `RunCycleFromPhase` replays from `resumeFromPhase` onward — *without*
  incrementing `LastCycleNumber` and *without* re-acquiring the lock (the
  checkpoint was written under lock).
- **Reset** (`core/reset.go`). `SealCycle` abandons a stuck cycle: the workspace
  is *moved* (never deleted) to `<workspace>.reset-<UTCnano>/` with a verbatim
  state snapshot + `reset-manifest.json`, `LastCycleNumber` advances so the
  number is never reused, an auditable ledger entry is appended, and
  `cycle-state.json` is removed (the abandon commit point).

A fresh `evolve loop` **refuses** (exit 2) when it detects an unfinished cycle,
printing the resume‖reset fork — see [state-and-ledger.md](state-and-ledger.md).

---

## 9. Cycle outcome labels

`finalizeOutcome` (orchestrator.go) translates a bare `SKIPPED` final verdict into
a specific label, because "the formal ship phase didn't run" used to conflate
three very different things:

| Label | Meaning |
|---|---|
| `PASS` / `FAIL` / `WARN` | pass through unchanged |
| `SHIPPED_VIA_BUILD` | HEAD moved during the cycle (build committed inline) |
| `SKIPPED_AUDIT_ADVISORY` | retro recorded a "would-have-blocked" decision |
| `SKIPPED_UNKNOWN` | phases ran, HEAD didn't move, no advisory — loud WARN invites inspection |

A `SKIPPED_UNKNOWN` cycle emits a stderr WARN naming the workspace, because it
means work may have been produced and then discarded with the worktree (the
cycle-148 mis-grade). Source: cycle-107 shipped a real commit but reported a bare
`SKIPPED` — the loud label is the fix.

---

## 10. Dynamic routing entry point

By default (`EVOLVE_DYNAMIC_ROUTING=off`) the static state machine drives the
cycle, byte-identically to the legacy path: no router digest, no extra ledger
entries. When routing is enabled, the orchestrator digests the completed
handoffs, asks a `RoutingStrategy` for a decision, records it forensically, and —
at Advisory and above — lets the (floor-clamped) decision override the static
successor. The full advisor model, the non-bypassable integrity floor, and the
shadow→advisory→enforce rollout are documented in
[routing-and-advisor.md](routing-and-advisor.md).
