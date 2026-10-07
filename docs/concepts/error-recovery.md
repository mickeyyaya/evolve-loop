# Error Recovery — How Failures Do Not Lose Work

> The four layers that handle failures are failedApproaches (cheap signal), retrospective (lesson extraction), checkpoint-resume (durable execution) and worktree preservation (survival of in-flight work). Each layer catches a different failure mode at a different cost.
> Audience: operators who run long cycles, and anyone whose first cycle just crashed.

## Table of Contents

1. [Why This Matters](#why-this-matters)
2. [The Four Recovery Layers](#the-four-recovery-layers)
3. [Layer 1: failedApproaches[] (Cheap, Always-On)](#layer-1-failedapproaches-cheap-always-on)
4. [Layer 2: Retrospective YAML Lessons (Mid-Cost)](#layer-2-retrospective-yaml-lessons-mid-cost)
5. [Layer 3: Checkpoint-Resume (Heavy, Durable Execution)](#layer-3-checkpoint-resume-heavy-durable-execution)
6. [Layer 4: Worktree Preservation (Last-Ditch)](#layer-4-worktree-preservation-last-ditch)
7. [Decision Tree: Which Layer Fires When](#decision-tree-which-layer-fires-when)
8. [Operator Recovery Commands](#operator-recovery-commands)
9. [Worked Example: Cycle 11 Subscription Quota Wall](#worked-example-cycle-11-subscription-quota-wall)
10. [Anti-Patterns: What Recovery Is NOT](#anti-patterns-what-recovery-is-not)
11. [References](#references)

---

## Why This Matters

Cycles that run for a long time fail. This does not occur occasionally: it occurs *routinely*. Subscription quotas become exhausted, API providers return 529 Overloaded, models hit context-window limits, and transient network errors fire 503. Before v9.1.0, every such failure discarded **all in-flight work**. The EXIT trap deleted the per-cycle worktree unconditionally, and the operator had nothing to resume.

The cycle 11 incident (2026-05-11) was the trigger. Three consecutive cycles aborted at the audit phase after substantial work. Each cycle lost ~30 minutes of Builder edits, because the cleanup trap was unconditional. v9.1.0 closes that gap with three layered mechanisms. v8.45.0 had already added the lesson-extraction layer for non-fatal failures.

Recovery is now part of the contract of the framework: **work-in-flight survives common failures.**

---

## The Four Recovery Layers

| Layer | Trigger | Cost | What it preserves | Lifetime |
|---|---|---|---|---|
| **1. failedApproaches[]** | Audit FAIL/WARN OR run-cycle rc=1 | ~free (single state.json append) | Raw failure record (cycle, verdict, errorCategory) | 30 days default (`expiresAt`) |
| **2. Retrospective YAML lessons** | Audit FAIL/WARN (auto-on v8.45.0+) | $0.30–0.50 per cycle (Sonnet retrospective subagent) | Structured root-cause + prevention rule | Permanent (tracked YAML files + state.json:instinctSummary[]) |
| **3. Checkpoint-resume** | Cumulative cost ≥95% of cap, OR rc=1 with quota-exhaustion signature | Heavy (the entire worktree + cycle-state are preserved) | Full mid-cycle state: the uncommitted edits of Builder, completed phases, cost-so-far | Until `--resume` or manual cleanup |
| **4. Worktree preservation** | Any rc≠0 if no checkpoint fired | None (passive: the worktree is just not deleted) | Worktree edits that survive the cleanup-skip | Until next `/evo:loop --reset` or manual cleanup |

These layers are independent. One failure can trigger 1, 2, 3 or 4 of them. The kind of the failure decides which.

---

## Layer 1: failedApproaches[] (Cheap, Always-On)

Every audit FAIL or WARN appends a structured record to `state.json:failedApproaches[]`:

```json
{
  "ts": "2026-05-10T01:58:01Z",
  "cycle": 7,
  "auditVerdict": "WARN",
  "errorCategory": "code-audit-warn",
  "failedStep": "artifact-persistence",
  "lessonIds": [
    "cycle-7-ephemeral-worktree-artifact",
    "cycle-7-ghost-complete-phase"
  ],
  "systemic": false,
  "expiresAt": "2026-06-09T01:58:01Z"
}
```

Cost: a single jq-update to state.json. Lifetime: 30 days by default. The operator can extend it with an `expiresAt` mutation, or mark it `systemic: true` so that it never expires.

**What the Scout of the next cycle does with it:** the Scout gets the entry in its prompt context. It uses `adaptiveFailureDecision` to choose between `PROCEED`, `RETRY` and `BLOCK`. The rules are:

| Recent failures (count of same `errorCategory`) | Decision |
|---|---|
| 0 | PROCEED (no signal) |
| 1–2 in last 30d | PROCEED with caution (mention in scout-report) |
| 3+ in last 30d | RETRY (try same task with adjusted approach) |
| 3+ AND `systemic: true` | BLOCK (refuse this task class until operator intervenes) |

`legacy/scripts/failure/failure-adapter.sh decide` computes the decision. The orchestrator reads its output and follows it verbatim.

---

## Layer 2: Retrospective YAML Lessons (Mid-Cost)

When audit FAIL/WARN fires, the retrospective subagent runs inline (v8.45.0+). It reads the artifacts of the cycle and produces:

- `retrospective-report.md`: a prose narrative + a `## Lessons` YAML block
- `handoff-retrospective.json`: the machine handoff with `lessonIds[]` + `lessonFiles[]`
- `.evolve/instincts/lessons/<id>.yaml`: one file for each lesson

Then `merge-lesson-into-state.sh` reads `handoff-retrospective.json`. It verifies that each YAML file exists on disk (integrity check), and appends to `state.json:instinctSummary[]`.

Cost: ~$0.30-0.50 per FAIL/WARN cycle (Sonnet). The output is permanent. See [self-evolution.md](self-evolution.md) for the mechanism that learns across cycles.

**Operator opt-out:** `EVOLVE_DISABLE_AUTO_RETROSPECTIVE=1` reverts to the record-only behavior of pre-v8.45. It is useful for cost-control deployments where the lesson extraction is not worth the cost.

---

## Layer 3: Checkpoint-Resume (Heavy, Durable Execution)

A cycle can be *about* to fail mid-flight (cost spike, quota signature). Or it can have already failed, but in a way that is likely recoverable. In both cases, a checkpoint records the full state of the cycle:

```json
{
  "cycle_id": 14,
  "phase": "build",
  "completed_phases": ["calibrate","intent","research","triage"],
  "active_worktree": "/var/folders/.../cycle-14",
  "checkpoint": {
    "enabled": true,
    "reason": "quota-likely",
    "savedAt": "2026-05-11T16:42:00Z",
    "resumeFromPhase": "build",
    "worktreePath": "/var/folders/.../cycle-14",
    "completedPhases": ["calibrate","intent","research","triage"],
    "gitHead": "abc123def456...",
    "costAtCheckpoint": 4.32
  }
}
```

The EXIT trap of `run-cycle.sh` reads this block. If the block is present, the trap SKIPs the worktree removal, the branch deletion and the cycle-state clear. The next operator invocation of `bash archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh --resume` continues at the paused phase.

### Three triggers (v9.1.0+):

| Trigger | Cycle | Mechanism |
|---|---|---|
| **Reactive** | 3 | `subagent-run.sh` classifies non-zero exit + empty stderr + ≥80% cost as quota-likely → writes checkpoint |
| **Pre-emptive** | 2 | The dispatcher tracks `BATCH_TOTAL_COST`. At ≥95% (`EVOLVE_CHECKPOINT_AT_PCT`), it exports `EVOLVE_CHECKPOINT_REQUEST=1` for the orchestrator of the next cycle. |
| **Operator-requested** | manual | `bash legacy/scripts/lifecycle/cycle-state.sh checkpoint operator-requested` |

### Env vars (v9.1.0+):

| Variable | Default | Purpose |
|---|---|---|
| `EVOLVE_CHECKPOINT_AT_PCT` | `95` | Pre-emptive trigger % (cost-based) |
| `EVOLVE_CHECKPOINT_WARN_AT_PCT` | `80` | Advisory WARN % |
| `EVOLVE_CHECKPOINT_DISABLE` | `0` | Set `1` to disable all checkpoint thresholds |
| `EVOLVE_QUOTA_DANGER_PCT` | `80` | Reactive classification cost threshold |
| `EVOLVE_RESUME_ALLOW_HEAD_MOVED` | `0` | Set `1` to bypass the HEAD-drift guard on resume |

See [`../architecture/checkpoint-resume.md`](../architecture/checkpoint-resume.md) for the full protocol.

---

## Layer 4: Worktree Preservation (Last-Ditch)

Sometimes the cycle exits with rc≠0, but no checkpoint fires (for example, a deterministic Build error and not a quota signature). The worktree can then still survive. It survives if the dispatcher classified the failure as `recoverable`.

Classifier categories (from `archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh:classify_cycle_failure`):

| Classification | What it means | Worktree preserved? |
|---|---|---|
| `infrastructure` | 429, 529, EPERM, sandbox errors, transient network | YES (the next cycle inherits + retries) |
| `audit-fail` / `audit-warn` | The cycle ran end-to-end; the verdict is not PASS | NO (worktree removed; lessons extracted at Layer 2) |
| `ship-gate-config` | Audit PASSed but ship-gate blocked (config drift) | YES (the operator can clear the gate condition + run ship.sh manually) |
| `build-fail` | Builder failed without a coherent report | NO |
| `exit-transport-hang` | (Opt-in through `EVOLVE_HANG_CLASSIFIER=1`) Two-factor: SHIPPED verdict + commit on main + rc=1, reclassified from `integrity-breach` | YES if the commit exists |
| `integrity-breach` | run-cycle rc≠0 + orchestrator-report unclassifiable | NO (the operator must investigate; treat as a breach) |

Operators who want stricter preservation can set `EVOLVE_PRESERVE_WORKTREE_ON_FAIL=1`. It overrides the decision that the classification makes.

---

## Decision Tree: Which Layer Fires When

```
                  Cycle exits
                       │
          ┌────────────┴────────────┐
          │                         │
       rc == 0?                  rc != 0?
          │                         │
          ↓                         ↓
   Verdict PASS?           Failure signature?
          │                         │
      ┌───┴───┐         ┌───────────┼───────────┐
      ↓       ↓         ↓           ↓           ↓
   memo    fail?    infrastructure  audit-fail  integrity-breach
   fires   (FAIL    or quota wall?  or build?   (unclassifiable)
   (Layer  /WARN)                    │              │
   2 if    │           ↓             ↓              ↓
   any     ↓        Layer 3       Layer 1 +      Layer 1 +
   defer-  Layer 1 +  Checkpoint   Layer 2:      EXIT (rc=2);
   rables) Layer 2:   (worktree   record +       NO recovery
           record +   + state      lessons +     attempted —
           lessons    preserved   worktree       operator
                      via         removed         must
                      EXIT trap                   investigate
                      skipping
                      cleanup)
```

The exit code of the dispatcher maps to operator intent:

| rc | Meaning | Operator action |
|---|---|---|
| 0 | All cycles completed and shipped | None: `git log` shows the commits |
| 1 | Reserved (unused) | — |
| 2 | INTEGRITY-BREACH | Investigate before you run again |
| 3 | DONE-WITH-RECOVERABLE-FAILURES | Review `state.json:failedApproaches[]`; the next run will adapt |
| 4 | BATCH-BUDGET-EXHAUSTED | Increase `--budget-usd` or run another batch |

---

## Operator Recovery Commands

When something goes wrong, use these canonical commands:

| Situation | Command |
|---|---|
| Resume a checkpointed cycle | `bash archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh --resume` |
| Manually checkpoint a hung cycle | `bash legacy/scripts/lifecycle/cycle-state.sh checkpoint operator-requested` |
| Clear a stuck cycle-state | `bash legacy/scripts/lifecycle/cycle-state.sh clear` |
| Inspect what failed | `tail -50 .evolve/runs/cycle-N/orchestrator-stdout.log` |
| Verify that the ledger is not tampered | `bash legacy/scripts/observability/verify-ledger-chain.sh` |
| Reset everything (nuclear) | `bash archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh --reset` |
| Re-render CLI Resolution post-hoc | `bash legacy/scripts/observability/render-cli-resolution.sh <cycle>` |
| Promote unshipped predicates to regression-suite | `bash legacy/scripts/utility/promote-acs-to-regression.sh <cycle>` |

The `--reset` does NOT touch Tier 1 hooks: they stay in force. It clears only the Tier 3 workflow state.

---

## Worked Example: Cycle 11 Subscription Quota Wall

This incident caused v9.1.0. The detailed timeline:

```
T+0:00   /evo:loop --cycles 3 invoked
T+0:02   Cycle 11 starts, Calibrate → Research → Build phases run
T+0:08   Cycle 11 audit phase begins
T+0:12   `claude -p` subprocess dies silently (rc=1, empty stderr)
         — Anthropic subscription quota window exhausted mid-audit
T+0:12   Pre-v9.1.0: EXIT trap fires; worktree removed; cycle-state cleared
         All Builder edits LOST; auditor partial output LOST
T+0:13   Dispatcher classifies as integrity-breach; exits rc=2
         Operator's only signal: empty stderr + missing orchestrator-report.md
```

Before v9.1.0, the only recourse of the operator was to wait ~30 min for the quota reset and run again from scratch. That discarded ~$2-4 of work.

After v9.1.0:

```
T+0:12   `subagent-run.sh` examines rc=1 + empty stderr + cost ≥80%
         → classifies as quota-likely
         → writes checkpoint to cycle-state.json (Layer 3, reactive)
T+0:12   EXIT trap reads checkpoint block → SKIPS worktree removal
T+0:13   Dispatcher reports CHECKPOINT-PRESERVED rc=4
T+30:00  Operator's quota window resets
T+30:01  Operator runs `evolve-loop-dispatch.sh --resume`
T+30:02  Resume reads checkpoint, picks up at audit phase
         Builder's edits intact in worktree
T+35:00  Cycle 11 completes; ships normally
```

Net work loss: ~5 minutes (the partial audit phase). The ~$2-4 of Builder work survives.

This is the canonical test of Layer 3. The expected result: subscription users (the dominant case for `/evo:loop`) NEVER lose >1 phase of work to quota.

---

## Anti-Patterns: What Recovery Is NOT

| Claim | Reality |
|---|---|
| "Recovery means the cycle will succeed eventually" | No. Layer 3 preserves *state*, not *outcome*. The same Builder code can still produce the same audit FAIL on resume. Layer 2 lessons can improve the next *new* cycle, not the resumed one. |
| "Checkpoint-resume avoids all costs" | No. The partial cost up to the checkpoint is sunk. The resume cost is the cost of the phases that remain. |
| "Worktree preservation lets me cherry-pick by hand" | Possible but discouraged. Cycle integrity (audit-binding through the tree SHA) requires ship.sh as the only ship path. A manual cherry-pick bypasses Tier 1 enforcement. |
| "Recovery means I never need to reset" | No. `--reset` is the right answer for kernel-confused states (for example, when cycle-state.json got into an invalid state). The reset clears only Tier 3 state; Tier 1 + Tier 2 stay in force. |
| "Layer 3 protects against bad code" | No. The checkpoint preserves the edits of Builder, regardless of correctness. On resume, the auditor computes a fresh verdict against the code that is now preserved. |

---

## References

| Source | Relevance |
|---|---|
| GitHub Anthropic claude-code issue #29579 | Subscription quota signature (rc=1 + empty stderr after substantial work). It motivated v9.1.0. |
| [`../architecture/checkpoint-resume.md`](../architecture/checkpoint-resume.md) | Full Layer 3 protocol with env-var reference |
| [`../architecture/abnormal-event-capture.md`](../architecture/abnormal-event-capture.md) | Layer 1 (`abnormal-events.jsonl`) event taxonomy |
| [`../architecture/auto-resume.md`](../architecture/auto-resume.md) | Layer 3 resume mechanics |
| [`../architecture/retrospective-pipeline.md`](../architecture/retrospective-pipeline.md) | Layer 2 lesson extraction protocol |
| [self-evolution.md](self-evolution.md) | What happens to the lessons of Layer 2 in the *next* cycle |
| [`../incidents/cycle-61.md`](../incidents/cycle-61.md) §Memo | The API 529 of Memo was a Layer 1 trigger, but the classifier originally missed it (B5) |
