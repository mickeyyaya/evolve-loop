# 2026-08-27 — Plan-mode dialogs: a live-reachable hang no manifest rule matched

## Issue

Both tmux CLIs can enter **plan mode** from the loop's production launch
configuration, and both then block on an interactive dialog that the
auto-responder had no rule for.

The pre-fix consequence is worse than a clean hang, and the first draft of this
document got it wrong by asserting one. claude's plan dialog reads **not busy**
(no `esc to interrupt`, no spinner line), so the first PAUSE checkpoint fires the
deterministic nudge — which sends `"Please write the deliverable to <path>"`
**plus Enter** into a modal option list. Artifact paths contain digits and claude
menus take digit hotkeys, so the real pre-fix behaviour is a *nondeterministic
keystroke into a modal*, landing on option 1, 2, or 3 depending on the path. That
is a stronger reason for a deterministic rule than the hang I originally claimed.

Captured live 2026-08-27:

- **claude** — an agent called `EnterPlanMode` from a
  `--dangerously-skip-permissions` session. The footer flipped from
  `⏵⏵ bypass permissions on` to `⏸ plan mode on`, and `ExitPlanMode` then raised:

  ```
  Claude has written up a plan and is ready to execute. Would you like to proceed?
  ❯ 1. Yes, and switch to BYPASS PERMISSIONS (no further prompts) for this session
    2. Yes, manually approve edits
    3. Tell Claude what to change
  ```

  **Bypass permissions does not prevent plan mode.** This was the load-bearing
  assumption that made the hole invisible.

- **codex** — `codex-cli 0.147.0` reports `collaboration_modes` at stage
  `removed` with state `true` (graduated, permanently on). `/plan` engages
  `Plan mode` in the status bar, and the agent then asks clarifying questions
  through a blocking picker (`Question 1/2 (2 unanswered) … enter to submit
  answer`).

Verified by running every existing manifest regex against both live panes:
**zero matches** on either CLI.

## Gap

Three separate things hid this:

1. **A stale code comment stood in for a capability check.** `driver_codex.go`
   states "Codex has no claude-style plan mode." That is true of the absent
   `--permission-mode` *flag* and false of the *mode*, which codex reaches via
   `/plan` or Shift+Tab. The comment was read as settling the question.
2. **The permission model was assumed to be the gate.** `--dangerously-skip-permissions`
   suppresses tool-approval prompts, so plan mode was assumed unreachable. It
   is not: plan mode is a collaboration mode, orthogonal to permissions.
3. **The stream classifier already knew — and it would not have helped.**
   `phasestream/classify.go` special-cases `ExitPlanMode` into a
   `KindInteraction` envelope. Nothing *acts* on that envelope (it is consumed
   by `phasestream/mask.go` only as a never-evict retention class), so the
   signal is recorded and never answered. But this is a weaker gap than it
   looks: `phasestream` is the stream-json classifier for the headless
   `claude -p` transport, which is opt-in — the default execution path is the
   tmux drivers, which have no JSON stream at all. No consumption policy on
   `KindInteraction` could have covered the path that actually hangs. The real
   signal that this was reachable was mundane: the loop's own tool reference
   (`skills/loop/reference/claude-tools.md`) advertises `EnterPlanMode` /
   `ExitPlanMode` to phase agents.

## Solution

One `interactive_prompts` rule per CLI, `policy: auto_respond`, `Enter`:

- `claude-tmux` / `plan_approval` — Enter accepts the pre-highlighted option 1,
  an approval in both observed variants. An autonomous loop has no human to read
  the plan, and the auditor grades the resulting work regardless, so approving
  beats a dead lane.
- `codex-tmux` / `plan_question` — Enter submits the pre-selected
  `(Recommended)` option; the rule re-fires per tick as the agent walks a
  multi-question form.

**Two anchors, and a line window.** Claude's option 1 text *varies by launch
mode* — "Yes, and use auto mode" under `--permission-mode plan` versus "Yes, and
switch to BYPASS PERMISSIONS…" under bypass — so a rule keyed on option 1 passes
a test written from either capture and fails silently in production. Both rules
therefore require an invariant sentence plus a stable second line.

