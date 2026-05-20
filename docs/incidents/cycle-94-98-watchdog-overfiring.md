# Cycle 94→98: Watchdog Post-Memo SIGTERM + Recovery Dance

> **Status: RESOLVED (v10.18.0)** — default flipped to `EVOLVE_OBSERVER_ENFORCE=1` (cycle 100). Phase-watchdog retained as DEPRECATED opt-out for one release window.
> **Severity:** MEDIUM (operator overhead, no integrity breach; real work shipped in every observed case)
> **Date range:** 2026-05-19 → 2026-05-20 (v10.16.0 → v10.17.0 batch)
> **Forensic dossiers:** [`watchdog-post-memo-sigterm-pattern`](../../knowledge-base/research/watchdog-post-memo-sigterm-pattern-2026-05-20.md), [`acs-promote-recovery-dance`](../../knowledge-base/research/acs-promote-recovery-dance-2026-05-20.md), [`dual-root-plugin-pattern-bite`](../../knowledge-base/research/dual-root-plugin-pattern-bite-2026-05-20.md)

## Summary

Five consecutive cycles (94, 95, 96, 97, 98) in the v10.17.0 release batch experienced `phase-watchdog.sh` SIGTERM during the post-memo `learn` phase. In every case the cycle's feat-commit landed on `main` BEFORE the SIGTERM fired — **real work shipped, but post-ship housekeeping (ACS predicate promotion to regression-suite) was lost** and required manual recovery. Total operator overhead: ~30-45 minutes plus ~$5-10 in additional ship commits across the batch.

This is **not a reward-hacking class incident** (no integrity invariant was violated, no forged artifacts). It's a **structural detector failure**: the file-mtime-based "no activity" heuristic miscounts the orchestrator's internal LLM finalization as idle, regardless of the threshold tuning.

## Timeline

| Time (UTC+8) | Cycle | Event | Commit |
|---|---|---|---|
| 2026-05-19 ~22:00 | 94 | feat shipped: token-economics roadmap P1+P5+L2 | `d24b403` |
| ~22:15 | 94 | watchdog SIGTERM at idle=248s (threshold=240s, +3%) | — |
| ~22:30 | 94 | Operator manual recovery: `ship.sh --class manual` | `89f2d08` |
| 2026-05-20 ~10:30 | 95 | feat shipped: auditor Sonnet right-sizing + O-1 | `392b064` |
| ~10:45 | 95 | watchdog SIGTERM at idle=250s (+4%) | — |
| ~11:00 | 95 | Operator manual recovery | `fb938bf` |
| ~11:30 | operator | perf shipped: watchdog default 240s → 600s | `ad07d25` |
| ~14:00 | 96 | feat shipped: builder turn-18 STOP + mastery | `1f40061` |
| ~14:30 | 96 | watchdog SIGTERM at idle=606s (threshold=600s, +1%) | — |
| ~14:45 | 96 | Operator manual recovery | `2af50aa` |
| ~17:30 | 97 | feat shipped: orchestrator digest-by-default | `a10ca24` |
| ~17:55 | 97 | `subagent-run.sh` INTEGRITY-FAIL on `orchestrator-report.md` mtime > 1279s | — |
| ~18:10 | 97 | Operator manual recovery | `3dbde30` |
| ~18:20 | 98 | feat shipped: PSMAS phase-skip foundation | `6466a3a` |
| ~18:35 | 98 | watchdog SIGTERM at idle=915s (threshold=900s, +2%) | — |
| ~18:50 | 98 | Operator manual recovery | `6461884` |
| ~10:57 (2026-05-21) | — | v10.17.0 release-pipeline.sh runs; marketplace sync closes the dual-root window | `6505fd3` |
| 2026-05-21 | 99 | watchdog post-memo SIGTERM recurred (turn-overrun event during PSMAS A/B verify); cycle work shipped before SIGTERM; manual recovery via `chore(cycle-99): promote ACS predicates` | `6c5248b` |

