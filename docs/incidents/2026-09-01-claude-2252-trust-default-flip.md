# 2026-09-01 — claude 2.1.252 flips the folder-trust default to "No, exit"

## Impact
Wave-20260901b: all three fleet lanes (cycles 1598/1599/1600) teardown-FAILed
in triage with artifact-timeout (exit 81), zero extends, `submit_wedged`
resends=3. 0 ships. Codex-driven scout phases completed normally; every
claude phase launched in a fresh cycle worktree died identically. Loop halted
by operator per ADR-0072 (three parallel same-fingerprint lane fails =
SYSTEM) before wave 2 dispatched; the three inbox items were auto-released by
the inbox-mover (failures were pre-build — no work lost).

## Root cause
claude 2.1.252 changed the boot-time folder-trust dialog in two ways at once:
the options lost their numbering, and the pre-highlighted default flipped
from "Yes, I trust this folder" to **"❯ No, exit"**. The v2.1.193-era
auto-respond rule (`trust_prompt`) therefore missed twice: its regex anchors
on the numbered `1. Yes, I trust this folder` line (no longer rendered), and
its response — a bare Enter for the then-Yes-default — now confirms
"No, exit", killing the REPL. The dialog's ❯ cursor also satisfies the REPL
prompt marker, so the boot loop declared ready and pasted the phase prompt
into the modal (the known v2.1.193 collision, unhandled for the new pane).
Preflight had WARNed the version drift (2.1.251 → 2.1.252) at batch boot.

## Evidence
- Escalation report final_pane (cycle-1599 triage): the dialog verbatim with
  `❯ No, exit` selected, then the shell prompt back — claude exited.
- Live reproduction same day: fresh tmp dir, `claude --model haiku
  --dangerously-skip-permissions` → identical dialog.
- Remedy verified live before coding: `Down` moves the cursor to
  "Yes, I trust this folder", `Enter` boots a trusted REPL (status bar
  confirms workdir + model).

## Fix (fix/claude-2252-trust-dialog)
New manifest rule `trust_prompt_no_default` (claude-tmux): matches the
selected-No line plus the Yes option, bottom-anchored on the
`Enter to confirm · Esc to cancel` footer (`\z`, `tail_lines` 12, distance
bounds — the plan_approval discipline from
2026-08-27-plan-mode-dialog-blind-spot.md), responds `Down,Enter`,
fire-once. The v2.1.193 rule stays for older builds and was bottom-anchored
symmetrically in the same change — the architecture review's probe showed
the unanchored form self-matching this repo's own tracked files when an
agent renders them, and showed the fix's own test fixture becoming a new
trigger with a masking flip (the old rule's once-budget, previously spent at
boot, would survive to fire on the first rendered file and then suppress a
genuine later dialog).

## Lessons
- A CLI's interactive dialogs are part of its interface contract: version
  drift WARNs at preflight deserve a dialog-smoke before a batch, not after.
- Every auto-respond rule carries the bottom-anchor discipline from birth;
  "prose cannot false-match" is not a property — `\z` + `tail_lines` is.
- Fixture text of dialogs inside tracked files is itself a false-match
  vector; the self-match guard test must cover every rule family that quotes
  its own dialog.

## 2026-10-05 — the blind `Down,Enter` lost its Down and confirmed "No, exit"

### Impact
Wave 59's first launch halted before any cycle ran: the loop readiness gate's
`bridge-boot` check failed for `claude-tmux` (rc=80, `ExitREPLBootTimeout`)
and the loop stopped with `stop_reason=preflight_failed`. The agy and codex
smoke boots passed. The rule fires on every boot in a fresh worktree or
preflight scratch directory, so the race costs a wave whenever it loses:
1 of the 38 fires the loop logs of waves 30–59b record (2026-09-29..10-05)
lost its Down. The restarted wave (59b) booted normally.

### Root cause
`trust_prompt_no_default` answered the dialog with one blind burst,
`Down,Enter`, 500 ms apart. The first boot tick runs about 1 s after the
launch, which is about when the dialog mounts, so the Down could arrive
before the dialog read input. That time the Down was dropped and the Enter
was not: it confirmed "No, exit", claude exited to zsh, and the dead-shell
guard rejected the marker for the remaining 60 s. Nothing checked that the
Down had moved the cursor before the Enter went out.

### Evidence
- The runtime plane's loop log for wave 59 (`.evolve/loop-20261005-wave59.log`,
  lines 14–118): one `[auto-respond] sent keys: Down,Enter
  (rule=trust_prompt_no_default)`, then `marker visible but pane process is a
  shell (zsh)` on every tick until `REPL prompt never appeared after 60s`. The
  readiness gate's final pane shows the dialog still on its default (cursor on
  "No, exit"), then the shell prompt: the Enter confirmed No.
- Live probe on claude 2.1.285, the same day (a fresh untrusted temp
  directory in tmux): the dialog renders about 0.5–1 s after the launch;
  `Down` moves the cursor No → Yes; a second `Down` wraps it Yes → No (so
  does `Up`); with the cursor on Yes, `Enter` removes the dialog within 250 ms
  and the REPL marker shows within 1 s.
