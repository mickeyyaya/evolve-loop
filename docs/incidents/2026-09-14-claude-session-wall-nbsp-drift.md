# 2026-09-14 — Claude's session wall went unrecognised because tmux indents it with U+00A0

**Class:** pipeline (a quota wall the bridge is built to fast-fail on ran the full artifact window instead).
**Surface:** `internal/bridge/manifests/claude-tmux.json` `controls.usage.exhausted_regex` (the guarded-exhaustion classifier's vocabulary).
**Found by:** wave 2 of the pipeline-health verification — both lanes' recovery audits stalled 40 minutes per dispatch; the bridge stderr carried `POSSIBLE EXHAUSTION-REGEX DRIFT`.

## What happened

Between 19:03 and 19:50 the account's rolling Claude session limit was reached. Every Claude
dispatch in that window (the recovery audits of lanes 1676 and 1677, on `claude-tmux@deep`)
printed

```
  ⎿  You've hit your session limit · resets 7:50pm (Asia/Taipei)
     /usage-credits to finish what you're working on.
✻ Cogitated for 0s · done 7:03 PM
```

and parked at the prompt. The bridge has a guarded fast-fail for exactly this: the usage
classifier (`ClassifyExhausted`) matches the wall, the persistence gate requires it on two
consecutive observations, a live probe on the cheapest tier corroborates it, and the
launch exits 85 so the runner fails over at once. None of it armed. The stop-reviewer saw
only "no output during the last 1200s interval", extended once, and after 2400 s the
launch exited 81 (`BRIDGE_EXIT_ARTIFACT_TIMEOUT`, `RUNNER_TEARDOWN_FAIL`). The runner
then re-dispatched the audit on the same CLI — into the same wall — until the limit reset.
At teardown the bridge printed its own diagnosis:

```
[claude-tmux] POSSIBLE EXHAUSTION-REGEX DRIFT: the teardown pane matches a broad
quota-wall heuristic but claude-tmux's controls.usage.exhausted_regex did not
```

## Root cause

tmux renders Claude Code's `⎿` indentation with a NO-BREAK SPACE: the captured line is
`⎿  You've hit your session limit · resets …`. The session-wall branch of
`exhausted_regex` is anchored `^[ \t]*(?:⎿[ \t]*)?you(?:[’']ve| have) (?:hit|reached) your
session limit[ \t]*[·—][ \t]*resets`, and `[ \t]` does not admit U+00A0. The unit test for
that branch (`TestClaudeSessionWallUsesGuardedExhaustion`) was written from a 2026-08
capture typed with plain spaces, so it stayed green while the live capture never matched.
The `drift_probe_regex` (the broad heuristic) has no such anchor, which is why the
diagnostic fired — and why it is diagnostic only: the probe is deliberately too broad to
drive a fail-over.

The same mistake is the memory rule from 2026-07-18 (`audit_quota_wording_drift`):
validate regexes against the REAL pane bytes, never a retyped fixture.

## Fix (this change)

Every `[ \t]` in claude's `exhausted_regex` becomes `[\t\p{Zs}]` (Go RE2: the Unicode
space-separator class, which holds U+0020 and U+00A0). `codex-tmux` and `agy-tmux` carry
no such anchors and are unchanged. Red first:
`TestClaudeSessionWall_NBSPIndentIsRecognised` feeds `ClassifyExhausted` the live line
with the U+00A0 spelled out (` `), in both apostrophe forms; it failed before the
change. The guarded path (persistence + live probe) is untouched — the earlier attempt to
widen the *prompt* `rate_limit` rule instead was reverted because it bypassed that guard,
exactly what `TestClaudeSessionWallUsesGuardedExhaustion` forbids.

## Cost on this wave

Two lanes × two recovery audits × 40 minutes of wall-clock, plus the re-dispatches; no
tokens were spent while walled (the model never answered). The limit itself reset at
19:50; the loop lost the whole window and both lanes then failed their ships on the
separate gate-environment leak.

## Operator notes

- `POSSIBLE EXHAUSTION-REGEX DRIFT` in a phase's launch-error file means the wording
  drifted again: copy the bytes from the escalation report's `final_pane` (never retype
  them), put them in the test, then fix the regex.
- A phase that "takes" 40 minutes with `Worked for 0s` in the pane is a wall, not a slow
  model; the per-phase duration on the dashboard is the fastest tell.
