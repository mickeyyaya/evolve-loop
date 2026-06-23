# v10.17.0 Release Debrief — 2026-05-20

**Status:** Shipped 2026-05-20 (commit `6505fd3 release: v10.17.0`)
**Severity:** N/A — release retrospective
**Functional impact:** Shipped 4 substantive features (P1, P5, L2; P2 + O-1; L1; P3 PSMAS foundation) + 1 operator-shipped watchdog default change. P4 found to be already-done in a prior cycle.
**Structural impact:** First release where the cycle pipeline shipped multiple roadmap items in a single batch. Established post-memo SIGTERM pattern as a known structural limitation.

## 1. What happened

The v10.17.0 release packaged 11 commits accumulated across cycles 94-100:

| # | Commit | Type | Cycle | Plan match | Substance |
|---|---|---|---|---|---|
| 1 | `d24b403` | feat | 94 | yes (P1+P5+L2) | retry fast-fail + retro YAML extern + stream-json visibility |
| 2 | `89f2d08` | chore | 94 | — | ACS promote rollup |
| 3 | `392b064` | feat | 95 | yes (P2+O-1) | auditor Sonnet right-sizing + fast-fail counter scope docs |
| 4 | `fb938bf` | chore | 95 | — | ACS promote rollup |
| 5 | `ad07d25` | perf | operator | (extra) | watchdog default 240→600s |
| 6 | `1f40061` | fix | 96 | no (diverged) | builder turn-18 STOP + mastery (triage diverged from plan P4+L1) |
| 7 | `2af50aa` | chore | 96 | — | ACS promote rollup |
| 8 | `a10ca24` | feat | 97 | yes (L1, P4 no-op) | orchestrator digest-by-default (P4 found already-done) |
| 9 | `3dbde30` | chore | 97 | — | ACS promote rollup |
| 10 | `6466a3a` | feat | 98 | yes (P3 foundation) | PSMAS phase-skip foundation (opt-in via EVOLVE_PSMAS_SKIP=1) |
| 11 | `6461884` | chore | 98 | — | ACS promote rollup |
| 12 | `6505fd3` | release | — | — | v10.17.0 version bump + CHANGELOG |

**Plan completion: 4 of 5 priorities shipped substantively.** P3 A/B verification half (rerun 5 historical cycles under PSMAS-skip, assert ≥20% token reduction) deferred to a future batch.

## 2. Research

### Per-cycle cost breakdown (from per-phase *-usage.json)

| Cycle | Intent | Scout | Triage | TDD | Builder | Auditor | Memo | Subtotal |
|---|---|---|---|---|---|---|---|---|
| 94 | $0.96 | $1.55 | $0.11 | $1.37 | $1.42 | $1.28 | $0.15 | **$6.85** |
| 95 | (unrecorded) | (unrecorded) | (unrecorded) | (unrecorded) | (unrecorded) | (unrecorded) | (unrecorded) | ~$6-7 |
| 96 | $0.78 | $1.12 | $0.14 | $3.06 | $1.13 | $1.17 | — | **$7.40** |
| 97 | $1.09 | $1.02 | $0.18 | $2.00 | (variable) | (variable) | — | ~$5-7 |
| 98 | $0.76 | (missing) | $0.10 | $2.47 | $1.07 | $1.43 | $0.11 | **$5.94** |

**Cycle 98 was cleanest** — triage $0.10 + builder $1.07/51 turns + auditor $1.43/18 turns. This reflects cumulative gains from cycles 94-97 shipped optimizations (P1 retry, P2 Sonnet auditor, L1 digest mode) — though the runtime was still pre-sync plugin install for those cycles.

### What didn't match plan

- **Cycle 96 diverged** — triage chose builder-STOP + mastery over the operator-stated P4+L1. See [`triage-autonomous-goal-divergence-cycle-96.md`](triage-autonomous-goal-divergence-cycle-96.md). Working as designed.
- **P4 was a no-op** — cycle 97 found `agents/evolve-triage-reference.md` already extracted. Operator's plan was over-scoped. The intent persona correctly adjudicated and skipped.
- **P3 A/B verification deferred** — only the foundation shipped (default-off opt-in). Future cycle to flip default-on after measuring.

### Recurring problem: post-memo watchdog SIGTERM

All 5 cycles (94, 95, 96, 97, 98) experienced watchdog SIGTERM during the learn phase. See [`watchdog-post-memo-sigterm-pattern-2026-05-20.md`](watchdog-post-memo-sigterm-pattern-2026-05-20.md) — fires at threshold + 1-4% regardless of tuning (240s → 248s, 600s → 606s, 900s → 915s). Real work shipped before each SIGTERM but learn-phase output was lost. Required 5 manual recovery commits (the ACS promote rollups above). See [`acs-promote-recovery-dance-2026-05-20.md`](acs-promote-recovery-dance-2026-05-20.md).

