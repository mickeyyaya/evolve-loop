# ADR-0072 — System-Failure Policy: classify, justify, and halt-and-diagnose instead of retrying the pipeline

- **Status:** Accepted (2026-07-17)
- **Supersedes/refines:** the standing rule `never_stop_queue_inject_inbox` (adds a bounded exception for system-level failures); complements ADR-0064 (pipeline-integrity boundary), ADR-0070 (signal-center exhaustion), the `failureadapter` kernel.
- **Deciders:** operator + session (post-mortem of the cycle 862→899 false-FAIL storm).

## Context — a loop that burned 38 cycles repeating the same result

The clean-exit deliverable-authority bug (fixed in `38b961d2`) caused the runner to synthesize a **FAIL** verdict from contaminated tmux scrollback whenever a phase agent exited clean-and-idle, even though the on-disk `audit-report.md` verdict was **PASS** and `acs-verdict.json` was **PASS**. The loop treated each occurrence as an ordinary task-level audit FAIL:

1. The cycle recorded FAIL → retro fired → the inbox items were released back to `.evolve/inbox/` root with **no memory of why they failed**.
2. Triage re-read the inbox root fresh the next cycle and re-selected the *same* task.
3. The task produced the *same* forged FAIL. Goto 1.

Result: the loop re-attempted **skill-overlay 5×** and **scoped-review 8×** across cycles 862–899, discarding verified-green work every time and burning tokens for an identical outcome. The consecutive-fail breaker did not save us because in fluent mode (`strict_audit` off) the failure-adapter's BLOCK rules degrade to `PROCEED`, and the breaker's default `max=1` is bypassed by the fleet/resume paths that re-triage released items.

### Root cause is architectural, not a single bug

The loop has **no progress monovariant**. It can repeat work indefinitely without checking whether the *outcome* is changing. "Burning tokens for the same result" is a livelock. Two properties were missing:

- **(P1) Coherence.** A recorded verdict is only trustworthy if it agrees with the artifacts the phases actually wrote. The clean-exit bug *forged a task-level verdict* — so any classifier reading the recorded verdict alone (human or LLM) would misclassify it as "task audit-fail → retry," the exact wrong call. A broken pipeline can lie about whose fault it is.
- **(P2) Termination.** The loop may only repeat work after a **coherent, progressing, task-level** result. Any incoherence, or any non-progress (same task/failure-class recurring, verified work not landing), is a **system fault** and must halt for diagnosis — not feed another retry.

## Decision

Introduce a **system-failure policy** that classifies every failure into a category, and — per the operator directive — **stops the loop and escalates for pipeline diagnosis on system-level failures** instead of retrying the todo. The decision is made by the **orchestrator** (LLM judgment, bounded by declarative policy), while **Go hard-enforces a non-negotiable floor** so a broken pipeline cannot be talked out of halting.

### Three-layer split

| Layer | Owns | Location |
|---|---|---|
| **Policy** (declarative) | Failure **categories**, each category's **level** (system/task), prescribed **action**, **fix-type**, thresholds | `.evolve/policy.json` → new `failure_policy` block; struct in `internal/policy` |
| **Orchestrator** (judgment) | Read the evidence dossier + policy → **classify** into one category, **justify** with cited evidence, **choose** next action + fix-type | `evo:evolve-orchestrator` / retrospective instructions → structured `failure-decision.json` |
| **Go** (deterministic) | Build the **evidence dossier** (coherence signal, non-progress counters, classification inputs); **enforce the floor**; enforce the chosen action; write escalation; deterministic **fallback** classification | `internal/failureadapter`, `internal/core`, `internal/inboxmover`, `cmd/evolve/cmd_loop*.go` |

**Authority model (operator-chosen): "orchestrator decides, Go enforces floor."** The orchestrator's classification/action is adopted *except* that two categories ALWAYS halt regardless of what the orchestrator says: `verdict-incoherence` and `infra-systemic`. When the orchestrator cannot run, the deterministic `failureadapter` (extended to read `failure_policy`) is the fallback (per `retro_always_on_failure`).

### The policy schema

