# Fast-Fail Counter Scope

> Architecture doc for cycle-94 P1 retry fast-fail (O-1). Documents the per-agent vs per-workspace counter semantics added to `legacy/scripts/dispatch/subagent-run.sh` and `agents/evolve-orchestrator.md` in commit `d24b403` (v10.17.0).

## Purpose

Prevent the cycle pipeline from burning cost retrying structurally-dead phase-agent launches. Before P1, the orchestrator could spawn the same phase-agent up to 3 times even when each launch exited at duration_s < 5 (sandbox-EPERM, nested-Claude restriction, profile schema violation, etc.). Cycle 93's retrospective burned ~$1.20 retrying a dead launch 3 times. After P1, two consecutive sub-5-second exits abort the cycle with a structured ledger entry.

## Mechanics

`legacy/scripts/dispatch/subagent-run.sh` tracks per-agent retry counts in `state.json:retryCounters[<role>]`. The schema:

```json
{
  "retryCounters": {
    "tdd-engineer": 0,
    "builder": 0,
    "auditor": 0,
    "retrospective": 0,
    "memo": 0
  }
}
```

At each `subagent-run.sh` invocation:

1. Read `state.json:retryCounters[<role>]` (default 0 if absent).
2. After the phase-agent completes (or is killed), measure `duration_s = end_time - start_time`.
3. If `duration_s < 5`:
   - If `retryCounters[<role>] >= 1` (prior sub-5s exit already counted): emit `kind:retry_exhausted_fastfail` ledger entry, exit with non-retry rc (currently 2).
   - Else: increment `retryCounters[<role>]` to 1.
4. If `duration_s >= 5`:
   - Reset `retryCounters[<role>]` to 0.

The counter is **per-agent**, not per-workspace. Each role (tdd-engineer, builder, auditor, etc.) gets its own counter. A 0-second exit from tdd-engineer doesn't increment the counter for builder.

## Why per-agent, not per-workspace?

Two structural reasons:

1. **Different failure modes per role.** Sandbox-EPERM affects tdd-engineer (writes test files) differently than auditor (mostly reads). A per-workspace counter would conflate unrelated failures.
2. **Recovery semantics.** When one agent fails, retrying it makes sense; retrying ALL agents (per-workspace counter) doesn't.

Per memory `feedback_parallelization_discipline.md`: "Parallel only for read-only/summarizing tasks; writes are sequential single-writer (codified as `parallel_eligible: bool` in agent profiles)." The counter follows the same single-writer logic — each writing agent has its own counter.

## Operator-facing flags

| Env var | Default | Effect |
|---|---|---|
| `EVOLVE_FAST_FAIL_THRESHOLD_S` | 5 | Duration in seconds below which an exit counts as "fast" (structural failure). Lower to be stricter; raise to be more tolerant. |
| `EVOLVE_FAST_FAIL_MAX_COUNT` | 2 | Number of consecutive fast exits required to trigger fastfail abort. Default 2 = abort on second occurrence. |
| `EVOLVE_FAST_FAIL_DISABLE` | 0 | Set to 1 to disable counter entirely (legacy behavior — up to 3 retries regardless of duration). For debugging only. |

## Orchestrator behavior on fast-fail abort

When `subagent-run.sh` emits `retry_exhausted_fastfail`:

1. Ledger entry is written with `kind:retry_exhausted_fastfail`, `severity:HIGH`, `agent:<role>`, `cycle:<N>`, `details:"2 consecutive sub-5s exits for <role>"`.
2. Subagent-run exits with rc=2 (distinguishes from rc=0 success, rc=1 normal failure).
3. The orchestrator persona (`agents/evolve-orchestrator.md`) reads ledger for `kind:retry_exhausted_fastfail` after each phase invocation. On match, STOP CRITERION fires: cycle aborts with FAIL status, no further phases attempted.
4. Cycle-state.json is checkpointed with `checkpoint.reason: retry_exhausted_fastfail` so `--resume` can be invoked but only by operator (not auto-resume).

## Worked example: cycle-93 retrospective burn

Pre-P1 (cycle 93 ledger):

```
retrospective: duration_s=214, rc=0       # but verdict was incomplete
retrospective: duration_s=341, rc=0       # retry attempt 1 - completed
retrospective: duration_s=471, rc=0       # retry attempt 2 - completed
```

Total: 3 attempts, 1026s, ~$1.20 of cost. The orchestrator retried because the first attempt's report was incomplete, not because it failed-fast.

Post-P1 (hypothetical replay of cycle-93 with P1 active):

The cycle-93 case wouldn't trigger fast-fail (durations were 214/341/471s, all > 5s). The counter only catches structural failures (sandbox-EPERM, missing binary). For the incomplete-report scenario, a different mechanism (Builder turn-budget guidance, shipped in cycle 96) addresses it.

**The fast-fail counter is specifically for structural failures**, not slow/incomplete agents. For slow agents, see [`docs/architecture/watchdog-stall-detection.md`](watchdog-stall-detection.md).

## References

- Shipped in commit `d24b403` (cycle-94, v10.17.0)
- Implementation: `legacy/scripts/dispatch/subagent-run.sh` lines 363-425 (write_ledger_entry callsite + counter update)
- Persona update: `agents/evolve-orchestrator.md` (STOP CRITERION extension paragraph)
- ACS predicates: `acs/regression-suite/cycle-94/002-fast-fail-counter-logic.sh`, `acs/regression-suite/cycle-94/003-orchestrator-fast-fail-stop-criterion.sh`
- Lesson YAML: `.evolve/instincts/lessons/cycle-94-retry-fast-fail-pattern.yaml`
- Research dossier (cycle-93 origin): `knowledge-base/research/cycle-93-trust-kernel-breach-2026-05-20.md`
