# Checkpoint-Resume Protocol (v9.1.0+)

> Canonical reference for evolve-loop's durable-execution layer. Pre-v9.1.0,
> a cycle that hit a Claude Code subscription quota wall or any other rc=1
> in a phase lost ALL in-flight work. v9.1.0 closes that gap.

## Why this exists

Three consecutive `/evolve-loop` dispatcher runs in the 2026-05-11 session
aborted at `rc=1` with the same signature: orchestrator subagent exits
non-zero, empty stderr tail, no `orchestrator-report.md` produced. Each
failure happened **after substantial cycle work** (cycle 11 reached audit
phase before dying; cycle 14 had two successful cycles + a partial third).

When this happened pre-v9.1.0:

1. `run-cycle.sh`'s EXIT trap unconditionally deleted the per-cycle worktree.
2. `cycle-state.json` was cleared.
3. The dispatcher's batch loop aborted at `rc=1`.
4. **All mid-flight work was discarded** — Builder's edits in the worktree
   were gone; workspace artifacts in `.evolve/runs/cycle-N/*.md` survived
   but were useless without the worktree.

**Root cause** — based on [GitHub issue #29579](https://github.com/anthropics/claude-code/issues/29579)
and the failure signature, this matches Claude Code subscription quota
exhaustion. Subscription auth has rolling-window usage caps; when the cap
is hit mid-cycle, the nested `claude -p` subprocess dies silently (process-
level kill rather than JSON-streamed error) because the outer Claude Code
process consumes the rate-limit response. Stderr passthrough is empty.

## What v9.1.0 does

Three layered mechanisms, each opt-in or default-on as documented:

### 1. Checkpoint write (Cycle 1)

Add a `checkpoint` block to `cycle-state.json` that preserves the cycle's
state at the pause moment:

```json
{
  "cycle_id": 14,
  "phase": "build",
  "completed_phases": ["calibrate", "intent", "research", "triage"],
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

`run-cycle.sh`'s EXIT trap reads this block: if present, it SKIPs the
worktree removal, branch deletion, and cycle-state clear that the default
cleanup path would do. Worktree + state survive for `--resume`.

### 2. Pre-emptive threshold (Cycle 2)

Dispatcher tracks cumulative batch cost. Two thresholds fire BEFORE the
existing v8.58 hard tripwire (`BATCH_TOTAL_COST > BATCH_CAP`):

| Threshold | Default | What it does |
|---|---|---|
| `EVOLVE_CHECKPOINT_WARN_AT_PCT` | 80% | Emits `BATCH-BUDGET WARN` to stderr. Advisory only. |
| `EVOLVE_CHECKPOINT_AT_PCT` | 95% | Exports `EVOLVE_CHECKPOINT_REQUEST=1`. The next cycle's orchestrator reads this and pauses at its next phase boundary (graceful, not mid-cycle abort). |

`EVOLVE_CHECKPOINT_DISABLE=1` opts out entirely.

### 3. Reactive classification (Cycle 3)

`subagent-run.sh` examines every non-zero exit. If the failure matches the
Claude Code subscription quota-exhaustion signature:

- `cli_exit == 1`
- Stderr tail empty / whitespace-only / sentinel `<empty>`
- Cumulative cycle cost ≥ `EVOLVE_QUOTA_DANGER_PCT%` (default 80%) of
  `EVOLVE_BATCH_BUDGET_CAP`

…then it writes a checkpoint with `reason: quota-likely` and signals the
EXIT trap to preserve state.

### 4. `--resume` flag (Cycle 4)

```bash
/evolve-loop --resume
# or
bash scripts/dispatch/evolve-loop-dispatch.sh --resume
```

Calls `scripts/dispatch/resume-cycle.sh` which:

1. Locates the live checkpoint via `cycle-state.sh is-checkpointed`.
2. Validates: git HEAD unchanged since pause, worktree directory exists,
   state fields well-formed.
3. Prints a summary (cycle N, phase X, reason, cost-at-pause).
4. Re-spawns `run-cycle.sh` with three env vars:
   - `EVOLVE_RESUME_MODE=1`
   - `EVOLVE_RESUME_PHASE=<phase>`
   - `EVOLVE_RESUME_COMPLETED_PHASES=<csv list>`

`run-cycle.sh` honors `EVOLVE_RESUME_MODE`: skips `cycle_state_init`
(state already preserved) and skips worktree provision (worktree already
on disk). Hands off to the orchestrator with resume env vars set.

### 5. Orchestrator resume-mode protocol (Cycle 5)

Defined in `agents/evolve-orchestrator.md#resume-mode`:

1. Read preserved state via `cycle-state.sh get cycle_id`, `cycle-state.sh resume-phase`.
2. Skip every phase in `EVOLVE_RESUME_COMPLETED_PHASES` — their artifacts
   already exist in `$WORKSPACE`.
3. Call `cycle-state.sh clear-checkpoint` before the first phase advance
   (signals "the pause is over").
4. Pick up at `EVOLVE_RESUME_PHASE` and continue the normal Phase Loop.
5. If the cycle pauses again (e.g., quota still exhausted), write a new
   checkpoint and exit. `--resume` workflow is idempotent.

## Three pause triggers

| Trigger | Source | Mechanism |
|---|---|---|
| `quota-likely` | Cycle 3 reactive | `subagent-run.sh` detects rc=1 + empty stderr + cost in danger zone after phase failure |
| `batch-cap-near` | Cycle 2 pre-emptive | Dispatcher's cumulative cost ≥ 95% of cap; signals next cycle to checkpoint at clean phase boundary |
| `operator-requested` | Manual | `bash scripts/lifecycle/cycle-state.sh checkpoint operator-requested` |

## What gets preserved during checkpoint

| Asset | Preserved | Why |
|---|---|---|
| Worktree directory | Yes | Builder's edits live here; without it, all in-flight code work is lost |
| Branch (`evolve/cycle-N`) | Yes | Worktree references the branch; deleting one corrupts the other |
| `cycle-state.json` | Yes (with `checkpoint` block added) | Phase boundary + completed-phases history |
| `.evolve/runs/cycle-N/` workspace | Yes (run-cycle.sh doesn't touch it on EXIT) | Phase artifacts (intent.md, scout-report.md, etc.) |
| `.evolve/baselines/*.json` | Hoisted out before pause only if NOT checkpointed | When checkpointed the baselines stay in-worktree for the resumed cycle to read |
| Ledger | Always preserved (append-only, never cleaned) | Trust kernel invariant |

## What gets cleared on a normal (non-checkpoint) cycle end

The same things that always have. The default cleanup path is byte-identical
to pre-v9.1.0 when there is no checkpoint.

## Recovery scenarios

### Scenario A: Quota wall, same git HEAD

```bash
# Cycle 14 ran out of subscription quota at the build phase.
[run-cycle] CHECKPOINT: worktree + state preserved at /var/folders/.../cycle-14; resume with --resume

# Wait for quota window to roll forward, then:
$ /evolve-loop --resume
[resume-cycle] RESUME: cycle 14
[resume-cycle]   paused phase    : build
[resume-cycle]   pause reason    : quota-likely
[resume-cycle]   cost at pause   : $4.32
# ... cycle continues from build phase
```

### Scenario B: Quota wall, git HEAD moved (hot-fix)

```bash
$ /evolve-loop --resume
[resume-cycle] STALE: git HEAD moved since checkpoint
[resume-cycle]   paused at: abc123...
[resume-cycle]   current  : def456...
[resume-cycle]   override: EVOLVE_RESUME_ALLOW_HEAD_MOVED=1 to proceed anyway (risky)
```

The HEAD-moved check exists because Builder's edits in the worktree were
made against the paused HEAD. Resuming on a different HEAD risks merge
conflicts or worse. Override only when the hot-fix is known-orthogonal
to Builder's work.

### Scenario C: Worktree manually deleted

```bash
$ /evolve-loop --resume
[resume-cycle] STALE: worktree no longer exists at /var/folders/.../cycle-14
[resume-cycle]   recovery: cannot resume this cycle. Run /evolve-loop fresh.
```

Without the worktree, Builder's edits are unrecoverable. Start a new cycle.

### Scenario D: Crash during resume

```bash
$ /evolve-loop --resume
# ... orchestrator subprocess dies again
[resume-cycle] orchestrator subagent exited rc=3 during resume

# The checkpoint block survives (EXIT trap honored it). Retry:
$ /evolve-loop --resume
# ... picks up from the SAME phase boundary
```

The checkpoint block is only cleared when the orchestrator successfully
calls `cycle-state.sh clear-checkpoint` (typically at the start of resume
mode, after validating the state). If the orchestrator dies before that
call, the checkpoint survives for the next `--resume`.

## Env-var reference

| Var | Default | Role |
|---|---|---|
| `EVOLVE_CHECKPOINT_AT_PCT` | `95` | Cumulative cost threshold (% of `BATCH_CAP`) at which the dispatcher signals next cycle to checkpoint |
| `EVOLVE_CHECKPOINT_WARN_AT_PCT` | `80` | Cumulative cost threshold (% of `BATCH_CAP`) for the advisory WARN log |
| `EVOLVE_CHECKPOINT_DISABLE` | `0` | Set `1` to disable both pre-emptive thresholds (the existing v8.58 hard tripwire still applies) |
| `EVOLVE_QUOTA_DANGER_PCT` | `80` | Cost threshold for reactive classification: rc=1 + empty stderr below this is NOT classified as quota-likely. Set `0` to fire for any empty-stderr rc=1. Set `100` to effectively disable. |
| `EVOLVE_CHECKPOINT_REQUEST` | unset | Set by the dispatcher's 95% threshold — read by the next cycle's orchestrator |
| `EVOLVE_CHECKPOINT_TRIGGERED` | unset | Set by reactive classification — read by `run-cycle.sh`'s EXIT trap to force preserve-mode |
| `EVOLVE_RESUME_MODE` | unset | Set by `resume-cycle.sh` to `1`; orchestrator branches into resume protocol |
| `EVOLVE_RESUME_PHASE` | unset | Phase to start at during resume |
| `EVOLVE_RESUME_COMPLETED_PHASES` | unset | Comma-separated phases to skip during resume |
| `EVOLVE_RESUME_ALLOW_HEAD_MOVED` | `0` | Set `1` to bypass the git-HEAD-moved guard during resume |

## Trust kernel invariants (unchanged)

The trust kernel is preserved across the checkpoint-resume protocol:

- `phase-gate-precondition.sh` still enforces Scout→Builder→Auditor sequence
  on resume. Subagent invocations follow the same allowlist regardless of
  cycle origin (fresh vs. resumed).
- `role-gate.sh` still enforces Edit/Write path allowlists per the active
  phase. Builder in a resumed cycle gets the same path restrictions as
  Builder in a fresh cycle.
- `ship-gate.sh` still requires audit PASS before commit/push. A cycle
  that paused before audit cannot be coerced into shipping by `--resume`.
- Ledger SHA-chain is append-only and never cleaned, in both modes.
- Checkpoint writes go through the same `_atomic_write` (mv-of-temp) pattern
  used by every other cycle-state mutation.

## See also

- `docs/architecture/context-window-control.md` — paired capability that
  handles context-budget exhaustion the same way checkpoint-resume handles
  cost-budget exhaustion.
- `agents/evolve-orchestrator.md` — orchestrator persona resume-mode section.
- `scripts/dispatch/resume-cycle.sh` — script implementation.
- `scripts/tests/checkpoint-roundtrip-test.sh` — round-trip test of the
  checkpoint write primitives (19 assertions).
- `scripts/tests/resume-cycle-test.sh` — validation of resume-cycle.sh
  (26 assertions).

## Research references

- [LangGraph durable execution](https://docs.langchain.com/oss/python/langgraph/durable-execution) — checkpoint-per-step pattern
- [Zylos AI Agent Workflow Checkpointing 2026](https://zylos.ai/research/2026-03-04-ai-agent-workflow-checkpointing-resumability)
- [arxiv 2512.24511 — LLM Checkpoint/Restore I/O Strategies](https://arxiv.org/html/2512.24511v1)
- [Claude Code rate-limit issue #29579](https://github.com/anthropics/claude-code/issues/29579) — observed failure pattern
- [Claude Code rate-limits explained](https://www.sitepoint.com/claude-code-rate-limits-explained/) — subscription window mechanics
