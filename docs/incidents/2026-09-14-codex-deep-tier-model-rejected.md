# 2026-09-14 — codex's deep tier pointed at a model the account rejects, and the bridge waited 25 minutes to notice

**Class:** pipeline (routing wall not recognised; every codex deep/top dispatch stalls before falling back).
**Surface:** `internal/bridge/manifests/codex-tmux.json` `interactive_prompts` (the auto-responder's rule set).
**Found by:** the operator, wave 2 of the pipeline-health verification — "codex limit reached, why doesn't those cycle fallback to claude".

## What happened

Lanes 1676 and 1677 dispatched build on `codex-tmux@deep`. The pane printed

```
⚠ Model metadata for gpt-5.6-sol not found. Using default model metadata.
■ {"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The 'gpt-5.6-sol' model is not supported when using Codex with a ChatGPT account."}}
```

twice and returned to the idle prompt. Nothing matched: the codex rule set knows trust
prompts, per-edit approvals, the auth re-check, the plan-mode question picker and the
rate-limit vocabulary (`usage limit reached`, `too many requests`, `quota exceeded`). A
400 `invalid_request_error` is none of those, so the artifact wait ran its full window —
`build-escalation-report.json` in both runs: `stop_kind: artifact_timeout`, "no output
during the last 1500s interval" — and only then did the runner fall back:
`fallback 1: trying cli=claude-tmux tier=deep (previous=codex-tmux@deep exit=81)`. Both
builds then passed on Claude. It was not a rate limit.

## Root cause

Two facts, one gap each:

1. The account no longer accepts `gpt-5.6-sol`, which is what `model_tier_map`
   pins codex's `deep` and `top` tiers to (the 2026-09-10 cost directive reverted the
   astra cutover back to sol). Every deep/top dispatch to codex is dead on arrival.
2. The bridge recognises a wall only through a manifest rule. A dead model prints an
   error and idles — the same shape as a quota wall — but no rule named it, so the
   bridge treated silence as "still working" until the artifact window expired.

## Fix (this change)

A `model_unsupported` rule, `policy: escalate`, matching the 4xx
`invalid_request_error` JSON codex prints or the sentence `The '<model>' model is not
supported when using Codex`, restricted to the last 30 lines of the pane (the live
screen) and, like every escalate rule, gated on the pane being idle so an agent quoting
the JSON while it works cannot fire it. Escalate is exit 85, the path the quota wall
already takes: `recipe_adapter` treats 85 as a fallback trigger and the runner moves to
the next family in seconds instead of after 25 minutes. The pattern name is deliberately
NOT `rate_limit`, so `clihealth` does not bench the codex family — codex at the other
tiers still works.

Red first: the real-manifest decision matrix row for the lane-1676 pane returned
`noop/0`; `TestDecideAutoRespond_CodexModelUnsupportedIsIdleGated` pins the busy gate.

## Not fixed here — the operator's call

The deep/top pin itself. Until codex's `deep`/`top` tiers point at a model the account
accepts, every deep/top dispatch to codex still fails, now in seconds rather than
minutes, and lands on Claude. Re-pinning is a high-model change and needs the operator's
explicit cost decision (memory: model_cutover_gpt56). Note that the manifest's
`chatgpt_safe_models` still lists `gpt-5.6-sol` — that list is stale, not a guard; the
account's own default (`~/.codex/config.toml`) is `gpt-5.3-codex-spark`. Whether the
`balanced`/`fast` pins (`gpt-5.6-terra`, `gpt-5.6-luna`) are still accepted is what the
wave's non-deep codex dispatches show (see the research doc, F7).

## What the same wave shows about the other tiers

The loop log for wave 2 records codex launching successfully for scout (7 dispatches),
triage (2) and tdd (2) — the `balanced`/`fast` pins (`gpt-5.6-terra`, `gpt-5.6-luna`)
are still accepted. Only the `deep`/`top` pin is dead. That bounds the operator's
decision to one row of the tier table.

## Cost on this wave

Two lanes × ~25 minutes of wall-clock at the build phase, plus the fallback build on
Claude each. No tokens were burned during the stall (the pane was idle), but the wave's
build phases read 56–64 minutes instead of ~30.

## Operator notes

- `BRIDGE_EXIT_ARTIFACT_TIMEOUT` on a codex deep/top dispatch whose final pane holds a
  4xx JSON is this class. After this change it reads `BRIDGE_EXIT_UNKNOWN_PROMPT`
  (`escalate:model_unsupported`) within seconds of the error.
- The dashboard's per-phase duration is the fastest tell: a build that "takes" 25
  minutes with no token telemetry is a stalled pane, not a slow model.
