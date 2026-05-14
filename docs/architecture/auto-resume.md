# Auto-Resume After Quota Hits (v10.6.0+)

> **Status:** ships with v10.6.0. Operator-confirmed scope: Layers 1–3
> (in-session via `ScheduleWakeup`). Layer 4 (cron-driven fallback for
> terminal-closed scenarios) deliberately deferred.

## Why this exists

Pre-v10.6.0, hitting a Claude Code subscription rate-limit (`You've hit your
limit · resets HH:MMam`) during an `/evolve-loop $N autoresearch run` left
the pipeline halted indefinitely. The v9.1.0 checkpoint mechanism correctly
preserved the worktree + `cycle-state.json`, but **nothing scheduled a wake-up
to call `/evolve-loop --resume` after the quota window reset**. The operator
had to notice manually, wait, and re-invoke.

Five concrete gaps drove the halt:

1. The reset time (`resets 5:20am`) was printed by the *outer* Claude Code
   TUI and never reached the nested subprocess stderr.
2. The checkpoint schema had no temporal field (`quotaResetAt`).
3. The dispatcher had no `DISPATCH_RC` code meaning "pause until T, then
   resume" — quota-likely fell through to a hard-fail or integrity-breach.
4. No scheduling primitives (`ScheduleWakeup`, cron) wired into the harness.
5. The "Monitor event" surface was decorative — nothing subscribed to it.

v10.6.0 closes all five with a three-layer design.

## The three layers

```
  Quota hit
    │
    ▼
  Layer 1: capture + estimate
    ├─ claude.sh stderr filter scrapes "resets HH:MMam" → quota-reset-hint.txt
    └─ estimate-quota-reset.sh consumes hint OR falls back to now + 5h25m
    │
    ▼
  Layer 2: persist in checkpoint
    └─ cycle_state_checkpoint() schema extended with 4 new fields:
       quotaResetAt, quotaResetSource, autoResumeAttempts, autoResumeMaxAttempts
    │
    ▼
  Layer 3: dispatcher emits QUOTA-PAUSE marker + DISPATCH_RC=5
    └─ SKILL.md handles DISPATCH_RC=5 → calls ScheduleWakeup until wake-at →
       re-invokes /evolve-loop --resume → resume-cycle.sh bumps attempts,
       runs the paused cycle from its last clean phase boundary
```

### Layer 1 — Reset-time capture and estimation

| Component | File | Purpose |
|---|---|---|
| stderr scraper | `scripts/cli_adapters/claude.sh` (~line 628+) | After every `claude -p` invocation, `grep -ioE 'resets +[0-9]{1,2}:[0-9]{2} *(am\|pm)'` the stderr log. On match, atomically write the captured time to `<workspace>/quota-reset-hint.txt`. |
| ETA helper | `scripts/dispatch/estimate-quota-reset.sh` | Pure compute. Reads operator override → hint file → fallback (in that order). Output: ISO 8601 timestamp + `source=<X>`. |

Source priority:

1. `$EVOLVE_QUOTA_RESET_AT` env var (operator override, ISO 8601 string)
2. Parsed hint at `<workspace>/quota-reset-hint.txt` (HH:MM am/pm format)
3. Fallback `now + EVOLVE_QUOTA_RESET_HOURS` (default `5.4167` ≈ 5h25min)

The hint-file path is rarely populated in practice because the nested
`claude -p` subprocess typically dies with empty stderr (the outer Claude
Code consumes the rate-limit response at the auth layer). The fallback
is therefore the *common case*; the hint capture is defense-in-depth for
direct-CLI invocations and future SDK changes that might surface the
message through the subprocess stderr.

### Layer 2 — Checkpoint schema extension

`scripts/lifecycle/cycle-state.sh:cycle_state_checkpoint()` writes 4
additional fields beyond the v9.1.0 baseline:

| Field | Type | Source | Notes |
|---|---|---|---|
| `quotaResetAt` | ISO 8601 string | `$EVOLVE_CHECKPOINT_QUOTA_RESET_AT` env var | Empty string when env unset (e.g. non-quota checkpoint). |
| `quotaResetSource` | `"operator-override" \| "parsed" \| "default"` | `$EVOLVE_CHECKPOINT_QUOTA_RESET_SOURCE` env var | Trail for forensics. |
| `autoResumeAttempts` | integer | preserved across re-checkpoints (carries old value); 0 on fresh write | Bumped by `resume-cycle.sh`; reset on full-cycle success. |
| `autoResumeMaxAttempts` | integer | `$EVOLVE_AUTO_RESUME_MAX_ATTEMPTS` env, default `3` | Cap on consecutive auto-resume invocations. |

Two new helpers operate the counter atomically:

