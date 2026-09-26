# Option 2 — BUILD an ollama-lane KV-persistence spike now

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

The pipeline queues a spike that makes the self-hosted lane reuse prefill state across model unloads.
There are two technical routes, and both need source changes:

- run a patched ollama with `OLLAMA_PREFILL_CACHE=1` from the open PR #17953 (E5), or
- switch the lane's server to llama-server with `--slot-save-path` and the slot save/restore endpoints
  (E6). The driver would also change from the `ollama run` REPL to HTTP calls (E3).

A soft-prompt follow-on (500xCompressor or gist tokens) would also need a compressor trained per target
model and KV injection. Ollama has no path for either (E4, E9).

The beneficiary would be any reasoning or review phase routed to ollama. Today there are none (E2).

## Causal chain to the goal metric

Prefill reuse ⇒ lower TTFT on reloads ⇒ faster ollama phases ⇒ some phases could move off billed lanes
⇒ fewer billed tokens. Every link after the first assumes traffic on the lane. The measured share is
0.08%, with 0 successful dispatches (E1, E2).

## Quantified expected effect

- **Latency:** ≈0.76 s (3B) to ≈2.0 s (8B) per triage-sized dispatch, which is 0.6%–1.6% of the
  129.8 s median dispatch. This counts only reloads. With `keep_alive: -1`, in-memory reuse already
  covers the resident case, so the marginal win falls toward 0 s (derived figures, A1, A2, A6).
- **Tokens:** no billed tokens are saved, because the lane is local. At one counterfactual dispatch per
  cycle, it avoids ≈7,464 local prefill tokens per dispatch. That is ≈0.06% of fleet tokens (E7, E8).
- **Realized today:** 0 tokens and 0 ms, because nothing is routed (E2).

## Cost and time-to-effect

A spike of one or two build cycles plus a bridge-driver change. It is gated on either maintaining a fork
of an unmerged PR (E5) or swapping the server (E6). Time-to-effect is 0 until some profile routes a
phase to ollama. First the bad_flags launch failure (E2) would have to be root-caused.

## Top risks and early detection

1. **We maintain a fork of an unmerged, deprioritized PR** (E5). *Detection:* upstream drift breaks the
   patch on the next ollama release.
2. **Engineering spent on a lane with no traffic.** *Detection:* the E1 telemetry query run 20 cycles
   after the spike ships still shows ≈0% successful ollama dispatches.
