# Build Explanation — Cycle 2248

## Build Binding
- Cycle: 2248
- Base SHA: ae84e4ad2a78cc6767ce1fd13943401f10752624

## Summary
A new package, `internal/logcompress`, removes passing-test runs, download noise, repeated lines and large diff bodies from a log. Every failure line stays byte-exact, except inside a diff body, which collapses whole. A new `log_compression` policy schema (`go/internal/policy/policy_logcompress.go:9`) defines each threshold and pattern list. `.evolve/policy.json` does not hold the block yet, and the operator must add it.

## Rationale
Test-heavy phases carry bulky logs into prompts. A pure, line-based filter cuts the tokens with no model call. Keeping each line unmodified makes the failure counts provable. Putting all values in policy.json keeps Go free of thresholds.

## Changed Areas
- `go/internal/logcompress/logcompress.go` — adds `Compress` and `Result`: classify each line, group the elided lines, and refuse a cut under the ratio.
- `go/internal/logcompress/logcompress_test.go` — unit tests for repeats, short runs, marker size, hunkless diffs, protected diffs, the diff-body end and the missing final newline.
- `go/internal/logcompress/apicover_named_test.go` — names `Compress` and `Result` in an executing assertion.
- `go/internal/policy/policy_logcompress.go` — adds `LogCompressionPolicy`, `LogCompressionSettings` and `Policy.LogCompressionConfig`, which refuse an absent or invalid block.
- `go/internal/policy/policy.go` — adds the `log_compression` field to `Policy`.
- `go/internal/policy/logcompress_config_test.go` — tests the decoding and each refusal.
- `go/internal/policy/strict_keys_shape_test.go` — adds `LogCompressionPolicy` to the reviewed set of custom decoders.
- `go/.apicover-enforce` — enrolls `./internal/logcompress`.
- `docs/architecture/packages/internal-logcompress.md` — the design page of the package.

## Design Decisions
The config has no compiled default: an absent block is an error, as the predicate requires. A never-compress region runs from the marker line to the next blank line, because the predicate needs the text after the blank line to compress. A diff file is collapsed whole, so failure markers do not apply inside a diff body. `Compress` carries an `apicover:ignore` directive, because the predicate runs apicover without a cover profile.

## Verification
The 13 predicates TestC2248_001 to TestC2248_016 pass. The logcompress and policy packages pass under the race detector, and `go vet` is clean.

## Compatibility
The new policy key is optional for every existing reader. No existing behavior changes, because the filter has no caller yet.

## Limitations
The operator must add the `log_compression` block to `.evolve/policy.json`: the build phase cannot edit that control-plane file. The filter is not wired into any prompt, so the token soak (acceptance 3) is deferred. A multi-line repeated block is not collapsed.
