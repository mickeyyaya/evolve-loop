# Build Explanation — Cycle 1882

## Build Binding
- Cycle: 1882
- Base SHA: 795d4157243f9ddb978c38d3a081b50bf9c0cac4

## Summary
The modelquery decision-surface pin now hashes code, not comments. `TestDecisionVersion_PinnedToAlgorithmSurface` hashes each decision-surface file after `commentaudit.StripComments`, so a comment-only edit keeps the pin and a code edit still breaks it. The five decision-surface files no longer carry comments. The source-reading `TestOllamaListMetadataExceptionDocumented` is retired, and `TestOllamaLister_NilRunDefaultsToExecRunner` now asserts on the success path too.

## Rationale
The pin hashed raw bytes, so any comment edit moved it, and the comment-zero trains could not strip these five files without a re-pin (inbox item `modelquery-pin-hashes-code-not-comments`). `commentaudit.StripComments` already proves that its output differs from the input only in comments (`Equivalent`), so it is the existing, tested normaliser. The new pin was checked against the pre-strip files at the base SHA: they hash to the same value, so the strip changed no code and `decisionVersion` stays.

## Changed Areas
- `go/internal/modelquery/decisionversion_pin_test.go` — hashes `commentaudit.StripComments` output instead of raw bytes, and pins the new sha256.
- `go/internal/modelquery/classifier.go` — comments removed by `evolve comments strip`; the code is unchanged.
- `go/internal/modelquery/latest.go` — comments removed by `evolve comments strip`; the code is unchanged.
- `go/internal/modelquery/complete.go` — comments removed by `evolve comments strip`; the code is unchanged.
- `go/internal/modelquery/lineage.go` — comments removed by `evolve comments strip`; the code is unchanged.
- `go/internal/modelquery/newestwins.go` — comments removed by `evolve comments strip`; the code is unchanged.
- `go/internal/modelquery/ollama.go` — a failed `ollama list` error now carries the truncated command output. The bare `exit status 1` named no cause, and with this use of `out` the predicate's lost-listing mutant compiles.
- `go/internal/modelquery/ollama_classifier_test.go` — retires the source-reading test, compares the nil-run lister with a direct `ollama list`, and adds `TestOllamaLister_FailureCarriesTheCommandOutput`.
- `docs/architecture/model-discovery-and-catalog.md` — the recipe-engine picker line no longer lists agy, which `AgyLister` enumerates through `agy models`.
- `docs/architecture/packages/internal-modelquery.md` — the pin, ollama-lister and nil-run invariants and findings describe the new behaviour.
- `docs/history/code-comments/internal-modelquery.md` — records the one history-bearing comment that the strip removed (`dateRun` in `lineage.go`), as `evolve comments history` selected it.
- `go/acs/cycle1882/predicates_test.go` — the TDD phase's acceptance predicates for this cycle; unchanged by Build.
- `.evolve/evals/modelquery-pin-hashes-code-not-comments.md` — the cycle's eval; unchanged by Build.

## Design Decisions
The pin hashes the output of the existing `commentaudit.StripComments`. A new token-stream hasher was rejected: it would duplicate a tested normaliser. A go/printer AST hash was rejected too: it is a second normalisation that the strip tool does not use. The nil-run test compares the lister with a direct run of the same runner, so it holds whether ollama is absent, failing or listing models. An assertion of a non-empty list was rejected, because it fails on a host where ollama lists no models.

## Verification
`go test -count=1 ./internal/modelquery` passes. `go test -tags acs -count=1 ./acs/cycle1882` passes all nine predicates. The pin test passes on a copy of the base-SHA (pre-strip) files. The predicate's lost-listing mutant now fails the nil-run test on its assertion, not on a compile error.

## Compatibility
`decisionVersion` is unchanged, so no catalog entry is reclassified. The `ollama list` error keeps its `ollama list: <cause>` prefix and the wrapped error.

## Limitations
The pin still treats any code change, a pure refactor included, as a surface change that needs a manual re-pin.