- `cycle-state.sh bump-auto-resume-attempts` — increment by 1. Returns 0 if
  post-increment count `< max`; returns 2 (exhausted, no state mutation) if
  pre-increment count `>= max`.
- `cycle-state.sh reset-auto-resume-attempts` — set to 0.

`subagent-run.sh` calls `estimate-quota-reset.sh` and exports the two env
vars before invoking `cycle-state.sh checkpoint quota-likely`, so the
schema fields are populated automatically when the `_quota_likely`
heuristic fires.

### Layer 3 — Dispatcher signal + SKILL handler

`evolve-loop-dispatch.sh` detects a fresh `quota-likely` checkpoint as
the first check in its non-zero-rc branch, emits a structured marker
line, and exits with `DISPATCH_RC=5`:

```
QUOTA-PAUSE: cycle=N wake-at=2026-05-15T05:20:00+0800 source=parsed attempts=0/3
```

`DISPATCH_RC=5` is the v10.6.0 addition to the dispatcher exit-code table
(0/1/2/3/4 were in use; 5 was free).

`.agents/skills/evolve-loop/SKILL.md` carries a "Quota Handling &
Auto-Resume" section that instructs the model: when `DISPATCH_RC=5` appears
in dispatcher output, parse the `wake-at=ISO8601` value, compute a clamped
`delaySeconds`, call `ScheduleWakeup` with `prompt="/evolve-loop --resume"`,
and chain wake-ups until the reset time. The clamp is `[60, 3600]` to
match `ScheduleWakeup`'s own range; for waits longer than 3600s the SKILL
schedules a fresh wake on each cycle.

`resume-cycle.sh` is the post-wake entry point. It now:

1. Bumps `autoResumeAttempts` on entry. If `bump` returns rc=2, refuses
   the resume and exits rc=2 (the marker stays for operator intervention).
2. Runs the existing HEAD-drift, worktree-exists, and cycle-state
   validations.
3. Spawns `run-cycle.sh` in resume mode (unchanged from v9.1.0).
4. On rc=0 success, calls `reset-auto-resume-attempts` so a future quota
   hit on a future cycle gets a fresh retry budget.

## Env var reference

| Variable | Default | Layer | Effect |
|---|---|---|---|
| `EVOLVE_QUOTA_RESET_AT` | unset | 1 | ISO 8601 string. Bypasses hint parsing and fallback; used verbatim. |
| `EVOLVE_QUOTA_RESET_HOURS` | `5.4167` | 1 | Float hours added to `now` when no hint/override available. |
| `EVOLVE_CHECKPOINT_QUOTA_RESET_AT` | populated by `_quota_likely` path | 2 | Internal: written by `subagent-run.sh` from estimate output. |
| `EVOLVE_CHECKPOINT_QUOTA_RESET_SOURCE` | populated by `_quota_likely` path | 2 | Internal: written by `subagent-run.sh`. |
| `EVOLVE_AUTO_RESUME_MAX_ATTEMPTS` | `3` | 2 | Cap on consecutive auto-resume invocations per paused cycle. |
| `EVOLVE_RESUME_ALLOW_HEAD_MOVED` | `0` | n/a (v9.1.0) | Existing escape hatch; unchanged. Auto-resume never sets this. |

## Operator workflow

Default behavior (always-on auto-resume):

```bash
/evolve-loop --budget-usd 100 autoresearch run
# ... cycles run normally ...
# ... quota hit at, say, midnight ...
# QUOTA-PAUSE: cycle=27 wake-at=2026-05-15T05:20:00+0800 source=parsed attempts=0/3
# SKILL.md calls ScheduleWakeup(delay=18900s, prompt="/evolve-loop --resume")
# ... 5h15m of sleep ...
# Claude wakes, fires /evolve-loop --resume
# resume-cycle.sh bumps attempts to 1, runs cycle 27 from its paused phase
# Cycle 27 completes; dispatcher continues with cycle 28
```

### Aborting an in-flight auto-resume

The `QUOTA-PAUSE: wake-at=ISO` log line is the intervention surface.
Operator reads the timestamp, decides whether they want to wait or
abort, and Ctrl+Cs the Claude Code session before the wake fires. There
is no `--no-auto-resume` flag; the loud log is the entire opt-out.

### Recovering from cap exhaustion

If the same cycle hits quota `autoResumeMaxAttempts` times in a row,
`resume-cycle.sh` exits rc=2 and leaves the checkpoint marker intact.
Recovery:

