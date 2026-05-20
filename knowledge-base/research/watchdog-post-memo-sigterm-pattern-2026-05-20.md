# Watchdog Post-Memo SIGTERM Pattern — Cycles 94-98 (2026-05-20)

**Status:** Open (heartbeat-touch fix not yet shipped as of v10.17.0)
**Severity:** MEDIUM-HIGH (cost: ~$1-2/cycle in recovery dance; integrity: real work always ships before SIGTERM, but cycle 94-100 lessons + research were lost on the first attempt of each cycle)
**Functional impact:** Real work ships in every observed instance (5/5). Memo/learn output is lost.
**Structural impact:** The file-mtime-based "no activity" detector miscounts the orchestrator's internal LLM finalization as idle, regardless of tuned threshold. 5 consecutive cycles fired SIGTERM at threshold +1-4%.

Cross-reference: extends [phase-watchdog-stall-detection-cycle-89.md](phase-watchdog-stall-detection-cycle-89.md) — that dossier introduced the watchdog; this dossier documents the cycle 94-98 follow-up where structural over-fire was observed.

## 1. What happened

Five consecutive cycles in the v10.17.0 batch (cycles 94, 95, 96, 97, 98) experienced watchdog SIGTERM during the post-ship `learn` phase. In every case, the cycle's feat-commit had already landed on `main` before the SIGTERM fired — only memo/learn output was lost. The pattern was independent of threshold tuning:

| Cycle | Threshold | Idle when fired | Excess over threshold | Real work shipped? |
|---|---|---|---|---|
| 94 | 240s | 248s | +3% | yes (`d24b403`) |
| 95 | 240s | 250s | +4% | yes (`392b064`) |
| 96 | 600s | 606s | +1% | yes (`1f40061`) |
| 97 | n/a (artifact-staleness) | 1279s | (different detector) | yes (`a10ca24`) |
| 98 | 900s | 915s | +2% | yes (`6466a3a`) |

Cycle 97 fired a different detector (`subagent-run.sh` INTEGRITY-FAIL on `orchestrator-report.md` mtime > 1279s old) but the root cause is the same: the orchestrator does internal LLM work for 15-22 minutes post-memo without touching file mtimes, so the file-mtime-based "no activity" detector counts the entire window as idle.

Each cycle required a manual recovery dance:
1. ACS promote commit was uncommitted (`acs/cycle-N/*.sh` deleted, `acs/regression-suite/cycle-N/` untracked)
2. Operator runs `ship.sh --class manual` with `EVOLVE_SHIP_AUTO_CONFIRM=1` to commit the rename
3. Operator runs `cycle-state.sh clear` to remove stale cycle-state.json
4. Operator removes the cycle worktree + branch
5. Dispatcher re-launched with --resume or --cycles (N-1)

Total operator overhead across the 5 cycles: ~30-45 minutes plus ~$5-10 in additional ship commits.

## 2. Research

### File-mtime detector mechanics

`scripts/dispatch/phase-watchdog.sh` polls file mtimes within the cycle workspace every `EVOLVE_INACTIVITY_POLL_S=15` seconds. If the most-recent mtime across all workspace files is older than `EVOLVE_INACTIVITY_THRESHOLD_S` (raised from 240s to 600s default in `ad07d25`), the watchdog FIREs SIGTERM on the process group.

The detector measures *file write activity* as a proxy for *agent activity*. This proxy fails when the orchestrator is doing LLM work (turn after turn of reasoning) without writing files — exactly what the post-memo finalization phase requires:
- Memo agent has already written `carryover-todos.json`
- Orchestrator is now reading scout-report, build-report, audit-report to synthesize the final `orchestrator-report.md`
- It also calls `promote-acs-to-regression.sh` which moves files (which DOES touch mtimes)
- Then writes a final summary, which DOES touch mtimes
- But the LLM reasoning between these file writes is the long idle window

### Why threshold tuning doesn't help

Across 5 cycles, the SIGTERM consistently fired 1-4% over whatever threshold was configured. Hypothesis: the orchestrator's post-memo workload self-adjusts to "available time," similar to Parkinson's Law. More likely: each orchestrator's actual workload is roughly fixed (~15-22 min) and varies by 1-4% based on prompt content; the threshold + epsilon = wall-clock = whenever the next file-touch happens.

Either way, the conclusion is identical: file-mtime as proxy for activity is structurally wrong for this phase.

### Cycle 97 staleness variant

Cycle 97 fired a different code path: `subagent-run.sh` INTEGRITY-FAIL on stale artifact (`orchestrator-report.md` mtime > 1279s old when subagent-run.sh checked it during finalization). This is the same root cause via a different detector — both check file mtimes, both miscount LLM reasoning as idle.

### What other detectors could work?

| Approach | Mechanism | Cost | Reliability |
|---|---|---|---|
| **Heartbeat-touch** | Orchestrator periodically `touch`es a heartbeat file during finalization | Trivial (1 line + cron-style call) | High — eliminates the false-idle window |
| **Tool-event detection** | Watchdog reads stream-json events from orchestrator-stdout, counts tool_use messages as activity | Medium (parser + json handling) | High — directly measures LLM activity |
| **Phase-state polling** | Watchdog re-reads `cycle-state.json:phase` periodically; reset baseline on phase change | Trivial (existing watchdog already does this) | Medium — `learn` phase doesn't sub-divide, so still gets one long window |
| **PID-based monitoring** | Check if orchestrator process is in syscall-wait vs CPU-active | Complex (platform-specific) | High but fragile across macOS/Linux |