Anchors alone were not enough, and the first draft of this fix shipped exactly
that mistake. This repository *documents* these dialogs — this file contains
both verbatim, as do the test fixtures — on tracked files an agent may `cat`.
`stripAgentDiffLines` protects a `git diff` view because those lines are `+`
prefixed; it does not protect a Read or `cat` render. So the rules' own source
material was a false-match vector, and a byte-distance bound could not fix it:
a *generous* bound still matches a dismissed dialog with a little output beneath
it (measured: 45 characters of subsequent output still fit inside a 400-byte
tail), while a *tight* one breaks when the dialog's own footer wraps.

The fix is structural, and it took two attempts. The first used a bounded
any-content tail (`.{0,N}\s*\z`) and **did not work** — under `(?s)` a dot
already matches newlines, so the bound absorbs real new output and the rule
re-fires on a dismissed dialog. Measured against the compiled rules: 1-3 lines
of realistic trailing output still matched. The tests passed only because their
3-4 line trailers happened to also cross the line window — the right answer for
the wrong reason, which is worse than a red test.

Both tails are now anchored to each dialog's **own final line**: claude's must
end at the footer's `ctrl+g … <plan>.md`, and codex's uses `[^\n]*`, which
cannot cross a newline, so the picker footer must be the last line. Any new
line beneath the dialog now ends the match structurally rather than after a
budget. Verified: one line of output below either dialog is enough to stop it.

`ManifestPrompt.TailLines` restricts a rule to the last _n_ lines of the
capture, as the second half of the same guarantee. A live modal is always at the bottom; anything that
scrolled above it — an answered dialog, or one quoted mid-document — is then
**unmatchable rather than merely improbable**. Both plan rules use a 9-line
window, sized against the live dialogs: the pinned claude fixture is 9 lines
with its first anchor on line 4, so the window still holds the anchor with up
to 3 lines appended beneath — a real, measured tolerance rather than an
absolute, and the reason the codex half of this needs its own post-answer
capture before that CLI's plan mode is ever enabled.

Verified against the raw captures rather than transcriptions: the live claude
dialog and the live codex picker both fire, and the pane captured **immediately
after answering** does not. That last capture also settled a question the review
raised — on claude the answered dialog does not linger at all; it is gone from
the pane within three seconds.

## Scope of the claim

This is a **net for a reachable hole, not a fix for an observed burn**. Across
384 retained run directories (cycles 1217–1574) there are zero occurrences of
either dialog or of the plan-mode footer. No phase agent has entered plan mode
in production. The rules exist so that the first one to do so does not cost a
lane.

## Operational note for enabling plan mode deliberately (SUPERSEDED — see the follow-up below)

Entering plan mode on codex **silently drops reasoning effort to the plan-mode
preset** — observed live as `gpt-5.6-sol xhigh` → `gpt-5.6-sol medium` the moment
`/plan` engaged. **Superseded 2026-08-27:** the loop now emits
`-c plan_mode_reasoning_effort=<tier>` on every codex launch, so no operator
action is needed and `config.toml` is the wrong place for it anyway — it is
host-global (no per-phase tier), it leaks into the operator's own interactive
sessions, and that file is already append-only trust state serialised under a
lock by `codex_pretrust.go`. The original advice is kept here only so the
follow-up section reads in sequence. There is no documented config key that forces a session to
*start* in plan mode; entry is `/plan` or Shift+Tab, both in-session, which the
tmux drivers can send.

## Follow-up (2026-08-27, later) — the effort downgrade is now fixed, and fixing it exposed a realizer defect

**Issue.** The operational note above said plan mode drops codex's reasoning
effort and left the fix to whoever enabled plan mode. That was too passive: the
loop passes `-c model_reasoning_effort=<tier>` on every codex launch, and plan
mode ignores it. Codex's config reference is explicit — `plan_mode_reasoning_effort`
is a "plan-mode-specific reasoning override" and "when unset, Plan mode uses its
built-in preset default"; there is no fallback to the general key, and no
plan-mode *model* key at all (`model` applies across modes).

Verified live on codex-cli 0.147.0, launched as the loop launches it:

| launch | before `/plan` | after `/plan` |
|---|---|---|
| `-m gpt-5.6-sol -c model_reasoning_effort=xhigh` | `gpt-5.6-sol xhigh` | **`gpt-5.6-sol medium`** |
| … plus `-c plan_mode_reasoning_effort=xhigh` | `gpt-5.6-sol xhigh` | **`gpt-5.6-sol xhigh`** |

