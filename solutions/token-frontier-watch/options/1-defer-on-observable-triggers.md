# Option 1 — DEFER both families and re-check on observable triggers

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

Both families stay out of the build queue. The watch becomes a concrete test rather than a calendar
reminder. The dossier names two triggers for the ollama family (T-A1: an ollama release ships persistent
KV or a public KV API; T-A2: at least 5% successful ollama-tmux dispatch share over 20 cycles) and three
for the latent family (T-B1 to T-B3: a fleet CLI imports compressed or latent context from another
model, a cross-vendor protocol, or a homogeneous self-hosted pair whose payload the auditor can read). The
next re-check, quarterly or on a trigger, runs three cheap probes: the state of PR #17953 (E5), the
ollama-lane dispatch share from telemetry (E1), and the CLI changelogs (E14).

The operator gets a re-check that is cheap and falsifiable. The pipeline spends nothing on a lane that
carries 0 successful dispatches (E2).

## Causal chain to the goal metric

The goal metric is per-agent token usage without an integrity loss. Its chain here is this: no build ⇒ no
source risk and no spent tokens ⇒ the watch reports a flip within one re-check interval once a trigger
fires ⇒ a BUILD item is queued only when a technique can move real tokens or latency.

## Quantified expected effect

- **Tokens and latency now:** 0 tokens and 0 ms saved, because the lane carries no successful traffic
  (E2). The strategy forgoes very little. The counterfactual ceiling is ≈768,792 unbilled local prefill
  tokens over 103 cycles, ≈0.06% of fleet tokens, and ≈2.0 s per triage dispatch at 8B (derived figures,
  A2, A6).
- **Cost:** about one small document cycle per quarter (A4).
- **Time-to-effect:** at most one re-check interval after a trigger fires. That lag is acceptable
  because the first move after a flip is still a spike (A5).

## Cost and time-to-effect

Four small document cycles a year. There is no source change and no new flag.

## Top risks and early detection

1. **A flip goes unnoticed between re-checks.** If PR #17953 merges or a vendor ships context import
   mid-quarter, we lose up to a quarter. *Detection:* the re-check protocol also lets a trigger seen in
   release notes pull the check forward, and the dossier names the exact search strings.
2. **The re-check becomes rote (DEFER forever).** *Detection:* each re-check must restate the measured
   share (E1) and the PR state (E5) with the date. If both numbers are unchanged twice in a row, the next
   re-check should consider Option 3 (retire).