```bash
# Inspect what's going on
cat .evolve/cycle-state.json | jq .checkpoint

# Option A: increment the cap and try again
jq '.checkpoint.autoResumeMaxAttempts = 10' .evolve/cycle-state.json \
   > .evolve/cycle-state.json.tmp && mv .evolve/cycle-state.json.tmp .evolve/cycle-state.json
bash scripts/dispatch/resume-cycle.sh

# Option B: abandon the cycle (worktree is preserved; you can salvage edits manually)
bash scripts/lifecycle/cycle-state.sh clear-checkpoint
```

## Out of scope (deliberate)

### Layer 4 — cron daemon for terminal-closed scenarios

We considered a `scripts/scheduler/auto-resume-daemon.sh` that would poll
`.evolve/cycle-state.json` from cron every 5 minutes and invoke
`resume-cycle.sh` when `now >= quotaResetAt`. This would survive the user
closing Claude Code entirely. The operator decision was to defer it
because:

- It introduces a long-lived process surface (cron job running as the
  user, touching the repo) which requires careful security review.
- The in-session ScheduleWakeup path covers the dominant use case (an
  operator queueing a long run before leaving the terminal open).
- If Claude Code is closed mid-wait, the worktree + cycle-state are
  preserved; the user can manually `bash scripts/dispatch/resume-cycle.sh`
  on the next session without losing work.

Layer 4 can ship later as a purely-additive feature; the Layer 1–3 design
exposes the right primitives (`bump-auto-resume-attempts`, the
`quotaResetAt` field, the `resume-cycle.sh` entry point) for a daemon to
hook into.

### Reset-time parsing beyond stderr

We do not try to introspect Anthropic's API for rate-limit reset
headers, nor parse the outer Claude Code TUI. Both would require either
an `ANTHROPIC_API_KEY` (most evolve-loop users authenticate via
subscription) or running outside the harness. The default 5h25min
fallback is calibrated against Anthropic's published Pro/Max 5-hour
rate-limit window with a 25min jitter buffer.

## Failure modes & guardrails

| Risk | Mitigation |
|---|---|
| Infinite quota-resume-quota loop | `autoResumeMaxAttempts` cap (default 3). The counter is preserved across re-checkpoints so the cap accumulates across the failure chain. |
| Operator wants to stop, didn't realize auto-resume was on | Loud `QUOTA-PAUSE: wake-at=ISO` log emitted before `ScheduleWakeup` fires. Operator interrupts the Claude Code session before the wake. |
| HEAD drifted while paused (e.g., hot-fix committed on main) | Existing v9.1.0 guard at `resume-cycle.sh:85-100` refuses to resume unless `EVOLVE_RESUME_ALLOW_HEAD_MOVED=1`. Auto-resume never sets that env var. |
| Cost spiral | `EVOLVE_BATCH_BUDGET_CAP` (default $20) is enforced by the dispatcher regardless of auto-resume. Cumulative spend never exceeds it. |
| Anthropic changes the "resets HH:MMam" message format | Falls back to default `now + EVOLVE_QUOTA_RESET_HOURS`. `EVOLVE_QUOTA_RESET_AT` lets the operator hard-code the right time. |
| `ScheduleWakeup` tool not loaded in the session | SKILL.md fallback: log the `QUOTA-PAUSE wake-at=ISO` marker verbatim and require manual `bash scripts/dispatch/resume-cycle.sh`. Never silently swallow `DISPATCH_RC=5`. |
| User closes Claude Code mid-wait | Cycle stays preserved on disk. Manual `--resume` from a future session picks it up. (Layer 4 would close this gap but is out of scope.) |

## Verification

```bash
# Unit tests
bash scripts/tests/auto-resume-test.sh         # 26 assertions, Layers 1+2+3
bash scripts/tests/checkpoint-roundtrip-test.sh # 19 assertions, no regression

# Manual end-to-end (requires triggering a quota hit)
EVOLVE_QUOTA_RESET_HOURS=0.05 /evolve-loop --budget-usd 5 "smoke test"
#   - run a short cycle that hits the cost ceiling
#   - observe QUOTA-PAUSE marker
#   - observe ScheduleWakeup invocation in transcript
#   - wait ~3min, observe auto-resume firing
```

## See also

- `scripts/dispatch/estimate-quota-reset.sh` — Layer 1 ETA helper
- `scripts/cli_adapters/claude.sh:628+` — Layer 1 stderr scraper
- `scripts/lifecycle/cycle-state.sh:cycle_state_checkpoint` — Layer 2 schema
- `scripts/dispatch/evolve-loop-dispatch.sh` — Layer 3 DISPATCH_RC=5 path
- `.agents/skills/evolve-loop/SKILL.md` — Layer 3 SKILL handler
- `docs/architecture/checkpoint-resume.md` — v9.1.0 baseline this builds on
- `knowledge-base/research/auto-resume-design.md` — research dossier