### Session cost vs batch cost divergence

Per `CLAUDE.md` session-cost-isolation note (v10.8.0+): `claude -p` subagent invocations bill to the OAuth session that launched the dispatcher (the parent Claude Code session), NOT the batch budget meter. So `state.json:currentBatch.cycleAccruedCostUSD` shows $0 while session cost climbed to >$1000. The batch cap (`EVOLVE_BATCH_BUDGET_CAP`, default $20, operator-raised to $100) tracks per-cycle accumulation only.

This was a real surprise: operator raised cap to $100 expecting it would catch runaway cost, but the meter doesn't see the actual session-level spend. **Run `/clear` before a new batch to isolate session cost** (per CLAUDE.md).

### Dual-root plugin pattern bite

`ad07d25` watchdog default change took effect for the operator's project repo but not the running plugin install. Env-var override (`EVOLVE_INACTIVITY_THRESHOLD_S=600`) was required for cycles 96-98. v10.17.0 marketplace sync closed the gap. See [`dual-root-plugin-pattern-bite-2026-05-20.md`](dual-root-plugin-pattern-bite-2026-05-20.md).

## 3. Reasoning

The release succeeded in shipping the planned scope (4/5 plan priorities substantively + 1 operator addition) but accumulated significant friction in the process:
- 5 manual recovery commits required (post-memo SIGTERM cascade)
- 1 cycle goal-divergence (cycle-96, working as designed but unexpected)
- 1 dual-root plugin pattern bite (env-var workaround until marketplace sync)
- Session cost climbed past the operator's intended $100 batch cap (visible but not enforced at session level)

Lessons for the NEXT release:
1. **Heartbeat-touch should ship first.** Eliminates the post-memo SIGTERM cascade. See watchdog dossier §4 Long-term Fix.
2. **Promote step should land in feat commit OR ship.sh post-hook.** Eliminates the 5 chore commits per batch. See acs-promote dossier §4 Long-term Fix.
3. **Operator should run `/clear` before starting a batch.** Isolates session cost from batch cost (per CLAUDE.md v10.8.0 note).
4. **Operator should expect goal divergence.** The triage system is autonomous; goal text is one input among several. Read `triage-decision.md` early to confirm direction.
5. **Use release-pipeline.sh for dispatcher changes.** Project-repo edits don't reach the running plugin install without marketplace sync.

## 4. Fix

No code fix in this dossier — this is a release retrospective. The fixes that emerged from this batch are scheduled for future cycles:

| Fix | Owner | Target cycle | Source dossier |
|---|---|---|---|
| Heartbeat-touch during learn-phase | orchestrator persona + watchdog | next available | watchdog-post-memo-sigterm-pattern |
| ACS promote in feat commit | ship.sh + orchestrator | next available | acs-promote-recovery-dance |
| P3 PSMAS A/B verification | dispatcher + 5 historical cycle reruns | next available | (in roadmap plan) |
| Operator `/clear` documentation in README | README rework Phase 3 | next available | this dossier |

## 5. Lessons

- **[[cycle-94-retry-fast-fail-pattern]]** — P1 shipped
- **[[cycle-95-auditor-mastery-gate]]** — P2 + O-1 shipped
- **[[cycle-96-builder-turn-18-stop]]** — fix shipped (with 54:1 test:code ratio flag for human review)
- **[[cycle-97-orchestrator-digest-default]]** — L1 shipped; P4 found already-done
- **[[cycle-98-psmas-phase-skip-foundation]]** — P3 foundation shipped (opt-in)

## 6. References

- Release commit: `6505fd3 release: v10.17.0` — 5 files (plugin.json, marketplace.json, SKILL.md, README.md, CHANGELOG.md)
- GitHub release: https://github.com/mickeyyaya/evolveloop/releases/tag/v10.17.0
- Marketplace propagation: confirmed 1s after push (per release-pipeline.sh log)
- Release journal: `.evolve/release-journal/10.17.0-20260520T105747Z.json`
- All 12 commits visible via: `git log --oneline 8cff9c1..6505fd3`
- Plan source: `~/.claude/plans/innovate-think-of-what-fluffy-rose.md`
- Cross-references (sibling dossiers):
  - [`watchdog-post-memo-sigterm-pattern-2026-05-20.md`](watchdog-post-memo-sigterm-pattern-2026-05-20.md)
  - [`dual-root-plugin-pattern-bite-2026-05-20.md`](dual-root-plugin-pattern-bite-2026-05-20.md)
  - [`acs-promote-recovery-dance-2026-05-20.md`](acs-promote-recovery-dance-2026-05-20.md)
  - [`triage-autonomous-goal-divergence-cycle-96.md`](triage-autonomous-goal-divergence-cycle-96.md)
