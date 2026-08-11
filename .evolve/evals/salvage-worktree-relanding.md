---
score_cap:
  - criterion: "A single unmatched (never-closing) backtick adjacent to a report's own verdict sentinel does not suppress it as a quoted echo"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestClassifyBadVerdict_UnmatchedBacktickFalsePositive ./internal/deliverable"
  - criterion: "A sentinel wrapped in balanced inline-code backticks is excised as a quoted echo, and the report's own tail sentinel is the one classified"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestClassifyBadVerdict_(QuotedDecoyCorpus|QuotedEchoStillSuppressed)' ./internal/deliverable"
  - criterion: "ClassifyBadVerdict never panics on a backtick flush against a content boundary (offset 0 or len-1)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestClassifyBadVerdict_BacktickAtContentBoundary ./internal/deliverable"
  - criterion: "The bad-verdict baseline sidecar has a production reader reachable from the CLI: `evolve salvage report` is registered in the dispatch table"
    max_if_missing: 5
    evidence: "cd go && go run ./cmd/evolve help 2>&1 | grep -q salvage"
  - criterion: "SummarizeBadVerdictBaseline folds the JSONL, filters foreign event types, and is named in an apicover assertion that executes it"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestSummarizeBadVerdictBaseline_NamesAndExercises ./internal/deliverable"
  - criterion: "A torn (unparseable) sidecar line is a loud error, never a silently skipped line that biases the measured rate"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1439_007 ./acs/cycle1439"
---

# Eval: Land the stranded salvage worktree (quote-aware classifier + `evolve salvage report`)

> This eval pins the payload of the cycle-1407 salvage landing, which sat
> unlanded across cycles 1407–1438 in `.evolve/worktrees/cycle-42824668-1407`
> (snapshot `04d3dee1`, continuation records `task-a-salvage-extraction-stage`
> and `task-b-decoy-sentinel-fixture`). Two behaviours are load-bearing and each
> guards against the naive fix for the other. First, `ownSentinelPayload` must be
> quote-AWARE and tail-ANCHORED: a sentinel a report merely quotes while
> discussing the contract is not that report's verdict (cycle-641 — classifiers
> MUST exclude verbatim echoes of injected instruction text — and the cycle-1298
> corpus, where five quoted decoys buried a real FAIL). Second, "quoted" must
> mean a backtick run that actually CLOSES: cycle-1407's adversarial finding F1
> showed `isQuotedEcho` trusting single-character adjacency, so one stray
> unmatched backtick excised a genuinely recoverable verdict and widened the
> error bars on the recoverable-malformed rate the `schema-aligned-salvage-layer`
> extraction stage is gated on.
>
> The third strand is reachability. `salvage_instrument.go` has appended to
> `.evolve/bad-verdict-baseline.jsonl` since cycle-1389 with no reader at all, so
> the gating number could only be produced by hand. `SummarizeBadVerdictBaseline`
> plus `evolve salvage report` is that reader — and it only counts if it is wired
> into `registry.go`'s dispatch table, since a reader whose only caller is a test
> is dead code. The rate must also be honest: a foreign `event_type` must not
> enter the denominator, and a torn append must fail loudly rather than deflate
> the measurement it exists to produce.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| unmatched-backtick-false-positive | One never-closing backtick must not suppress the report's own sentinel (finding F1) | 8/10 | `go test -run TestClassifyBadVerdict_UnmatchedBacktickFalsePositive ./internal/deliverable` |
| quoted-echo-excision | Balanced-backtick echoes excised; tail sentinel wins among survivors | 7/10 | `go test -run 'TestClassifyBadVerdict_(QuotedDecoyCorpus\|QuotedEchoStillSuppressed)' ./internal/deliverable` |
| boundary-safety | No panic when a backtick is flush at offset 0 / len-1 | 6/10 | `go test -run TestClassifyBadVerdict_BacktickAtContentBoundary ./internal/deliverable` |
| cli-reachability | `salvage` registered in the dispatch table — an unwired reader is dead code | 5/10 | `go run ./cmd/evolve help \| grep -q salvage` |
| reader-apicover | `SummarizeBadVerdictBaseline`/`BaselineSummary` exercised by a named apicover assertion (ADR-0069 repo-wide gate; `internal/deliverable` is enrolled) | 6/10 | `go test -run TestSummarizeBadVerdictBaseline_NamesAndExercises ./internal/deliverable` |
| torn-record-fails-loud | A corrupt sidecar line errors instead of being skipped (rule 12) | 7/10 | `go test -tags acs -run TestC1439_007 ./acs/cycle1439` |