- The new boot test's fake pane, set to swallow the first Down, reproduces
  the wave-59 log line for line on the old manifest.

### Fix (fix/trust-prompt-verify-before-confirm)
Verify before confirm, as data ([ADR-0118](../architecture/adr/0118-confirm-a-menu-only-on-the-observed-choice.md)):

- `trust_prompt_cursor_on_no` matches the dialog with the cursor on
  "❯ No, exit" and sends `Down` only. It is not fire-once: it repeats on each
  boot tick that still shows the cursor on No (a swallowed Down, or a Down that
  wrapped back), up to the auto-respond loop guard's budget.
- `trust_prompt_cursor_on_yes` matches the same dialog with the cursor on
  "❯ Yes, I trust this folder" and sends `Enter`. It is the only rule that
  confirms this dialog, so Enter is only ever sent on a capture that shows the
  cursor on Yes.
- Both keep the bottom anchor (footer, `\z`, `tail_lines` 12, distance
  bounds). The confirm rule is strict: it needs the "No, exit" line with no
  cursor directly above an unnumbered "❯ Yes" line, so a torn redraw with a
  cursor on both lines gets the navigation rule's Down. The navigation rule is
  permissive on purpose. The three trust rules are disjoint on every frame the
  dialogs render; for text no build renders, manifest order (navigation first)
  is the tiebreak, so ambiguity sends Down, never Enter.
- The engine's boot waits (`bootTmuxREPL`, and the recipe adapter's
  `EnsureSession`) share one rule, `autoResponder.bootTick`, on one capture per
  pass: the tick runs on that frame, a tick that sent keys re-polls (it was "a
  fire-once rule fired"), and only a pass that sent nothing judges readiness, on
  the frame the rules saw. The dialog's ❯ cursor is the claude REPL marker and a
  repeatable rule leaves the dialog up between ticks. The recipe used to tick
  one frame and judge another, so a dialog mounting between the two captures
  went unanswered and the recipe's first command confirmed "No, exit".
- A tick that trips the loop guard ends the boot at once with a FAIL line
  naming the rule ("matched more than 5 times": the guard counts matches, and
  an escalate rule sends nothing) and exits `ExitREPLBootTimeout` (rc 80), the
  class the boot-failure bench counts. The boot wait used to discard the guard
  and time out 60 s later with the same rc.

Pinned by `TestClaudeTrustDialog_ASwallowedDownNeverLetsEnterConfirmNo`,
`TestClaudeTrustDialog_ADownThatLandsBootsWithOneDownAndOneEnter`,
`TestClaudeTrustDialog_ACursorThatNeverMovesEndsTheBootOnTheLoopGuardWithoutConfirming`,
`TestClaudeBoot_AnEscalatePromptThatNeverClearsEndsAsABootTimeoutWithoutSendingKeys`,
`TestClaudeTrustDialog_ADialogMountingAfterLaunchStillBootsOnYes`,
`TestRecipeBoot_TrustDialogIsConfirmedOnYesBeforeReady`,
`TestRecipeBoot_ADialogMountingAfterLaunchIsAnsweredBeforeTheFirstCommand`,
`TestRecipeBoot_ACursorThatNeverMovesFailsOnTheLoopGuard`,
`TestClaudeTrustDialog_AFailedCaptureOfTheDialogIsNeverReadiness`,
`TestRecipeBoot_AFailedCaptureOfTheDialogIsNeverReadiness`,
`TestClaudeTrustDialog_ALostEnterOnYesIsRetriedAndBootsOnTheREPL`,
`TestClaudeBoot_AnEscalateMatchOnTheREPLLeavesReadinessToTheMarker`,
`TestAutoRespond_ClaudeTrustRulesAreDisjointOnEveryDialogFrame`,
`TestAutoRespond_ClaudeTrustNavigationWinsAFrameBothRulesMatch` and
`TestAutoRespond_ClaudeTrustDialog_v2252`; the self-match guard reads the new
fixture file too.

The architecture review (Block, then fixed in the same change) found three
defects in the first draft: the loop-guard exit (rc 86) escaped the
boot-failure bench, which reads every exit but 80 as "booted"; the recipe boot
ticked one frame and judged readiness on another; and the confirm regex also
matched a frame with a cursor on both lines, leaving only manifest order
between a Down and an Enter. Its re-review (Warning) found that a failed capture's
frame could still decide readiness (tmux returns output with the error), now
skipped, and pinned the lost-Enter retry and the rc 85 path.

### Residual risk
A Down still unrendered a full boot interval (1 s) after it was sent, at the
capture that shows the cursor on Yes, could wrap the cursor back to No just
before the Enter. That needs a render latency above 1 s twice in a row; the
live dialog moved in well under 0.5 s. The codex and agy trust rules are not
the same shape: codex answers with the absolute hotkey `1` on a Yes-default
menu and agy with a bare Enter on a Yes-default menu, so a lost key there still
confirms Yes.

### Lessons
- A keystroke burst against a menu is open-loop control: each key assumes the
  one before it landed. A menu whose default is destructive gets one rule per
  observable state, and the confirm key goes only to the state that shows the
  wanted choice.
- A boot wait that re-checks a marker the dialog itself renders must not judge
  a pane on a tick that changed it.
