# Auto-Resume After Claude Code Quota Hits — Design Research

**Research date:** 2026-05-14
**Researcher:** evolve-loop maintainer (operator-driven design discussion)
**Status:** SHIPPED in v10.6.0 (Layers 1–3); Layer 4 deferred
**Companion runtime doc:** `docs/architecture/auto-resume.md`

---

## Trigger event

Operator ran `/evolve-loop --budget-usd 100 autoresearch run` and observed:

```
⏺ Monitor event: "evolve-loop dispatcher per-cycle events ($100 autoresearch run)"
  ⎿  You've hit your limit · resets 5:20am (Asia/Taipei)
```

Dispatcher exited, worktree + cycle-state were preserved per the v9.1.0
checkpoint mechanism, but **the pipeline never woke back up after 5:20am**.
The operator had to investigate manually.

Three connected questions drove the research:

1. Why can't it recover automatically?
2. Couldn't the orchestrator pick up the signal from the Monitor stream
   and retry after a delay?
3. What should we do to achieve that goal?

## Diagnostic — five concrete gaps

Three parallel Explore agents traced the chain from the user-visible
message down to the underlying primitives. They found that the v9.1.0
checkpoint design was **operator-driven by intent** — `docs/architecture/
checkpoint-resume.md` never mentioned auto-resume — but the primitives
were close to what was needed. The five gaps:

| # | Where | What was missing |
|---|---|---|
| 1 | `scripts/cli_adapters/claude.sh` stderr capture | No filter for `resets HH:MM(am\|pm)` pattern; the rate-limit message bypassed `STDERR_LOG` because the outer Claude Code TUI consumed it at the auth layer. |
| 2 | `scripts/lifecycle/cycle-state.sh:cycle_state_checkpoint` | No `quotaResetAt` / `wakeAt` / `resumeAfter` field in the checkpoint payload schema. Eight existing fields, none temporal. |
| 3 | `scripts/dispatch/evolve-loop-dispatch.sh` main batch loop | `DISPATCH_RC` table had codes 0/1/2/3/4 (success/hard-fail/integrity/recoverable/batch-cap). No "pause until T, then resume" code. RC=5 confirmed free. |
| 4 | Harness scheduling primitives | Zero references to `ScheduleWakeup`, `CronCreate`, `cron`, or `at`. No `auto-resume` daemon. |
| 5 | Monitor event stream | The "Monitor event: ..." line the operator saw was just Claude Code rendering the dispatcher's `log()` output. Nothing subscribed to it to trigger resumption. |

The orchestrator literally could not pick up the Monitor signal because
(a) nothing emitted a structured "quota-paused, wake at T" event and
(b) nothing subscribed to one.

## Anthropic subscription rate-limit semantics

For pricing context (sourced from Anthropic public documentation as of
2026-05):

- **Pro:** 5-hour rolling window with a usage cap. When the window
  exhausts, prompts return a `429`-ish UX with `resets HH:MMam` in the
  user-visible message.
- **Max:** Same 5-hour window structure with a larger cap.
- **API key (`ANTHROPIC_API_KEY`) users:** Different rate-limit model;
  separate Tokens-Per-Minute (TPM) and Requests-Per-Minute (RPM) buckets.
  Auto-resume's reset-time logic targets subscription users; API key
  users are not the primary audience because their rate limits don't
  produce the "resets HH:MMam" surface.

The **5h25min default fallback** (`EVOLVE_QUOTA_RESET_HOURS=5.4167`) is:

- 5h: documented Pro/Max window
- +25min jitter buffer to defend against:
  - clock skew between operator's machine and Anthropic's reset clock
  - the window resetting slightly later than the message claims
  - chain-resume churn (a 4min-late retry burns one of the 3 attempts
    before the window actually reopens)

We considered making the buffer configurable. Operators who want tighter
control can set `EVOLVE_QUOTA_RESET_AT` (explicit ISO 8601) or
`EVOLVE_QUOTA_RESET_HOURS=5.0` for a tighter floor.

## ScheduleWakeup vs cron — decision rationale

The two natural scheduling primitives are:

| Primitive | Surface | Cost | Survives Claude Code close? |
|---|---|---|---|
| Claude Code `ScheduleWakeup` tool | in-session, model-driven | ~$0.01–0.05 per wake check (model observes and returns to sleep) | No |
| Unix `cron` (or systemd timer) | OS-level, daemon | Negligible | Yes |

The operator decision was **ScheduleWakeup-only for v10.6.0** because:

- The dominant use case is "operator queues a long unattended run before
  leaving the terminal open" — Claude Code stays alive.
- `cron` introduces a long-lived process surface (running as the user,
  touching the repo, no kernel-level isolation) requiring a careful
  security review.
- A cron daemon must own conflict resolution for concurrent
  invocations (operator manually running `--resume` while cron also
  fires). ScheduleWakeup is single-tenant by construction.
- If Claude Code is closed mid-wait, the worktree + cycle-state are
  preserved on disk; the user can manually `--resume` later without
  losing work. The fallback is graceful.

Layer 4 (cron daemon) was scoped as a future additive feature. The
v10.6.0 Layer 1–3 design exposes all the primitives a daemon would need:

- `quotaResetAt` field to poll against
- `bump-auto-resume-attempts` for atomic retry-counting
- `resume-cycle.sh` as the standard entry point

## Default-on, no-opt-out — operator directive

Initial recommendation was opt-in (`--auto-resume` flag, default OFF) on
the grounds that "auto-resume surprises operator who wanted to stop." The
operator overruled with:

