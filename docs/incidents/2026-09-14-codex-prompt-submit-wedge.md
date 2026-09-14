# 2026-09-14 — "submit_wedged": the driver declared a 35 KB prompt wedged 2.5 s after pasting it

**Status:** fixed (this change) · **Severity:** P1 pipeline (whole dispatches thrown away; the most frequent token burner of the first post-decomposition wave) · **Found by:** the Signal Center stream (`BRIDGE_EXIT_ARTIFACT_TIMEOUT … cause=submit_wedged`) on cycles 1673, 1674 and 1675, then the per-phase `submit_verify` interaction ledger.

## What the stream and the ledger said

```
[bridge] bridge.warning WARN BRIDGE_EXIT_ARTIFACT_TIMEOUT cycle=1674 phase=scout attempt=1 … cause=submit_wedged reason="prompt submit_wedged (resends=3)" driver=codex-tmux
[bridge] bridge.warning WARN BRIDGE_EXIT_ARTIFACT_TIMEOUT cycle=1675 phase=scout attempt=1 … cause=submit_wedged … driver=codex-tmux
[bridge] bridge.warning WARN BRIDGE_EXIT_ARTIFACT_TIMEOUT cycle=1673 phase=build attempt=… cause=submit_wedged … driver=codex-tmux   (the recovery build after SHIP_GIT_FLEET_REBASE_NEEDED)
```

`*-interactions.ndjson`, cycle 1674, `kind=submit_verify`:

| phase | payload | result |
|---|---|---|
| scout (attempt 1) | resends=3 | **submit_wedged** → exit 81, dispatch discarded |
| build | resends=2 / resends=3 | submitted_after_resend |
| bug-reproduction | resends=2 | submitted_after_resend |
| fault-localization | resends=2 | submitted_after_resend |
| error-handling-scan | resends=3 | submitted_after_resend |
| audit (claude-tmux) | resends=0 | submit_verified |

Every codex-tmux phase in the wave needed two or three re-sends just to land; the two largest prompts (scout 17.7 KB, build 35 KB) hit the cap.

## Root cause

`pastePrompt` loads the prompt into a tmux buffer, pastes it, sleeps a **fixed 1 s**, and presses Enter. The codex TUI is still ingesting a multi-KB bracketed paste at that point — it renders `[Pasted Content 1 +N lines]` and grows N as it goes — so the Enter is swallowed into the paste. `verifySubmitted` then sees the chip still parked at the `›` line and re-sends Enter three times **500 ms apart**: three re-sends in 1.5 s, all still mid-ingestion, and the driver gives up "wedged" ~2.5 s after the paste. The re-send cap is the right idea (hammering a truly wedged pane is not recovery); the timing was tuned for a claude-tmux prompt a fraction of the size.

## Fix (timing only — vocabulary, stderr lines, the cap and the short-circuit are unchanged)

| Where | Change | Test (red first, `driver_tmux_submit_settle_test.go`) |
|---|---|---|
| `paste_settle.go` (new — the ONE delivery tail, `settlePasteThenEnter`, used by the prompt paste AND `injectText`) | the settle before the first Enter scales with the paste: `1 s + 1.5 s per 10 KB`, capped at 6 s (`pasteSettleFor`; 1.5 s so no size lands on the 2 s `artifactWaitInterval`); an unreadable prompt is said out loud and treated as a 50 KB paste | `TestPastePrompt_WaitsForTheStablePaneBeforeEnter`, `TestPastePrompt_UnreadablePromptAssumesALargePaste`, `TestPasteTiming_NeverEqualsTheArtifactWaitInterval`, `TestInjectText_SharesTheDeliveryTail` |
| `waitPaneStable` | before pressing Enter, wait for the pane to **stop changing** — two consecutive captures agree — bounded to 8 polls of 500 ms; pastes under 2 KB take the byte-identical old path (1 s, no capture) — the no-regression floor every fixture sits under; the outcome (`stable` / `unstable` / `skipped` / `capture_fault`) and the settle ride the submit-verify ledger payload (`paste_settle=… stability=…`) on the success path too | `TestPastePrompt_EnterFollowsTwoAgreeingCaptures`, `TestPastePrompt_StabilityWaitIsBounded`, `TestPastePrompt_SmallPromptTakesTheCaptureFreePath`, `TestPastePrompt_CaptureFaultFailsOpen`, `TestRecordSubmitVerify_CarriesThePasteEvidence` |
| `verifySubmitted` | re-send settles back off through `submitVerifyBackoff` (one entry per allowed re-send: 500 ms → 1.5 s → 2.5 s — no entry equals `artifactWaitInterval`, now named once in `driver_tmux_wait.go`) | `TestVerifySubmitted_BacksOffBetweenResends`; `TestTmuxREPL_PromptSubmitWedged_ShortCircuitsSilenceBudget` (existing) still green |

