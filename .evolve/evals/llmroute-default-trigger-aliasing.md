---
score_cap:
  - criterion: "Editing one resolved Plan's Triggers (element write or append through a reslice) never changes a later Plan's Triggers or DefaultTriggers, for both Resolve and ChainFor"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1821_001_' ./acs/cycle1821"
  - criterion: "A nil profile, a profile with a nil cli_fallback_on_exit and one with an empty list each get a private copy of the default trigger set, while a profile's own list still wins and stays untouched"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1821_002_' ./acs/cycle1821"
  - criterion: "Decisions from the production cliroute router (dispatch launch through Resolve, advisor launch through ChainFor) do not share a Triggers backing array"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1821_003_' ./acs/cycle1821"
  - criterion: "ApplyUniversalFallback takes no unused lookPath parameter and no llmroute function carries an unused parameter, while the discovered tail still appends deduped"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1821_004_' ./acs/cycle1821"
  - criterion: "ApplyBench and ApplyDriverBench delegate to one bench implementation parameterized by its key, with family and driver demotion behaviour unchanged"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1821_005_' ./acs/cycle1821"
  - criterion: "The llmroute package stays green under the race detector, formatted and vetted"
    max_if_missing: 8
    evidence: "cd go && go vet ./internal/llmroute/ && test -z \"$(gofmt -l internal/llmroute)\" && go test -count=1 -race ./internal/llmroute/"
---

# Eval: llmroute hands every Plan its own trigger slice

> Pins the repair of inbox item `llmroute-default-trigger-aliasing` (found by
> comment-reduction batch 20, 2026-09-26, shipped in cycle 1821). When a profile
> set no `cli_fallback_on_exit`, `resolveTriggers` returned the package-level
> `defaultFallbackOnExit` slice itself, so a caller that wrote to or appended
> through `Plan.Triggers` rewrote the default for every later `Resolve` and
> `ChainFor` in the process, a defect that would surface in some other cycle.
> The same item dropped `ApplyUniversalFallback`'s unused `lookPath` parameter
> and folded the near-duplicate `ApplyBench` / `ApplyDriverBench` into one
> function keyed by a strategy. Prescription 4 (Family() in the overlay rung)
> moved to `bridge-family-manifest-facade` and is not pinned here. The cycle's
> git-state guards (no comments added, sizeratchet offenders untouched;
> `TestC1821_006_`, `TestC1821_007_`) diff against the cycle's merge-base and are
> cycle-scoped, so they are deliberately not replayed as permanent evidence.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| alias-element-and-append | Edits to one Plan's Triggers never leak into another | 8/10 | `go test -tags acs -run TestC1821_001_ ./acs/cycle1821` |
| edge-empty-profiles | nil / nil-list / empty-list profiles get a private copy | 7/10 | `go test -tags acs -run TestC1821_002_ ./acs/cycle1821` |
| production-caller | cliroute dispatch and advisor Decisions do not share Triggers | 7/10 | `go test -tags acs -run TestC1821_003_ ./acs/cycle1821` |
| no-unused-param | ApplyUniversalFallback has no lookPath parameter | 6/10 | `go test -tags acs -run TestC1821_004_ ./acs/cycle1821` |
| one-bench | ApplyBench / ApplyDriverBench share one keyed implementation | 6/10 | `go test -tags acs -run TestC1821_005_ ./acs/cycle1821` |
| package-health | llmroute green, vetted, formatted | 8/10 | `go test -race ./internal/llmroute/` |