```jsonc
// .evolve/policy.json  → additive block; compiled defaults live in internal/policy
"failure_policy": {
  "categories": {
    "verdict-incoherence": { "level":"system", "action":"halt-and-diagnose", "fix_type":"pipeline-repair",
      "signature":"recorded FAIL/WARN but on-disk audit AND acs verdicts are PASS",
      "floor":true },
    "infra-systemic":      { "level":"system", "action":"halt-and-diagnose", "fix_type":"pipeline-repair",
      "floor":true },
    "transport-hang":      { "level":"system", "action":"halt-and-diagnose", "fix_type":"pipeline-repair" },
    "non-progress":        { "level":"system", "action":"halt-and-diagnose", "fix_type":"pipeline-repair",
      "signature":"same task or same failure-class recurs >= repeat_ceiling cycles with no landed progress, OR verified-green work not landed >= verified_not_landed_ceiling times" },
    "code-build-fail":     { "level":"task", "action":"retry-with-fix", "fix_type":"build-repair",           "max_retries":2 },
    "code-audit-fail":     { "level":"task", "action":"retry-with-fix", "fix_type":"address-audit-findings", "max_retries":2 },
    "intent-malformed":    { "level":"task", "action":"defer-or-quarantine", "fix_type":"reintent" }
  },
  "thresholds": { "repeat_ceiling": 2, "verified_not_landed_ceiling": 2, "task_retry_ceiling": 2 },
  "on_task_retry_ceiling": "quarantine",       // stop re-picking a task-level poison todo
  "on_system_level":       "halt-loop-and-escalate"
}
```

Compiled Go defaults (in `internal/policy`, surfaced when the block is absent) match the table above so behavior is correct without editing the checked-in `policy.json` (mirrors the `gates` default pattern).

### The evidence dossier (deterministic, Go)

Built at cycle finalization from **independent on-disk artifacts** — never from the recorded verdict alone:

