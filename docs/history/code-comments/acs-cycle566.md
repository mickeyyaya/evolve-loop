# Comment history: `acs/cycle566`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle566/predicates_test.go:3` — above `package cycle566`

```text
// Package cycle566 materialises the cycle-566 acceptance criteria for the
// fleet-lane-committed top_n task `per-phase-effort-routing` (inbox weight 0.88;
// see triage-report.md and .evolve/inbox/2026-07-05T15-10-00Z-per-phase-effort-
// routing.json).
//
// Committed scope (triage top_n): the PLUMBING slice only — add an abstract
// `effort` (low|medium|high) dimension to LaunchIntent, realize it per-manifest to
// each CLI's native mechanism (claude effort flag, codex reasoning_effort;
// agy/ollama noop), and pin per-phase defaults in config. Retry-escalation and
// telemetry/soak validation are EXPLICITLY deferred to a follow-up cycle.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549/553/555/557/561/
// 563/565 precedent). Each predicate shells `go test`/`go vet` over the RED unit
// tests authored this cycle in go/internal/bridge (effort_routing_test.go) and
// go/internal/profiles (effort_defaults_test.go). RED now — the bridge tests do
// not compile until Builder adds LaunchIntent.Effort + realizeScalar("effort",…) +
// the manifest params.effort entries, and the profiles matrix asserts values the
// shipped config does not yet carry. GREEN once effort is wired and the per-phase
// defaults are aligned.
```
