# CLI Deliberate-Refresh & Canary — Design Note (ADR-0048 §non-goals)

- **Status:** Design note (deferred to a focused, integration-gated increment — not built now).
- **Date:** 2026-06-13
- **Relates to:** [ADR-0048](adr/0048-work-conservation-fast-reland-resilient-ship.md) §non-goals ("cli-deliberate-refresh-and-canary needs live-CLI execution; build with a driver seam so the decision logic is unit-testable and only the live drive is integration-gated"), `internal/clihealth` (the bench/canary substrate — `Store.Expired()` already returns "canary candidates"), the cli-pin / cli-version-freeze preflight (ADR-0044 S3).

## Why a note, not code (yet)

The deliverable has two halves with very different testability:

1. **Decision logic — unit-testable, but under-specified.** "When should a CLI be deliberately refreshed or canaried?" The substrate already exists: `clihealth.Store.Expired()` returns families whose bench window has lapsed — the natural canary candidates. A `clihealth.Bench` records a strike; an expired bench means "the CLI was benched a while ago; has it recovered?" The decision logic is roughly: *for each expired-bench family, schedule ONE canary before routing real work back to it.* But the precise triggers (version-change-detected vs bench-expiry vs operator-forced refresh) and the back-off policy are not pinned, so coding it now would be speculative (YAGNI).

2. **The live canary drive — integration-gated.** A canary must drive a REAL generation on the candidate CLI (a tiny prompt → assert a coherent response) to prove recovery. That cannot be a unit test; it needs a live CLI and belongs behind an integration gate (like the other live-CLI validations).

Building (1) without a spec, and (2) without the live harness, would produce a half-feature on a sensitive path (CLI routing). The honest increment is to spec it first.

## Sketch for the focused increment

- **Seam:** `type CanaryDriver interface { Probe(ctx, cli string) (ok bool, detail string, err error) }`. A real impl drives one bounded generation via the existing `*-tmux` recipe; a fake impl returns scripted results for unit tests.
- **Decision (pure, unit-testable):** `canary.Plan(expired map[string]clihealth.Entry, now, lastCanary map[string]time.Time) []string` → the families to canary this cycle, with a min-interval back-off so a flapping CLI isn't canaried every cycle.
- **Wiring:** at the cycle boundary (near the bench snapshot the router already takes), run `Plan`; for each, call `CanaryDriver.Probe`; on `ok`, `Store.Clear(family)` (un-bench → routable again); on failure, re-`Bench` with a fresh window. Gated `EVOLVE_CLI_CANARY=off|shadow|enforce`, shadow logs would-canary.
- **Tests:** `Plan` unit tests (expiry + back-off); a fake-driver integration test for the clear/re-bench wiring; the live `Probe` behind the integration tag.

## Deferral rationale

Same discipline as the ADR-0047 Stage 4 finding: a sensitive path (CLI routing/recovery) earns a corpus/live-validated increment, not a speculative bolt-on at the tail of a multi-item session. The substrate (`clihealth.Expired`) is ready; the spec + live harness are the work.