## Root cause

The watchdog (`scripts/dispatch/phase-watchdog.sh`) polls file mtimes within the cycle workspace every `EVOLVE_INACTIVITY_POLL_S=15` seconds. If the most-recent mtime is older than `EVOLVE_INACTIVITY_THRESHOLD_S`, it fires `SIGTERM` on the process group.

This proxy — *file write activity* as a stand-in for *agent activity* — fails for the orchestrator's post-memo finalization phase:

1. Memo agent has already written `carryover-todos.json` (file mtime updated)
2. Orchestrator now reads scout-report, build-report, audit-report and **reasons internally** (LLM tokens, no file writes) for 5-15+ minutes
3. Orchestrator writes the final `orchestrator-report.md` (file mtime updated again)
4. Between steps 1 and 3, the most-recent mtime is older than threshold → SIGTERM

Cycle 97 fired a *different* detector (`subagent-run.sh` INTEGRITY-FAIL on stale artifact) but the root cause is identical — both detectors measure file mtimes, both miscount LLM reasoning as idle.

**Why threshold tuning didn't fix it:** SIGTERM consistently fires at threshold + 1-4% across all 5 cycles regardless of tuning (240s→248s, 600s→606s, 900s→915s). The orchestrator's workload is roughly fixed (~15-22 min of LLM reasoning); whatever threshold is configured, the finalization wall-clock exceeds it by a small epsilon at the next file-touch.

## Compounding factor: dual-root plugin pattern

When the operator raised the watchdog default from 240 → 600s via `ad07d25`, the change took effect for the **project repo** (`/Users/danleemh/ai/claude/evolve-loop/`) but NOT for the **running plugin install** (`~/.claude/plugins/marketplaces/evolve-loop/`). The dispatcher's `find` expression in `skills/evolve-loop/SKILL.md` explicitly resolves to the plugin install, not the project repo. Cycles 96-98 required env-var override (`EVOLVE_INACTIVITY_THRESHOLD_S=600`) on the dispatcher invocation to apply the new default.

This is **architectural**, not a defect — the plugin install is the runtime-vetted release; the project repo is the source of truth. Closure required publishing v10.17.0 via `release-pipeline.sh`, which propagates the change via marketplace sync. See [`dual-root-plugin-pattern-bite`](../../knowledge-base/research/dual-root-plugin-pattern-bite-2026-05-20.md) for the full path topology.

## The recovery dance (operator workaround)

Each cycle's post-ship housekeeping step — `promote-acs-to-regression.sh`, which renames `acs/cycle-N/*.sh` → `acs/regression-suite/cycle-N/*.sh` — happens AFTER the feat-commit but BEFORE the orchestrator commits the rename. The watchdog SIGTERM hits during that window, leaving:

```
git status:
 D acs/cycle-N/predicate-name.sh
?? acs/regression-suite/cycle-N/
```

The operator's recovery script (5 invocations during the batch):

```bash
git add acs/cycle-N/ acs/regression-suite/cycle-N/
EVOLVE_SHIP_AUTO_CONFIRM=1 bash scripts/lifecycle/ship.sh --class manual \
  "chore(cycle-N): promote ACS predicates to regression-suite"
bash scripts/lifecycle/cycle-state.sh clear
git worktree remove --force <cycle worktree>
git branch -D evolve/cycle-N
```

See [`acs-promote-recovery-dance`](../../knowledge-base/research/acs-promote-recovery-dance-2026-05-20.md) §4 for the full runbook.

## What we shipped (short-term mitigation)

- `ad07d25` — raised watchdog default from 240s to 600s. Reduced SIGTERM frequency for typical cycles but did NOT eliminate the pattern (cycle 96 still fired at 606s).

This bought time but did not fix the structural issue. The pattern recurs at threshold + epsilon regardless of value.

## What's queued (long-term fix)