Recommended fix: **heartbeat-touch**. Lowest effort, highest reliability, matches existing detector. Orchestrator adds `touch "$WORKSPACE/.heartbeat"` every 30 seconds during finalization; watchdog observes the touched file and resets baseline.

## 3. Reasoning

The watchdog was originally designed to catch *infinite loops* or *deadlocked agents* (per cycle-89 dossier). Those failure modes show as zero file activity AND zero CPU activity — the detector works for them. The post-memo orchestrator finalization is a *different* failure mode: high CPU (LLM reasoning) but low file-write activity. The detector wasn't designed for this case and false-positives on it.

The fix is NOT "raise the threshold higher" — the structural pattern is over-fire at threshold + small epsilon, regardless of threshold. The fix is to change WHAT the detector measures.

Operator overhead in the meantime: ~5-10 min per cycle for the recovery dance. With 5+ cycles per batch typical, this is 25-50 minutes of recovery work per batch. The dual-root pattern (project repo edit vs running plugin install) compounds this — the project-repo `ad07d25` watchdog change from 240→600s didn't reach the plugin install until v10.17.0 marketplace-synced, requiring env-var override for the intermediate cycles. See [dual-root-plugin-pattern-bite-2026-05-20.md](dual-root-plugin-pattern-bite-2026-05-20.md).

## 4. Fix

### Short-term (shipped)

Cycle A operator-commit `ad07d25` raised the default threshold from 240s to 600s. This reduced the SIGTERM frequency for "typical" cycles but did NOT eliminate the pattern (cycle 96 still fired at 606s; cycle 98 at 915s). The 240→600s change is documented in `scripts/dispatch/phase-watchdog.sh:32`, `phase-observer.sh:116`, `run-cycle.sh:732,737`.

### Long-term (proposed, not yet shipped)

Add heartbeat-touch to the orchestrator persona during finalization phase:

```bash
# In agents/evolve-orchestrator.md, during memo/learn phase:
while finalizing; do
  touch "$WORKSPACE/.heartbeat"
  sleep 30  # less than EVOLVE_INACTIVITY_THRESHOLD_S/2
  # ... do actual work ...
done
```

Or change watchdog detector to consume stream-json events:

```bash
# In scripts/dispatch/phase-watchdog.sh:
last_event_ts=$(jq -r 'select(.type=="message") | .timestamp' \
  "$WORKSPACE/orchestrator-stdout.log" | tail -1)
# treat last_event_ts as last_activity_time
```

Either fix eliminates the false-idle window. Heartbeat-touch is recommended (lower effort, lower risk).

## 5. Lessons

- **[[cycle-94-retry-fast-fail-pattern]]** — sibling cycle-94 lesson; covers the per-agent retry counter pattern. Not directly related to watchdog but in the same batch.
- **[[cycle-93-build-report-commit-sha-fabrication]]** — cycle 93 had related builder-overrun + post-build issues but the failure mode is different (Builder turn-overrun, not orchestrator post-memo idle).

This dossier informs no immediate lesson yaml — the fix is not yet shipped, so the "double-loop change" cannot be canonicalized as a learned governing value. When heartbeat-touch ships, write `cycle-N-heartbeat-touch-eliminates-false-idle.yaml`.

## 6. References

- Source commits:
  - `d24b403` cycle-94 feat (P1 + P5 + L2)
  - `89f2d08` cycle-94 ACS promote chore
  - `392b064` cycle-95 feat (P2 + O-1)
  - `fb938bf` cycle-95 ACS promote chore
  - `ad07d25` perf(watchdog): 240→600s default
  - `1f40061` cycle-96 fix (builder STOP + mastery)
  - `2af50aa` cycle-96 ACS promote chore
  - `a10ca24` cycle-97 feat (L1)
  - `3dbde30` cycle-97 ACS promote chore
  - `6466a3a` cycle-98 feat (P3 PSMAS foundation)
  - `6461884` cycle-98 ACS promote chore
- Scripts:
  - `scripts/dispatch/phase-watchdog.sh` (file-mtime detector)
  - `scripts/dispatch/phase-observer.sh` (sibling detector, same flaw)
  - `scripts/dispatch/run-cycle.sh:730-738` (watchdog spawn site)
  - `scripts/dispatch/subagent-run.sh` (artifact-staleness detector, fired in cycle 97)
- Related dossiers:
  - [`phase-watchdog-stall-detection-cycle-89.md`](phase-watchdog-stall-detection-cycle-89.md) — origin
  - [`dual-root-plugin-pattern-bite-2026-05-20.md`](dual-root-plugin-pattern-bite-2026-05-20.md) — compounds recovery
  - [`acs-promote-recovery-dance-2026-05-20.md`](acs-promote-recovery-dance-2026-05-20.md) — operator workaround
- Memory references:
  - `feedback_orchestrator_hang_false_breach` — confirms the pattern; check `git log -1 main` for the cycle's commit before re-running
