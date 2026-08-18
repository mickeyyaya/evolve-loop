---
score_cap:
  - criterion: "An unsubmitted one-shot nudge is detected and Enter is re-sent, loudly"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestTmuxREPL_NudgeUnsubmitted_ResendsEnter$' ./internal/bridge"
  - criterion: "A submitted input line triggers NO re-send (no double-submit / pane desync)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestTmuxREPL_NudgeSubmitted_NoResend$' ./internal/bridge"
  - criterion: "Re-sends are bounded; a never-clearing input line terminates instead of spinning"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestTmuxREPL_NudgeUnsubmitted_ResendBounded$' ./internal/bridge"
  - criterion: "The same submit-verify covers the prompt-paste delivery site (driver_tmux_repl.go:368-376)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestTmuxREPL_PromptPasteUnsubmitted_ResendsEnter$' ./internal/bridge"
---

# Eval: tmux driver verifies that its sends were actually submitted

> Pins the submit-verify contract on the claude-tmux REPL driver's send path.
> The driver used to fire keys with `enter=true` and walk away: `PasteBuffer` +
> one blind Enter at `driver_tmux_repl.go:374-376`, and a one-shot nudge at
> `:806-818` guarded only by a `nudgeSent` bool. In cycles 1505, 1510 and 1517
> the nudge was still parked, unsubmitted, at the pane's `❯` input line in the
> final capture, and every nudge record in `<phase>-interactions.ndjson` read
> `"result":"no_effect"` — the driver had no way to know its own key send did
> nothing. Source incident: cycle 1526 (4th recurrence of the
> `retro-prompt-delivery-stall` class; the "prompt was never submitted" framing
> was falsified by that cycle's premise challenge, which relocated the defect to
> the nudge site and to the missing verification generally).
>
> Two of the four caps are deliberately paired: "re-send when stuck" and "never
> re-send when clear". Either alone is gameable — an unconditional second Enter
> satisfies the first while re-submitting whatever the agent typed next, which
> is a worse pane desync than the stall it replaces.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| resend-when-unsubmitted | Nudge parked at `❯` ⇒ Enter re-sent, attempt logged loudly | 8/10 | `go test -run TestTmuxREPL_NudgeUnsubmitted_ResendsEnter ./internal/bridge` |
| no-resend-when-clear | Input line cleared ⇒ zero extra Enters (anti-double-submit) | 8/10 | `go test -run TestTmuxREPL_NudgeSubmitted_NoResend ./internal/bridge` |
| bounded-retry | Never-clearing pane ⇒ capped re-sends, run still terminates | 7/10 | `go test -run TestTmuxREPL_NudgeUnsubmitted_ResendBounded ./internal/bridge` |
| shared-submit-path | Prompt-paste delivery verified by the same code path | 6/10 | `go test -run TestTmuxREPL_PromptPasteUnsubmitted_ResendsEnter ./internal/bridge` |
