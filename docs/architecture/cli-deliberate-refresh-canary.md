# CLI Deliberate-Refresh & Canary — Design Note (ADR-0048 §non-goals)

- **Status:** Design note (deferred to a focused, integration-gated increment — not built now).
- **Date:** 2026-06-13
- **Relates to:** [ADR-0048](adr/0048-work-conservation-fast-reland-resilient-ship.md) §non-goals ("cli-deliberate-refresh-and-canary needs live-CLI execution; build with a driver seam so the decision logic is unit-testable and only the live drive is integration-gated"), `internal/clihealth` (the bench/canary substrate — `Store.Expired()` already returns "canary candidates"), the cli-pin / cli-version-freeze preflight (ADR-0044 S3).

## Why a note, not code (yet)

The deliverable has two halves with very different testability:

1. **Decision logic — unit-testable, but under-specified.** "When should a CLI be deliberately refreshed or canaried?" The substrate already exists: `clihealth.Store.Expired()` returns families whose bench window has lapsed. These families are the natural canary candidates.

   A `clihealth.Bench` records a strike. An expired bench means "the CLI was benched a while ago; has it recovered?" The decision logic is roughly: *for each expired-bench family, schedule ONE canary before routing real work back to it.*

   The precise triggers (version-change-detected, bench-expiry or operator-forced refresh) and the back-off policy are not pinned. Coding it now is speculative (YAGNI).

2. **The live canary drive — integration-gated.** A canary must drive a REAL generation on the candidate CLI to prove recovery. The generation uses a tiny prompt, and the canary asserts a coherent response.

   A unit test cannot do this. It needs a live CLI and belongs behind an integration gate (like the other live-CLI validations).

If you build (1) without a spec and (2) without the live harness, you produce a half-feature on a sensitive path (CLI routing). The honest increment is to spec it first.

## Sketch for the focused increment

- **Seam:** `type CanaryDriver interface { Probe(ctx, cli string) (ok bool, detail string, err error) }`. A real impl drives one bounded generation with the existing `*-tmux` recipe. A fake impl returns scripted results for unit tests.
- **Decision (pure, unit-testable):** `canary.Plan(expired map[string]clihealth.Entry, now, lastCanary map[string]time.Time) []string` → the families to canary this cycle, with a min-interval back-off so a flapping CLI isn't canaried every cycle.
- **Wiring:** at the cycle boundary (near the bench snapshot the router already takes), run `Plan`. For each result, call `CanaryDriver.Probe`. On `ok`, call `Store.Clear(family)` (un-bench, so the family is routable again). On failure, re-`Bench` with a fresh window. The flag `EVOLVE_CLI_CANARY=off|shadow|enforce` gates this. In shadow mode, the system logs the canary that it plans to run.
- **Tests:** `Plan` unit tests (expiry + back-off); a fake-driver integration test for the clear/re-bench wiring; the live `Probe` behind the integration tag.

## Deferral rationale

This follows the discipline of the ADR-0047 Stage 4 finding. A sensitive path (CLI routing/recovery) earns a corpus/live-validated increment. It does not earn a speculative bolt-on at the tail of a multi-item session.

The substrate (`clihealth.Expired`) is ready. The spec and the live harness are the work.
