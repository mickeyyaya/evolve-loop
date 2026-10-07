# 2026-10-06 — agy's first launch after an idle period misses the 60 s REPL budget, and the next launch boots

**Class:** a false halt or a false skip at wave start. Seen twice.

**Surface:**
- the boundary CLI updater's `doctor live` probe (`cmd/evolve` `liveProbeWith`, `internal/cliupdate`);
- the readiness gate's `bridge-boot` check (`internal/looppreflight`);
- the cli-health canary, which shares the live probe.

**Inbox:** `agy-cold-start-boot-exceeds-the-60s-repl-budget` (filed from the console at the wave 67 boundary).

## What happened

| When | First agy launch of the wave | Next agy launch |
|---|---|---|
| Wave 62 | preflight's `bridge-boot` booted `agy-tmux`; the REPL never appeared (rc 80) and the loop halted | `evolve doctor live agy-tmux`, about a minute later, passed |
| Wave 67 boundary | the boundary updater's `doctor live agy-tmux`: `FAIL: REPL prompt never appeared after 60s` (rc 80). The updater logged `agy skipped 1.3.0: doctor live did not answer, so the family counts as unsubscribed` | preflight's `agy-claude-tmux` boot, about 17 s after the probe's session was killed and 80 s after it started, then its `agy-tmux` boot: both drew `? for shortcuts` and booted |

Evidence: `runtime/.evolve/loop-20261006-wave67.log`, lines 5–40.

In both cases the first agy launch after an idle period paid a cold-start cost that took longer than the fixed 60 s boot wait, and a launch seconds later booted in a few seconds. The cost is probably an auth refresh or the model list fetch; it was not measured.

## Root cause

- **One boot budget for every launch.** `bootTmuxREPL` waits `tmuxREPLBootTimeoutS` (60 s) for the prompt marker, whether agy is cold or warm.
- **No retry where it matters.** The probes that run first in a wave (the updater's `doctor live`, then preflight's `bridge-boot`) launched once. Phase dispatch has a fallback chain and a boot-strike bench; these probes had neither.
- **A boot timeout read as "unsubscribed".** The updater's probe seam returned a bare `doctor live agy-tmux rc=80 pattern=""` error, and `cliupdate` treated every probe error as a missing subscription. The operator was told the wrong thing, and the pane that showed what agy was doing was dropped.

## The fix

**Option chosen: one retry of a bare REPL-boot timeout in the probes, counted by the family manifest.**

- `probe_boot_retries` is a manifest field. agy-tmux sets 1; agy-claude-tmux inherits it through `base`; every other driver has 0.
- `bridge.BootProbe.Retry` runs a probe launch once per allowed attempt:
  - It retries only exit 80 with no classified wall. A login or quota wall at boot is not a cold start.
  - It never retries after the context ends.
  - It logs each step: `cold start: boot attempt 1 of 2 timed out …; retrying`, then `… got past the REPL boot`. A last timeout logs `FAIL: the REPL never drew its prompt on any of 2 boot attempts`.
- `liveProbeWith` (the updater's probe, its smoke and the canary) and `looppreflight.bootOne` call it, and each attempt gets its own full budget. Phase dispatch is unchanged.
- Both callers hand the retry the attempt's classified wall: the live probe returns its escalation pattern, and preflight's boot tester returns `BootOutcome.Wall`, both read through `bridge.EscalationPattern`. The first cut of preflight returned no wall, so it retried any exit 80, a login wall included; review round 1 caught it (`TestRun_BridgeBoot_ABootTimeoutCarryingAWallIsNotRetriedAndNamesTheWall`).
- The updater reports a bare boot timeout as status `boot-timeout`. The probe error wraps `cliupdate.ErrBootTimeout` and carries the attempt count and the final pane's last lines. The detail says the version was neither checked nor updated and that a boot timeout says nothing about the subscription. It is a WARN line, as `skipped` is. Preflight's `bridge-boot`, which runs next, still halts if agy cannot boot.

**Why a retry and not a longer first-launch budget** (the inbox's option b):

- **The evidence fits a retry.** In both incidents the next launch, seconds after the first was killed, booted at once. Whatever the cold start pays for survives the killed session.
- **It holds no state.** A first-launch-of-the-run allowance needs a process-wide record of which binaries have launched. That is shared mutable state in the bridge's hot boot path, and every bridge test would share it.
- **It stays in the probes.** The retry lives in the probe entry points. A larger budget would also change phase dispatch, which already has its own recovery.
- **The deadlines already fit.** Each attempt keeps the deadline it had: 90 s per preflight boot, 4 minutes per live probe. No caller's bound had to grow.

**When to switch to option b.** Do so if a retry ever times out where a longer single wait would have booted. That would mean the cold-start work restarts with each killed session. The log shows it as two `cold start` lines followed by `FAIL … any of 2 boot attempts`, where a later launch boots.

## Proof

Each test failed before the change and passes after it.

| Test | Package | What it pins |
|---|---|---|
| `TestBootProbeRetry_AnAgyThatDrawsItsREPLLateOnItsFirstLaunchOnlyBootsOnTheRetry` | bridge | a fake pane that never draws on agy's first session and draws at once on the next; `BootSmokeTest` boots on attempt 2, and the log names the cold start |
| `TestBootProbeRetry_AnAgyThatNeverDrawsItsREPLFailsWithThePaneCaptured` | bridge | two attempts, rc 80, the pane captured, a loud `FAIL` |
| `TestBootProbeRetry_OnlyABareBootTimeoutIsRetried`, `…ACancelledContextIsNotRetried`, `…ADriverWhoseManifestDeclaresNoRetryLaunchesOnce`, `TestProbeBootAttempts_ComeFromTheFamilyManifest`, `TestParseManifest_RefusesANegativeProbeBootRetryCount` | bridge | what is retried, and that the count is manifest data |
| `TestUpdate_ADoctorLiveBootTimeoutIsReportedAsABootTimeoutNotAsUnsubscribed` | cliupdate | red output was the exact wave 67 wording: `status = "skipped" … counts as unsubscribed` |
| `TestLiveCheck_ABareREPLBootTimeoutIsABootTimeoutWithItsPaneAndAWallIsNot`, `TestLiveProbeWith_RetriesAnAgyBootTimeoutOnceBeforeConcluding`, `TestBoundaryCLIUpdate_ABootTimeoutOrAVerifiedQuotaCauseIsAWarningLine` | cmd/evolve | the probe error, the retry in the shared live probe, the WARN line |
| `TestRun_BridgeBoot_AnAgyColdStartThatBootsOnTheRetryPasses`, `TestRun_BridgeBoot_AnAgyThatNeverBootsHaltsAfterBothAttemptsWithThePane` | looppreflight | the readiness gate passes on the retry with a full budget for each attempt, and halts after both attempts with the pane |

**Not proven:** the real agy binary was not launched for this change; the tests use fake panes. The next wave boundary is the live check. Grep the loop log for `cold start:`.