> the default for auto resume should be on without flag

Rationale captured in memory `feedback_rate_limit_recovery.md`: the
recurring friction of *not* auto-resuming (operator wakes up to a halted
pipeline, manually investigates, manually re-invokes) outweighs the
surprise risk. Risk is mitigated by:

1. The loud `QUOTA-PAUSE: wake-at=ISO` log line emitted before any
   `ScheduleWakeup` call. Operator reads the timestamp and decides
   whether to Ctrl+C the wait.
2. `autoResumeMaxAttempts=3` cap. Three consecutive quota hits without
   progress exhausts the budget and leaves the marker for operator
   intervention.

This matches the design ethos in `feedback_architecture_first_design.md`
— "scope-bounds processes per-phase/per-request, separates insight from
action, prefers unified message envelopes + system-wide severity
vocabularies." The `QUOTA-PAUSE: cycle=N wake-at=T source=X attempts=K/M`
marker is one such envelope.

## Why preserve `autoResumeAttempts` across re-checkpoints

Initial schema design wrote `autoResumeAttempts: 0` on every fresh
checkpoint write. This was wrong for the cap-the-loop use case:

```
attempt 1 → quota hit → checkpoint A written, attempts=0
resume → bump to 1
attempt 2 → quota hit → checkpoint B overwritten, attempts=0 ← LOOP NEVER CAPS
resume → bump to 1
...
```

The fix (`autoResumeAttempts: (.checkpoint.autoResumeAttempts // 0)`) reads
the prior value and preserves it. This way the cap correctly accumulates
across the failure chain. When the orchestrator clears the checkpoint on
successful cycle completion (existing v9.1.0 path), the next quota hit
writes a fresh checkpoint with `// 0` → fresh budget.

The `reset-auto-resume-attempts` call from `resume-cycle.sh` on rc=0
exit is the "full success" signal: the cycle completed normally, so a
future quota hit (on a future cycle) gets the full retry budget.

## Bump semantics — `< max` not `<= max`

The bump function returns rc=2 (exhausted) when `pre-increment count >=
max`, not when `post-increment count >= max`. This means:

- `max=3, attempts=0 → bump → attempts=1, rc=0`
- `max=3, attempts=1 → bump → attempts=2, rc=0`
- `max=3, attempts=2 → bump → attempts=3, rc=0`
- `max=3, attempts=3 → bump → no change, rc=2` ← exhausted

The semantic is "max=N means allow N retries total." First wrong
implementation returned rc=2 when post-increment hit max, which gave
only N-1 retries. Fixed during testing (Test 8 of `auto-resume-test.sh`
exercises the boundary).

## Stderr capture — the "5 invocation sites" problem

`scripts/cli_adapters/claude.sh` invokes `claude -p` from **5 separate
sites** in v10.5.0 (one per sandbox mode: macOS sandbox-exec, Linux
bwrap, no-sandbox, etc.). The naive approach — adding a stderr-tee to
each invocation — would have been 5x maintenance burden and risked
drift.

Better approach: extend the existing post-invocation diagnostic block at
lines 604+ which runs after every invocation regardless of sandbox mode.
A single new `grep -ioE 'resets +[0-9]{1,2}:[0-9]{2} *(am|pm)'` against
`$STDERR_LOG` covers all five paths.

The hint capture is best-effort: in practice the message rarely reaches
the nested subprocess stderr (consumed by outer Claude Code at the auth
layer). The fallback (default 5h25m) handles the common case; the
capture is defense-in-depth.

## Departures from the original plan

The plan (`/Users/danleemh/.claude/plans/investigate-when-the-pipeline-
jazzy-planet.md`) was iterated twice:

1. **Initial proposal:** `--auto-resume` flag, default OFF, opt-in.
   **Operator override:** always-on, no flag.

2. **Initial Layer 1 strategy:** add stderr-tee to all 5 `claude -p`
   invocation sites. **Better strategy after Explore:** extend the
   existing diagnostic block at line 604+; one filter covers all sites.

3. **Initial bump semantics:** `[ "$attempts" -lt "$max" ]` after
   increment (rejected one retry shy of the cap). **Fix:** check
   pre-increment against max; allow exactly N retries.

## References

- `docs/architecture/auto-resume.md` — runtime architectural contract
- `docs/architecture/checkpoint-resume.md` — v9.1.0 baseline
- Anthropic documentation (subscription rate limits, 2026)
- `feedback_rate_limit_recovery.md` — operator preference for cron/loop
  on rate limits
- `feedback_architecture_first_design.md` — unified envelope ethos
- `feedback_offline_autonomy.md` — autonomy execution principles
- evolve-loop plan file (this cycle's work):
  `~/.claude/plans/investigate-when-the-pipeline-jazzy-planet.md`

## Future work

- **Layer 4 cron daemon** (deferred): `scripts/scheduler/auto-resume-
  daemon.sh` + `install-auto-resume-cron.sh` for terminal-closed
  scenarios. Primitives are in place; only needs the daemon shell.
- **Anthropic API rate-limit header parsing** (if/when subscription
  users get API access to their own quota state): query rate-limit
  reset directly via API headers instead of parsing the
  user-visible message. Would tighten the ETA from 5h25min ± jitter
  to exact.
- **Multi-cycle wake batching**: if the dispatcher pauses mid-batch
  with N more cycles queued, the wake could resume cycle K and
  continue cycles K+1..N automatically. Today the dispatcher exits
  after one cycle's resume; further batch continuation requires
  another `--resume` invocation. Low priority — current behavior
  matches the v9.1.0 single-cycle resume contract.
