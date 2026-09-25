# 2026-09-26 — a correction stalled because its deliverable was already right

**Class:** pipeline. A correct correction round could not complete: the bridge waited for a deliverable rewrite that the agent had no reason to make, and its one reminder did not say why.
**Surface:** `bridge/completion.go` (the dispatch baseline), `bridge/driver_tmux_wait_disposition.go` (the idle nudge).
**Found by:** the console, watching wave 8 (cycle 1691, lane `warn-ship-consumption-gap`) during the six-consecutive-ships campaign.

## What happened

At 04:37:58 the build handoff floor rejected 1691's build: one Changed Areas path in the explanation document needed a what/why line. Correction 1/2 re-dispatched the builder.

At 04:39 the builder fixed the explanation document. That was correct: `build-report.md` needed no change, and `evolve phase verify build` reported it well-formed.

The bridge's completion check did not accept it. The dispatch baseline records each candidate deliverable's size and mtime when the dispatch starts, and completion requires the deliverable to change after that. This is the stale-leftover guard (cycle 1550: never accept a pre-dispatch artifact as this attempt's result). The report's mtime, 04:37:45, predated the correction dispatch, so the bridge treated the phase as incomplete. When the agent went idle it sent the one-shot nudge, "Please write the deliverable to …/build-report.md to complete the phase."

At 04:58 the agent re-verified the report, found it well-formed and accurate, and answered "Phase is complete" without rewriting it. It then sat idle. The stop-review would have closed the phase as exit 81, failing a correct cycle on a pipeline defect. At 05:16:07 the operator touched `build-report.md` (content unchanged, verified), and the phase completed 90 s later and moved on to audit.

## Root cause

The stale-leftover guard is right for **every** dispatch; its founding case (cycle 1550) was itself a correction re-dispatch. What was missing is a second, honest path for a deliverable that is legitimately carried forward, when a correction's fix lives in another file (an explanation document, source, a test) and the previous attempt's deliverable is still this phase's accurate result. Nothing asked the agent to re-check and re-affirm it. The nudge named only *what* to do ("write the deliverable"), never *why*: the host detects completion by the deliverable being written after this dispatch. An agent that reads the file and finds it correct reasonably concludes there is nothing to write.

## Fix (F39, part 1)

`idleNudgeFor` is the one decision the reminder's text, the operator log label and the interaction trigger come from.
- **Resolution.** It locates the deliverable through `artifactLocate`, the resolution the completion poll and the admission check share, so a leftover at a fallback location counts too.
- **When the deliverable still matches the dispatch baseline** (`artifactBaseline.matches`):
  - The reminder names it and the canonical path the rewrite must land on, says it was not rewritten during this attempt, and explains that the host completes the phase only when the deliverable is written after this dispatch.
  - It binds carrying the report forward to a **re-check** against this attempt's work (architecture review M1). A stale verdict must never be carried forward unexamined, which is the cycle-1550 case in a new form.
  - The remedy follows the format (M2): a markdown report takes an appended line recording what the attempt changed and that it was re-verified; any other format (a JSON plan) is written again in full, because a line appended to JSON breaks its parse.
  - The log label is "idle with an unrewritten deliverable" and the trigger is `idle_unrewritten_deliverable`.
- An absent deliverable keeps the plain reminder, the `idle with missing artifact` label and the `idle_no_artifact` trigger.

## Pins

All in `bridge/nudge_stale_deliverable_test.go`:
- `TestIdleNudgeFor_NamesAnUnrewrittenDeliverableAndBindsARecheck`: unchanged since dispatch, absent, and changed since dispatch; the label and trigger come from the same decision.
- `TestIdleNudgeFor_CarriesANonMarkdownDeliverableForwardInFull`: goes red when every format is told to append (mutation-checked).
- `TestIdleNudgeFor_SeesALeftoverAtAFallbackLocation`: goes red when resolution is canonical-only (mutation-checked).
- `TestTmuxREPL_IdleNudge_ExplainsAnUnrewrittenDeliverable`: the real claude-tmux wait with the production baseline sends exactly one explaining nudge, labeled in the log. It goes red when the wait loop sends the plain wording (mutation-checked).
- `TestTmuxREPL_IdleNudge_AnAgentThatReChecksAndAppendsCompletes`: an agent that appends its re-verification line after the nudge completes with exit 0, with no operator touch.

The existing nudge contracts are unchanged: sent once, submit-verified, bounded resend, and no nudge once the deliverable has been written since dispatch.

## Not fixed here (F39 part 2, console-owned: `correction-completion-needs-deliverable-rewrite`)

Part 1 relies on the agent acting on an explicit, reasoned reminder. Part 2 adds a second, deterministic evidence source for a carried-forward deliverable without relaxing the guard. The acceptance is in the item.
