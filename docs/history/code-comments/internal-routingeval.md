# Comment history: `internal/routingeval`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d12

### `go/internal/routingeval/corpus_test.go:13` — above `func TestGoldenRoutingCorpus_LoadsAndValidates(t *testing.T) {`

```text
// WS4-S1 (ADR-0052): the corpus loads + validates. A malformed case file is a
// LOUD error, never a skipped case.
```

### `go/internal/routingeval/corpus_test.go:89` — above `func TestGoldenCorpus_ReplayProducesExpectedRunSet(t *testing.T) {`

```text
// WS4-S2 (ADR-0052): replay each captured response through the REAL parse +
// integrity-floor clamp (core.ReplayPlanFromResponse — the WS3-S5 entry point)
// and assert the run-set, the clamps that fire, the phases that must never run,
// and the universal ship⇒audit safety invariant. This locks the parse + floor:
// a regression that weakened either would change a run-set and break a case.
```

### `go/internal/routingeval/routingeval.go:1` — above `package routingeval`

```text
// Package routingeval is the golden routing-eval corpus (ADR-0052 WS4): a
// curated set of captured advisor responses, each replayed through the SAME
// parse + integrity-floor clamp the live planning path runs
// (core.ReplayPlanFromResponse, WS3-S5) and asserted against an expected
// run-set, the clamps that must fire, and the phases that must NEVER run. It
// locks the parse + floor against the real entry point, so a regression there
// (a parse change, a floor weakening) breaks a corpus case rather than silently
// shipping an unsafe plan.
//
// Leaf package: imports router + config + stdlib only. The replay assertion
// (which calls core) lives in the test, so routingeval itself never imports
// core — same isolation rule routingtest documents. Nothing in production
// imports it; only *_test.go consumers do.
```
