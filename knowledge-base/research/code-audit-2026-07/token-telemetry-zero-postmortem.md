# Post-mortem: token telemetry all-zeros on its first full-chain batch (2026-07-08)

> Companion to `docs/plans/token-telemetry-2026-07.md` (the campaign design). Outcome of the first real measurement, batch `bkm5wr3ey` (cycles 612–619), the first batch run on a binary containing all of S1–S7.

## What was expected vs observed

Pre-declared acceptance (design doc + boundary plan): `evolve tokens report --last 8` shows non-zero per-phase tokens with `source=transcript` for claude-tmux phases, advisor calls attributed, per-attempt waste visible.

Observed: report **structure fully correct** — 16 phases enumerated, per-phase cycle counts right (audit 17, ship 16 reflecting the reaudit loops), waste and cache-ratio lines rendered — but **every token count zero**, and `find .evolve/runs -name llm-calls.ndjson` → **no file exists anywhere**.

## Diagnosis (three probes, ~5 minutes)

1. `evolve tokens report` → structure-correct-but-zero ⇒ S6/S7 projection+CLI work; the zeros enter upstream.
2. `find` for `llm-calls.ndjson` → none ⇒ the S3 per-Launch appender never executed (it writes unconditionally when telemetry is active, once per attempt, even on failures).
3. `<phase>-usage.json` sidecar → **has** the `tokens{...}` block (S4 projection wired, schema present) but zeros ⇒ `BridgeResponse.Tokens` was never populated ⇒ the Launch-side instrumentation is dormant, not broken.
4. `grep -rn TokenResolver go/ --include='*.go' | grep -v _test` → **only** `internal/bridge/engine.go:157,166,527,531`.

## Root cause

`Deps.TokenResolver` is a DI seam (deliberately not a policy toggle, per the no-feature-flags rule). The engine.go doc comment states the contract: *"the orchestrator building the bridge Deps wires this to the shipped tokenusage.Chain(TranscriptCollector/EventsResultCollector/ScrollbackPeakCollector)… withDefaults leaves it nil (telemetry disabled)."*

**Nobody wires it.** Neither composition root — `adapters/bridge` (engine factory used by phase runners, advisor, retro, swarm) nor `subagent/validateprofile.go` (the direct-construction adapter-bypass path the design doc itself flagged) — injects the chain. S1–S7 shipped the scanner, the chain, the seam, the appender, the projections, the rollups, and the CLI — everything except the one line that turns it on. Tests pass because they inject stubs at the seam, which is exactly what the seam is for.

## Why it was invisible for a full batch

The resolver is **fail-open by design** (a telemetry error must never fail a Launch) and **nil is the documented "disabled" state**. So missing wiring is byte-for-byte indistinguishable at runtime from working-but-quiet telemetry. No log line, no error, no gate. The scan filed the general class the same day (`selfcheck-breaker-fail-loud`: fail-open posture applied where silence hides a dead wire) — this is the same disease at the composition layer.

## Lessons (general, not just this campaign)

1. **A DI seam whose nil-default silently disables a shipped feature needs a composition-root test** ("production factory yields non-nil X") — unit tests with injected stubs can never catch the missing injection.
2. **Fail-open needs one loud line.** Fail-open ≠ silent: a single boot-time WARN ("token telemetry disabled: no resolver") converts an invisible gap into a greppable fact without violating the never-fail-the-Launch contract.
3. **"Chain complete" ≠ "chain connected."** Slice-by-slice campaigns must include an explicit final wiring slice with an end-to-end acceptance probe; S1–S7 each PASSed their own acceptance while the system-level acceptance (non-zero report) had no owner until the boundary check caught it.
4. **Pre-declared falsifiable acceptance pays off**: "if still zeros at the next boundary, that becomes a defect item with a ready hypothesis ledger" made this a 5-minute diagnosis instead of a debugging session.

## Fix

Filed `token-resolver-production-wiring` (weight **0.96**, top of queue for batch begun 2026-07-08): a shared `tokenusage.DefaultResolver(configRoot)`-style constructor wired into **both** composition roots (single-source so they can't drift), composition-root non-nil tests for both paths, one fixture-driver Launch → exactly one `llm-calls.ndjson` record, and the boot-time nil WARN. Batch-level acceptance: next boundary's `evolve tokens report` shows non-zero `source=transcript` tokens.
