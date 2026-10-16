# internal/research

## Purpose

`internal/research` is the programmatic knowledge-base port. It gives the orchestrator and the advisor a typed and deterministic way to query the lessons corpus (`.evolve/instincts/lessons/*.yaml`) when they interpret a failure. This is the recall-memory tier of the three-tier memory model. The search was once shell-only (`kb-search.sh`, called by an agent). Here it is a Go capability with no LLM in the loop, because the ranking is pure code.

## Design

- **`Query` mirrors `core.Taxonomy`.** It has Source, FailureMode and Consequence, plus free keywords. The orchestrator can turn the verdict reason of a failed phase into a `Query`. The KB does not import `core`, so the dependency runs one way. `Source` matches `Lesson.FailedStep`.
- **Query terms.** `terms` returns the lowercase, de-duplicated search terms in input order. The first occurrence wins. `tokenize` drops tokens shorter than 3 characters as noise.
- **`KB` is a small read port with one method.** The orchestrator depends on the interface, and tests can fake it. An empty result (no corpus, or no match) is not an error. It is the signal that the failure is new, which is the trigger of the consolidation workstream (WS2).
- **`FileKB` reads from the file system.** It has no cache, because the corpus is small and a lookup happens at most once per cycle. It has no LLM, because the ranking is arithmetic. A root that does not exist is skipped at lookup time, and an absent corpus is normal.
- **Ranking is deterministic.** `score` gives a strong signal for an exact source match. It adds the token overlap between the query terms and the searchable text of the lesson. It weights the sum by the confidence of the lesson. A lesson with high confidence outranks one with low confidence and equal overlap.
- **Unscored lessons and ties.** A lesson with no confidence still counts, but weakly (`defaultConfidence`). Ties break by higher confidence and then by id, so the same corpus and query always give the same order. Lessons are de-duplicated by canonical `Lesson.ID` across the roots.
- **Recall bound.** `maxResults` is the built-in bound. It is enough for the recall section of the advisor, and it does not flood the prompt. The `research` block of `policy.json` can override it. The composition root resolves it from `policy.ResearchConfig().RecallK`.
- **Recall is a prefix.** `NewFileKBWithRecall` returns at most `recallK` lessons. They are the prefix of the same ranking, never a resample. A value of zero or less falls back to the built-in bound. A KB with zero recall can turn off the recall memory of the advisor without a signal. A typo of an operator must not cause that.
- **Zero-value recall.** A `FileKB` built as a struct literal has recall 0, which means the built-in bound and never none.
- **Corpus rot.** `parseLessonFile` returns an error for a malformed file, and never a silent empty result, so rot shows. `Lookup` skips such a file, because recall is best-effort and must not fail the cycle. `listLessonFiles` is a flat, non-recursive walk of the lessons directory. A missing root gives an empty list.
- **Search paths.** `SplitSearchPaths` splits a colon-separated value and drops empty entries, so the composition root can build a `FileKB` without a second parse. `SearchPathsFromEnv` reads the policy config and falls back to the documented default when `KBSearchPaths` is empty. It replaced the read of `EVOLVE_KB_SEARCH_PATHS`.
- **Digest.** `Digest` is a one-line summary for the recall section of the prompt. It uses `PreventiveAction` when it is set and `Description` otherwise. `firstSentence` cuts the text at its first sentence so that the digest stays on one line.

## Invariants

- **The lesson schema is pinned between producer and consumer.** `faillearn.RenderLessonYAML` (the fallback writer of lessons) and `parseLessonFile` (the only reader of the corpus) must agree. `faillearn_contract_test.go` is that guard, and it imports `faillearn` on purpose. Do not remove the import.
- **Why the guard exists.** The test lets `faillearn` stay a leaf and `research` the reader. It prevents a silent divergence between two copies of one schema. If either side changes shape, this test fails first.
- **How to update the guard.** `expectedLessonSchemaFields` counts the 8 scalar fields plus `failureContext`. When the count drifts, a field was added to `lessonYAML` that `faillearn` may not write, or the reverse. Update the twin schema struct in `faillearn` and the assertions together. The fixture mirrors `fixtureEvent` in `faillearn_test.go`, because a test helper cannot be shared across packages.
- **The recall bound is a prefix.** Pinned by `TestFileKB_ConfiguredRecallTakesRankingPrefix`. The default is 5, pinned by `TestFileKB_DefaultConstructorRecallUnchanged`.
- **A non-positive recall falls back.** Pinned by `TestFileKB_NonPositiveRecallFallsBackToDefault` and `TestFileKB_ZeroValueRecallStillRecalls`.
- **`*FileKB` implements `KB`.** The orchestrator holds a `research.KB` that `NewFileKB` fills. Pinned by `TestFileKB_SatisfiesKBInterface`. A `FileKB` with no roots returns no lesson and no error.

## Findings

- None.