**Recommended: heartbeat-touch.** During the orchestrator's finalization phase, periodically `touch $WORKSPACE/.heartbeat` (every ≤ threshold/2 seconds). Watchdog observes the heartbeat file and resets baseline. Eliminates the false-idle window with one line of orchestrator-side code and zero watchdog-side changes.

```bash
# In agents/evolve-orchestrator.md, during memo/learn phase:
while finalizing; do
  touch "$WORKSPACE/.heartbeat"
  sleep 30
  # ... do actual work ...
done
```

**Alternative: tool-event detection.** Watchdog reads stream-json events from orchestrator stdout, counts `tool_use` messages as activity. Higher implementation cost; equivalently reliable.

**Companion fix: ACS promote in ship.sh post-commit hook.** Move the `promote-acs-to-regression.sh` invocation INTO `ship.sh` post-commit, so the promote and the feat-commit land in the same orchestrator turn (gap < 10s, well within any watchdog threshold). Eliminates 5 chore commits per batch. See [`acs-promote-recovery-dance`](../../knowledge-base/research/acs-promote-recovery-dance-2026-05-20.md) §4 Long-term Fix.

## Lessons for operators

1. **After a watchdog SIGTERM, check `git log -1 main` BEFORE assuming the cycle failed.** Per `feedback_orchestrator_hang_false_breach`: in every observed instance, the cycle's feat-commit landed before SIGTERM. The dispatcher classifier may report INTEGRITY-BREACH but the work itself is on `main`.

2. **Run the recovery dance immediately; don't re-dispatch.** The cycle-state is in a partial state that blocks normal operations via `role-gate.sh`. Clear it with `cycle-state.sh clear`, commit the pending promote with `ship.sh --class manual`, then re-dispatch.

3. **For dispatcher-script changes, ship via `release-pipeline.sh` not raw push.** The dual-root pattern means project-repo edits don't reach the plugin install without marketplace sync. Use `release-pipeline.sh X.Y.Z` to close the loop.

4. **Run `/clear` before starting a new batch.** Per CLAUDE.md v10.8.0+ note: `claude -p` subagent invocations bill to the parent OAuth session, not the batch budget meter. The batch cap (`EVOLVE_BATCH_BUDGET_CAP`) won't catch session-level spend across batches.

## Resolution (cycle-100)

Default flipped to `EVOLVE_OBSERVER_ENFORCE=1` in v10.18.0 (cycle 100). Phase-observer's event-based detection does not have the post-memo file-mtime failure mode. Phase-watchdog is retained for one release window as a DEPRECATED opt-out via `EVOLVE_OBSERVER_ENFORCE=0` (emits WARN).

**Status: RESOLVED (v10.18.0)**

## Cross-references

- [`knowledge-base/research/watchdog-post-memo-sigterm-pattern-2026-05-20.md`](../../knowledge-base/research/watchdog-post-memo-sigterm-pattern-2026-05-20.md) — full pattern analysis with mechanism table
- [`knowledge-base/research/acs-promote-recovery-dance-2026-05-20.md`](../../knowledge-base/research/acs-promote-recovery-dance-2026-05-20.md) — recovery runbook
- [`knowledge-base/research/dual-root-plugin-pattern-bite-2026-05-20.md`](../../knowledge-base/research/dual-root-plugin-pattern-bite-2026-05-20.md) — propagation mechanics
- [`knowledge-base/research/v10-17-0-release-debrief.md`](../../knowledge-base/research/v10-17-0-release-debrief.md) — multi-cycle synthesis and roadmap
- [`knowledge-base/research/phase-watchdog-stall-detection-cycle-89.md`](../../knowledge-base/research/phase-watchdog-stall-detection-cycle-89.md) — origin of the watchdog
- `docs/architecture/phase-observer.md` — watchdog/observer design (predates this incident)
- Memory: `feedback_orchestrator_hang_false_breach` — operator pattern: check `git log -1 main` first
