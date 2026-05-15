# Error Recovery — How Failures Don't Lose Work

> The four layers of failure handling: failedApproaches (cheap signal), retrospective (lesson extraction), checkpoint-resume (durable execution), and worktree preservation (in-flight work survival). Each layer catches a different failure mode at a different cost.
> Audience: operators running long cycles; anyone whose first cycle just crashed.

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

Long-running cycles fail. Not occasionally — *routinely*. Subscription quotas exhaust, API providers return 529 Overloaded, models hit context-window limits, transient network errors fire 503. Pre-v9.1.0, every such failure discarded **all in-flight work** — the per-cycle worktree was deleted by the EXIT trap unconditionally, leaving the operator with nothing to resume.

The cycle 11 incident (2026-05-11) was the trigger: three consecutive cycles aborted at audit phase after substantial work, each losing ~30 minutes of Builder edits because the cleanup trap was unconditional. v9.1.0 closes that gap with three layered mechanisms; v8.45.0 had already added the lesson-extraction layer for non-fatal failures.

Recovery is now part of the framework's contract: **work-in-flight survives common failures.**

---

## The Four Recovery Layers

| Layer | Trigger | Cost | What it preserves | Lifetime |
|---|---|---|---|---|
| **1. failedApproaches[]** | Audit FAIL/WARN OR run-cycle rc=1 | ~free (single state.json append) | Raw failure record (cycle, verdict, errorCategory) | 30 days default (`expiresAt`) |
| **2. Retrospective YAML lessons** | Audit FAIL/WARN (auto-on v8.45.0+) | $0.30–0.50 per cycle (Sonnet retrospective subagent) | Structured root-cause + prevention rule | Permanent (tracked YAML files + state.json:instinctSummary[]) |
| **3. Checkpoint-resume** | Cumulative cost ≥95% of cap, OR rc=1 with quota-exhaustion signature | Heavy (entire worktree + cycle-state preserved) | Full mid-cycle state — Builder's uncommitted edits, completed phases, cost-so-far | Until `--resume` or manual cleanup |
| **4. Worktree preservation** | Any rc≠0 if no checkpoint fired | None (passive — the worktree just doesn't get deleted) | Worktree edits surviving cleanup-skip | Until next `/evolve-loop --reset` or manual cleanup |

These are independent. A single failure may trigger 1, 2, 3, or 4 of them depending on its kind.

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

Cost: a single jq-update to state.json. Lifetime: 30 days default (operator can extend via `expiresAt` mutation or mark `systemic: true` to never expire).

**What next-cycle Scout does with it:** it gets the entry in its prompt context and uses `adaptiveFailureDecision` to choose between `PROCEED`, `RETRY`, or `BLOCK`. The mapping:

| Recent failures (count of same `errorCategory`) | Decision |
|---|---|
| 0 | PROCEED (no signal) |
| 1–2 in last 30d | PROCEED with caution (mention in scout-report) |
| 3+ in last 30d | RETRY (try same task with adjusted approach) |
| 3+ AND `systemic: true` | BLOCK (refuse this task class until operator intervenes) |

The decision is computed by `scripts/failure/failure-adapter.sh decide` — orchestrator reads its output and follows verbatim.

---

## Layer 2: Retrospective YAML Lessons (Mid-Cost)

When audit FAIL/WARN fires, the retrospective subagent runs inline (v8.45.0+). It reads the cycle's artifacts and produces:

- `retrospective-report.md` — prose narrative + `## Lessons` YAML block
- `handoff-retrospective.json` — machine handoff with `lessonIds[]` + `lessonFiles[]`
- `.evolve/instincts/lessons/<id>.yaml` — one file per lesson

Then `merge-lesson-into-state.sh` reads `handoff-retrospective.json`, verifies each YAML file exists on disk (integrity check), and appends to `state.json:instinctSummary[]`.

Cost: ~$0.30-0.50 per FAIL/WARN cycle (Sonnet). Output is permanent — see [self-evolution.md](self-evolution.md) for the cross-cycle learning mechanism.

**Operator opt-out:** `EVOLVE_DISABLE_AUTO_RETROSPECTIVE=1` reverts to pre-v8.45 record-only behavior. Useful for cost-control deployments where the lesson extraction isn't worth the cost.

---

## Layer 3: Checkpoint-Resume (Heavy, Durable Execution)

When a cycle is *about* to fail mid-flight (cost spike, quota signature) OR has already failed but in a way that's likely recoverable, the cycle's full state is checkpointed:

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

`run-cycle.sh`'s EXIT trap reads this block: if present, it SKIPs worktree removal, branch deletion, and cycle-state clear. The next operator invocation of `bash scripts/dispatch/evolve-loop-dispatch.sh --resume` picks up at the paused phase.

### Three triggers (v9.1.0+):

| Trigger | Cycle | Mechanism |
|---|---|---|
| **Reactive** | 3 | `subagent-run.sh` classifies non-zero exit + empty stderr + ≥80% cost as quota-likely → writes checkpoint |
| **Pre-emptive** | 2 | Dispatcher tracks `BATCH_TOTAL_COST`; at ≥95% (`EVOLVE_CHECKPOINT_AT_PCT`) exports `EVOLVE_CHECKPOINT_REQUEST=1` for the next cycle's orchestrator |
| **Operator-requested** | manual | `bash scripts/lifecycle/cycle-state.sh checkpoint operator-requested` |

### Env vars (v9.1.0+):

| Variable | Default | Purpose |
|---|---|---|
| `EVOLVE_CHECKPOINT_AT_PCT` | `95` | Pre-emptive trigger % (cost-based) |
| `EVOLVE_CHECKPOINT_WARN_AT_PCT` | `80` | Advisory WARN % |
| `EVOLVE_CHECKPOINT_DISABLE` | `0` | Set `1` to disable all checkpoint thresholds |
| `EVOLVE_QUOTA_DANGER_PCT` | `80` | Reactive classification cost threshold |
| `EVOLVE_RESUME_ALLOW_HEAD_MOVED` | `0` | Set `1` to bypass HEAD-drift guard on resume |

See [`../architecture/checkpoint-resume.md`](../architecture/checkpoint-resume.md) for the full protocol.

---

## Layer 4: Worktree Preservation (Last-Ditch)

If the cycle exited with rc≠0 but no checkpoint fired (e.g., a deterministic Build error rather than a quota signature), the worktree may still survive — depending on whether the dispatcher classified the failure as `recoverable` or not.

Classifier categories (from `scripts/dispatch/evolve-loop-dispatch.sh:classify_cycle_failure`):

| Classification | What it means | Worktree preserved? |
|---|---|---|
| `infrastructure` | 429, 529, EPERM, sandbox errors, transient network | YES (next cycle inherits + retries) |
| `audit-fail` / `audit-warn` | Cycle ran end-to-end; verdict not PASS | NO (worktree removed; lessons extracted at Layer 2) |
| `ship-gate-config` | Audit PASSed but ship-gate blocked (config drift) | YES (allows operator to clear gate condition + ship.sh manually) |
| `build-fail` | Builder failed without coherent report | NO |
| `exit-transport-hang` | (Opt-in via `EVOLVE_HANG_CLASSIFIER=1`) Two-factor: SHIPPED verdict + commit on main + rc=1 — reclassified from `integrity-breach` | YES if commit exists |
| `integrity-breach` | run-cycle rc≠0 + orchestrator-report unclassifiable | NO (operator must investigate; treat as breach) |

Operators who want stricter preservation can set `EVOLVE_PRESERVE_WORKTREE_ON_FAIL=1` (overrides the classification-based decision).

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

The dispatcher's exit code maps to operator intent:

| rc | Meaning | Operator action |
|---|---|---|
| 0 | All cycles completed and shipped | None — `git log` shows commits |
| 1 | Reserved (unused) | — |
| 2 | INTEGRITY-BREACH | Investigate before re-running |
| 3 | DONE-WITH-RECOVERABLE-FAILURES | Review `state.json:failedApproaches[]`; next run will adapt |
| 4 | BATCH-BUDGET-EXHAUSTED | Increase `--budget-usd` or run another batch |

---

## Operator Recovery Commands

When something goes wrong, these are the canonical commands:

| Situation | Command |
|---|---|
| Resume a checkpointed cycle | `bash scripts/dispatch/evolve-loop-dispatch.sh --resume` |
| Manually checkpoint a hung cycle | `bash scripts/lifecycle/cycle-state.sh checkpoint operator-requested` |
| Clear a stuck cycle-state | `bash scripts/lifecycle/cycle-state.sh clear` |
| Inspect what failed | `tail -50 .evolve/runs/cycle-N/orchestrator-stdout.log` |
| Verify ledger isn't tampered | `bash scripts/observability/verify-ledger-chain.sh` |
| Reset everything (nuclear) | `bash scripts/dispatch/evolve-loop-dispatch.sh --reset` |
| Re-render CLI Resolution post-hoc | `bash scripts/observability/render-cli-resolution.sh <cycle>` |
| Promote unshipped predicates to regression-suite | `bash scripts/utility/promote-acs-to-regression.sh <cycle>` |

The `--reset` does NOT touch Tier 1 hooks — they remain enforcing. Only Tier 3 workflow state is cleared.

---

## Worked Example: Cycle 11 Subscription Quota Wall

The motivating incident for v9.1.0. Detailed timeline:

```
T+0:00   /evolve-loop --cycles 3 invoked
T+0:02   Cycle 11 starts, Calibrate → Research → Build phases run
T+0:08   Cycle 11 audit phase begins
T+0:12   `claude -p` subprocess dies silently (rc=1, empty stderr)
         — Anthropic subscription quota window exhausted mid-audit
T+0:12   Pre-v9.1.0: EXIT trap fires; worktree removed; cycle-state cleared
         All Builder edits LOST; auditor partial output LOST
T+0:13   Dispatcher classifies as integrity-breach; exits rc=2
         Operator's only signal: empty stderr + missing orchestrator-report.md
```

Pre-v9.1.0, the operator's only recourse was to wait ~30 min for quota reset and re-run from scratch. ~$2-4 of work discarded.

Post-v9.1.0:

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

This is the canonical test of Layer 3. Subscription users (the dominant case for `/evolve-loop`) should NEVER lose >1 phase of work to quota.

---

## Anti-Patterns: What Recovery Is NOT

| Claim | Reality |
|---|---|
| "Recovery means the cycle will succeed eventually" | No — Layer 3 preserves *state*, not *outcome*. The same Builder code may still produce the same audit FAIL on resume. Layer 2 lessons may improve the next *new* cycle, not the resumed one. |
| "Checkpoint-resume avoids all costs" | No — the partial cost up to the checkpoint is sunk. Resume cost is the remaining phases' cost. |
| "Worktree preservation lets me cherry-pick by hand" | Possible but discouraged. Cycle integrity (audit-binding via tree SHA) requires ship.sh as the only ship path. Manual cherry-pick bypasses Tier 1 enforcement. |
| "Recovery means I never need to reset" | No — `--reset` is the right answer for kernel-confused states (e.g., when cycle-state.json got into an invalid state). The reset clears Tier 3 state only; Tier 1 + Tier 2 remain enforcing. |
| "Layer 3 protects against bad code" | No — checkpoint preserves Builder's edits regardless of correctness. The auditor's verdict on resume is computed fresh against the now-preserved code. |

---

## References

| Source | Relevance |
|---|---|
| GitHub Anthropic claude-code issue #29579 | Subscription quota signature (rc=1 + empty stderr after substantial work) — motivated v9.1.0 |
| [`../architecture/checkpoint-resume.md`](../architecture/checkpoint-resume.md) | Full Layer 3 protocol with env-var reference |
| [`../architecture/abnormal-event-capture.md`](../architecture/abnormal-event-capture.md) | Layer 1 (`abnormal-events.jsonl`) event taxonomy |
| [`../architecture/auto-resume.md`](../architecture/auto-resume.md) | Layer 3 resume mechanics |
| [`../architecture/retrospective-pipeline.md`](../architecture/retrospective-pipeline.md) | Layer 2 lesson extraction protocol |
| [self-evolution.md](self-evolution.md) | What happens to Layer 2's lessons in the *next* cycle |
| [`../incidents/cycle-61.md`](../incidents/cycle-61.md) §Memo | Memo's API 529 was a Layer 1 trigger — but classifier originally missed it (B5) |
