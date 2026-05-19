# Phase-Watchdog Stall Detection — Cycle 89 Operational Learning

> **Status:** Documented from live incident on 2026-05-19 · phase-watchdog mechanism verified working as designed
> **Related cycle:** 89 (research-as-tool Phase C implementation)
> **Companion dossier:** `research-as-tool-implementation-cycles-87-89.md`

## What the phase-watchdog is

A polling activity-based watchdog that detects when an orchestrator subagent goes idle longer than a configurable threshold and:
1. Writes a `stall-progress.json` checkpoint
2. Sends `SIGTERM` to the process group
3. After a grace period, sends `SIGKILL`
4. Preserves the worktree + state for `--resume`

Defaults:
- `EVOLVE_INACTIVITY_THRESHOLD_S=240` (4 min)
- `EVOLVE_INACTIVITY_GRACE_S=10`
- Poll interval: 15s; WARN at 75% (180s)

## What happened in cycle 89

Cycle 89's orchestrator entered `learn` phase after a successful `audit → ship` sequence. Learn invokes retrospective + memo + lesson-merge serially. Combined runtime exceeded 240s without new file activity (last file: `orchestrator-stdout.log`).

Watchdog log:

```
[phase-watchdog] WARN: idle for 180s; stall threshold 240s
[phase-watchdog] FIRE: idle for 241s >= threshold 240s
[phase-watchdog] wrote stall-progress.json
[phase-watchdog] checkpoint stall-inactivity requested
[phase-watchdog] sending SIGTERM to pgid 43252
Terminated: 15
[run-cycle] CHECKPOINT: worktree + state preserved; resume with --resume
[run-cycle] preserved cycle-state at .evolve/cycle-state.json (phase=learn)
[phase-watchdog] sending SIGKILL to pgid 43252 (post-grace)
```

## What went well

| Element | Evidence |
|---|---|
| Stall detected promptly | Fired at 241s, close to 240s threshold |
| Checkpoint written | `stall-progress.json` + cycle-state checkpoint preserved |
| Worktree preserved | Left intact for `--resume` |
| Graceful cleanup | SIGTERM-then-SIGKILL with 10s grace |
| Pipeline integrity | Ship commits already on origin/main; only post-ship cleanup interrupted |

## What this reveals about the threshold

**240s is too tight for `learn` phase.** Learn comprises retrospective (60-90s), memo (fast), lesson-merge (file I/O, slow under load), ACS promotion, and doc cleanup operations. Serialized: 4-6 minutes legitimate.

## Recommended tuning (plan Phase 4)

Per-phase thresholds instead of global:

| Phase | Threshold | Rationale |
|---|---|---|
| calibrate, intent, discover, triage | 180s | Fast read-only setup |
| tdd, build | 300s | Substantial work, frequent file writes |
| audit | 240s (current) | Single subagent |
| ship | 120s | Brief; ship.sh fast |
| **learn / retrospective** | **480s** | 4-6 serialized subagents + mutations |

Implementation: extend `scripts/dispatch/phase-watchdog.sh` with phase-specific env-var lookup (`EVOLVE_INACTIVITY_THRESHOLD_LEARN_S=480`, etc.).

## Non-atomic post-ship cleanup

When watchdog kills mid-cleanup:
- Ship commit on `origin/main` (durable)
- Promotion (mv predicates) half-done in working tree
- Doc deletions uncommitted

Operator must manually `git add -A` + ship a follow-up commit. Today's flow: `215488b` cleanup + `9c6cf19` stewardship recovery.

Improvement paths:
1. **Atomic ship + cleanup** — fold promotion + doc operations into ship.sh's commit (requires ship.sh to know cycle cleanup steps)
2. **Resume-from-promotion** — `--resume` re-runs only cleanup steps when watchdog interrupted mid-cleanup
3. **Pre-ship validation** — verify planned cleanup is in scope before ship fires; commit message accurately covers full diff

## The wider pattern

This parallels the cycle-85 fake-predicate incident: prompt-layer expectation ("orchestrator finishes learn promptly") violated; kernel-layer mechanism (watchdog) caught it; recovery preserved state.

**Phase-watchdog is doing for time what role-gate does for permissions.** Both are kernel layers below the prompt layer catching failures the prompt couldn't enforce. evolve-loop's design rests on this: don't trust the prompt to enforce safety; encode in shell hooks that run regardless of what the agent says.

## Cost of the incident

| Item | Cost |
|---|---|
| Cycle 89 | ~$10 (interrupted) |
| Manual cleanup commits | ~$0.40 |
| Operator investigation time | ~30 min |
| **Direct loss** | minimal — work preserved, ship completed first |

The watchdog prevented a much worse outcome (indefinite hang + lost work).

## References

- Plan: `~/.claude/plans/i-have-question-of-velvet-toast.md` § Phase 5C
- Dispatcher output: `/private/tmp/claude-501/.../bt3dgw9hl.output`
- Recovery commits: `215488b`, `9c6cf19`
- Companion: `research-as-tool-implementation-cycles-87-89.md`
- Implementation: `scripts/dispatch/phase-watchdog.sh`
- Env vars: `EVOLVE_INACTIVITY_THRESHOLD_S`, `EVOLVE_INACTIVITY_GRACE_S`