**Gap.** Every parity assertion in the tree pins *launch flags*, so all of them
stayed green while the session ran a tier lower. A gate that cannot observe the
downgrade is worse than no gate, because it gets cited as evidence the downgrade
did not happen.

**Solution.** Every tier in `codex-tmux.json`'s effort param now emits both keys
in lockstep, so plan mode inherits the phase's tier instead of a preset.

**And the realizer would have silently dropped it.** `dedupeLaunchFlags` deduped
individual TOKENS, so the second `-c` was discarded as a duplicate and the
realized argv became `-c model_reasoning_effort=high plan_mode_reasoning_effort=high`
— the plan key demoted to a bare positional. Its own doc comment had warned that
"flag-value pairs that legitimately repeat should NOT be deduped this way", and
separately claimed `-m a -m b` was "correctly kept twice", which was false: that
input produced `-m a b`, passing a model name as a positional argument, i.e. into
the prompt. Two tests **endorsed** the old behaviour — one calling it "the documented
contract", one a "documented footgun" — while two exact-argv expectations
**detected** it (the realized-manifest pin and the end-to-end launch string). A
wrong belief replicated in the prose, caught by the assertions that spelled the
command line out in full. Dedupe now operates on flag-value **units**; the two
endorsing pins are flipped and the two detecting pins updated to the corrected
argv, each with the reason recorded.

The exact-argv assertions are what caught this. Had they used `Contains` instead
of equality, the dropped flag would have shipped silently.

## Two further defects this work surfaced

**A manifest regex that does not compile is silently inert.** `decideAutoRespond`
does `regexp.Compile(p.Regex); if err != nil { continue }`, so an invalid pattern
is not a loud failure — it is a rule that quietly does nothing, and the symptom
is the hang it was written to prevent. Found the hard way: a `{0,1200}` bound was
authored here, Go's RE2 caps repeat counts at 1000, and the rule was dead on
arrival — visible only because a new test happened to assert it fired.
`controls.exhausted_regex` has had a compile guard for months;
`interactive_prompts`, the field whose silent death costs a lane, had none.
`manifest_prompt_regex_compile_test.go` now covers every rule in every manifest.

**Existing rules already false-match this repository's own files.** Feeding
`manifests/claude-tmux.json` through the responder as if an agent had rendered it
fires `cli_feedback_rating` (`send:0,Enter`) and, for the codex rule set,
`escalate:auth_recheck` — because the manifest contains those patterns as *data*.
This predates the plan rules and is not fixed here; the new self-match guard is
deliberately scoped to the plan rules so it cannot fail for a reason it cannot
address. Filed as a follow-up.

## Lessons

1. A code comment is a claim about the world at the time it was written. This one
   was three CLI releases stale and was believed twice — once when the driver
   rejected `permission_mode`, and once when it was quoted back as evidence that
   the feature did not exist.
2. Coverage on one transport is not coverage. `ExitPlanMode` was classified
   years-deep in the stream-json path and that path is opt-in, so the tmux
   default — the one that actually hangs — was never in scope. "We already
   handle that" is a claim about a code path, not about a product.
3. When a matcher's fixture is transcribed by hand, verify it against the raw
   capture too. Both rules here were re-checked against the untranscribed probe
   output, because a transcription slip yields a rule that is green in test and
   inert in production.
4. "Improbable" is not a safety property. The first draft defended against
   *paraphrase* of the dialog and called it protection against the repo's own
   docs — which quote it *verbatim*. The distinction only became visible when a
   review asked what happens when an agent reads the file, and the fix that
   followed (a line window) is the difference between unlikely and impossible.
5. A bound is not a boundary. `.{0,400}\s*\z` reads like "nothing but
   whitespace to the end" and means "up to 400 characters of anything." The
   trailing `\s*` was vestigial and the comment describing it was confident.
   Anchor to content you can name, not to a budget you hope is small enough.
6. A saved capture is not a pane. After the tail anchors landed, the codex rule
   looked inert against the stored capture — because that file has
   `[exited with code 0]` appended by the harness that saved it, so the footer
   was no longer the final line. `tmux capture-pane` never appends that. The
   temptation is to loosen the rule until the fixture passes; the correct move
   is to find out why the fixture differs from production.
7. The evidence that settles a design question is often one capture away. The
   whole re-fire debate — `once: true` versus not, loop-guard headroom, scrollback
   depth — was resolved by capturing the pane immediately after answering the
   dialog, which took under a minute and had not been taken.