- **Coherence signal**: `(recordedVerdict, auditVerdict, acsVerdict, bridgeErrClass)` → `coherent | incoherent`. Incoherent ⇔ `recorded ∈ {FAIL,WARN}` **and** `auditVerdict==PASS` **and** `acsVerdict==PASS` **and** the audit phase actually ran (artifacts present) **and** no substantive bridge error occurred. This is a pure comparison (`CheckVerdictCoherence`), the deterministic input that lets the orchestrator (and the Go floor) catch a forged verdict.
- **Non-progress counters**: per-task failure count (from the inbox item's new failure memory) + verified-not-landed count (audit PASS + ACS PASS but no landed SHA) + same-class streak (from `state.failedApproaches`).
- **Classification inputs**: the existing `failureadapter.Entry` history + the bridge error class (`IsInfraTeardownError`, `ErrAllFamiliesExhausted`, transport-hang).

The dossier is written to `failure-dossier.json` in the cycle workspace and injected into the orchestrator/retrospective prompt.

### The floor (Go, always-on, cannot be overridden)

Before adopting any orchestrator/adapter decision, the loop checks the dossier:

- `coherence == incoherent` → `verdict-incoherence` → **HALT** (`ActionHaltDiagnose`).
- bridge error class == infra-systemic (`ErrAllFamiliesExhausted` / systemic teardown) → `infra-systemic` → **HALT**.

The floor is independent of `strict_audit` and independent of the orchestrator. It would have stopped the 862→899 storm at **cycle 862**, on the first forged verdict.

### Halt behavior — stop + escalate + diagnose

On any `halt-and-diagnose`:

1. Batch loop sets `StopReason = "system_failure_halt"` (new; distinct rc from task-fail/quota-pause/integrity-breach) and breaks.
2. Writes `.evolve/pipeline-escalation.json`: `{category, level, signature, evidence, suspectPhase, cyclesAffected, reproPointers, hypothesis}` (hypothesis from the orchestrator when present).
3. Auto-files a **P0 `pipeline-defect-<category>` inbox item** tagged `kind=pipeline-repair`, so on resume the pipeline fix is worked **before** any feature task (honoring the operator directive "deep dive into the pipeline issue and fix first"), and honoring `never_stop_queue_inject_inbox` for the *queue* even as the loop *halts*.

### Task-level: quarantine instead of infinite re-pick

For task-level categories, the inbox item gains a **durable failure memory** (`failureHistory: [{cycle, classification, verdict}]`) written by `inboxmover` on release. When a task's failure count reaches `task_retry_ceiling`, `inboxmover.Promote` routes it to `.evolve/inbox/quarantine/` (with the diagnostic) instead of back to root — triage stops re-selecting it, and the loop keeps working other tasks. This wires up the previously-dead `CyclesUnpicked` counter with real semantics.

### Reconciling with `never_stop_queue_inject_inbox`

That HIGHEST rule ("NEVER stop queue/loop") is **refined, not broken**: it governs **task-level** progress — never stop for permission, keep grinding the backlog, queue via inbox. A **system-level** failure is a different regime where continuing is damage, not progress. New unified rule:

> **Task-level failure → never stop** (retry within ceiling / defer / quarantine, queue via inbox). **System-level failure (verdict-incoherence, infra-systemic, non-progress) → halt + escalate + auto-file a P0 pipeline-repair item.** The *queue* is still injected (rule honored); the *loop* halts so the pipeline is fixed first.

## Alternatives considered

1. **Smarter deterministic retry only (no orchestrator).** Rejected: a pure classifier reading the recorded verdict is fooled by a forged verdict (P1). Deterministic logic is kept only as the *floor* + *fallback*, not the primary decider — categorization/justification is judgment work (Core Rule 5).
2. **Orchestrator full authority, no Go floor.** Rejected by the operator: a mis-classification could still let a system failure be retried. The floor guarantees a broken pipeline cannot be talked out of halting.
3. **Quarantine the poison task, keep looping (no halt).** Insufficient for *system*-level failures: a broken pipeline poisons *all* tasks, not one. Quarantine is the correct response for *task*-level repetition; halt is correct for system-level. The policy encodes both.
4. **Raise the consecutive-fail breaker threshold.** Treats the symptom (count) not the cause (incoherence/non-progress) and still repeats N times before stopping. The coherence floor stops at N=1.

## Consequences

- **Positive:** the loop gains a termination/progress guarantee; forged verdicts are caught on the first cycle; token burn on repeated system failures ends; single poison tasks are quarantined without stopping the whole loop; every failure carries a cited category + justified fix-type for the next cycle; system-level halts hand the operator a diagnostic + a queued P0 pipeline-repair task.
- **Negative / cost:** an autonomous overnight run now *halts* on a system-level failure instead of grinding — by design, this is the point (better to halt-and-wait than burn tokens repeating). Mitigation: the auto-filed P0 pipeline-repair item + escalation dossier make resumption a fast, guided fix.
- **Testing:** deterministic layers (coherence signal, floor, non-progress counters, quarantine routing, escalation write) are unit-tested TDD-first; the orchestrator decision is contract-tested against `failure-decision.json` schema with the deterministic fallback exercised when the artifact is absent.

## Implementation slices (one campaign)

- **S1 — Policy schema + loader:** `failure_policy` struct in `internal/policy` + compiled defaults + policy.json block. TDD.
- **S2 — Evidence dossier + coherence signal:** `CheckVerdictCoherence` + non-progress counters + `failure-dossier.json`. TDD.
- **S3 — Floor + halt:** `ActionHaltDiagnose`, floor enforcement in the retro-decision path, `StopReason="system_failure_halt"` + rc in `cmd_loop`. TDD.
- **S4 — Orchestrator integration:** instructions + `failure-decision.json` schema + consumption in `decideAfterRetro*`, Go floor override, deterministic fallback. Contract-tested.
- **S5 — Task quarantine:** inbox failure memory + `inboxmover` quarantine routing at `task_retry_ceiling`. TDD.
- **S6 — Escalation + auto-file:** `pipeline-escalation.json` + P0 `pipeline-defect` inbox item. TDD.
- **S7 — Docs + rule update:** this ADR, `runtime-reference.md` (StopReason table + failure_policy), AGENTS.md rule refinement, memory `feedback` update.