Budget before a submission is called wedged, for a 35 KB codex prompt: was 1 s + 3 × 0.5 s = 2.5 s; now up to 5.5 s (settle) + up to 4 s (stability) + 4.5 s (backoff) ≈ 14 s, and the common case costs nothing extra beyond the size-scaled settle because the stability poll returns on the first agreeing capture.

## What the operator will see

`submit_verify … result=submit_verified resends=0` for the large codex prompts, `submitted_after_resend` only when the TUI genuinely needed a nudge, and `BRIDGE_EXIT_ARTIFACT_TIMEOUT cause=submit_wedged` reserved for a pane that is actually wedged.

## Follow-ups

- The driver could read the chip's `+N lines` and compare it with the prompt's line count as a positive "ingestion complete" signal instead of pane stability; deferred until the wave shows the stability wait is not enough.
- `interaction.ResultSubmittedAfterResend` at resends ≥ 2 on a given driver is a leading indicator worth a WARN signal of its own (a `bridge.warning` per phase with `fields.resends`) — filed as a follow-up so the next timing regression is one grep away.

## Review folds (diff-scoped fleet: code-simplifier → architecture-reviewer ∥ go-reviewer)

Simplifier: no edits. Go review: FIX_THEN_MERGE — MAJOR: an unreadable prompt on the Go side silently became size 0 and skipped the fix (now said out loud, treated as a 50 KB paste); MINORs: the unread bool return dropped, the duplicated capture-fault line given one home, the backoff as a table. Architecture review: FIX_THEN_MERGE, every finding folded red-first:

| Finding | Fold |
|---|---|
| HIGH-1 — the settle formula itself produced exactly 2 s for 10–20 KB prompts, the value the wedge short-circuit pins count as an artifact-wait poll | `artifactWaitInterval` named once in `driver_tmux_wait.go`; settle is 1.5 s per 10 KB; `TestPasteTiming_NeverEqualsTheArtifactWaitInterval` sweeps 0–120 KB, the backoff table, the poll and the cap |
| HIGH-2 — `injectText` kept the flat-1 s twin of the fixed belief | one delivery tail, `settlePasteThenEnter` in `paste_settle.go`, used by the prompt paste and the inject; small bodies are byte-identical to before |
| HIGH-3 — the new diagnostics lived only on stderr, which survives the failure path only | `pasteOutcome{Settle, Stability}` rides the submit-verify ledger payload (`paste_settle=… stability=…`) — the success path now has a denominator; the nudge site records none |
| MEDIUM — backoff rationale orphaned; delivery policy split across two files; prefix-less stderr; scrollback 0 unstated; the 2 KB floor and the capture-fault fail-open unpinned | rationale on the table; `paste_settle.go` owns settle + floor + stability + Enter, `human_input.go` keeps only the human cadence; every line carries the per-driver prefix; scrollback 0 explained; `TestPastePrompt_SmallPromptTakesTheCaptureFreePath`, `TestPastePrompt_CaptureFaultFailsOpen` |
| LOW — capture-count assertions pinned mechanism; expectation computed from a different size than the fixture | `TestPastePrompt_EnterFollowsTwoAgreeingCaptures` (the last two captures agree); `writePromptSized` returns the real size |

One pin was updated deliberately: `tmux_codex_paste_test.go` pinned the exact submit-verify payload JSON, which now carries the additive `paste_settle=1s stability=skipped` suffix for its 19-byte fixture prompt.
