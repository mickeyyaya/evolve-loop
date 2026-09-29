# Build Explanation — Cycle 1752

## Build Binding
- Cycle: 1752
- Base SHA: aedac036be451da3dd8e017ce1aadd3b657eab96

## Summary
Two functions that sat over the 50-line size ratchet now fit under it without any behavior change. `fleet.PartitionGraph` dropped from 53 to 39 lines: its owning-bucket resolution moved into a new helper, `resolveOwningBuckets`. `committedset.DispositionsFrom` dropped from 57 to 40 lines: its anonymous decision struct became a named package type, `answeringBuckets`. This build adds characterization tests that kill the 14 one-line mutants the baseline suites let survive, 7 for each function. Predicates 010 and 011 prove the kill by running the current suites against each mutant of the baseline function.

## Rationale
The size ratchet lists both functions in `offenders.json` as allowances, and an allowance is a ceiling. Shrinking a function below its allowance is accepted as-is, so the file stays untouched. The triage action named the owning-bucket block as the one to extract from `PartitionGraph`. It is one self-contained computation over (pkgs, owner, buckets, isGZ, gzBucket), so a pure function is the natural seam. For `DispositionsFrom`, the eval states that hoisting the decision struct is an equally valid shrink. It moves 18 lines of type declaration out of the function and leaves the four bucket loops readable in one place, where splitting them into four helpers would scatter the precedence order across several call sites. Both functions keep their names, signatures and every comment, including the two trailing comments on `owner` and `gzBucket`.

## Changed Areas
- `go/internal/fleet/packagegraph.go` — `PartitionGraph` now calls the new `resolveOwningBuckets`, which holds the moved package-owner lookup and global-zone claim block unchanged, so the function fits the ratchet.
- `go/internal/fleet/partition_test.go` — adds characterization tests pinning the n<1 clamp, the wrapped and nil-result failure path, global-zone bucket claiming and pull-in, two-bucket deferral and ownership following the chosen bucket, so the refactor is guarded against the baseline's surviving mutants.
- `go/internal/committedset/committedset.go` — `DispositionsFrom` decodes into the hoisted `answeringBuckets` type instead of an inline anonymous struct, with the same JSON tags, so the function fits the ratchet.
- `go/internal/committedset/dispositions_test.go` — adds tests pinning that a well-formed decision with one mistyped field answers for nothing, plus id, evidence and sha trimming, blank-reason and negative-fail_count fallbacks and reason-over-fail_count precedence.
- `go/acs/cycle1752/predicates_test.go` — the TDD phase's 16 cycle predicates (size, real extraction, ceiling untouched, mutation kill, differential equivalence, scope, explanation wording), shipped with the cycle that satisfies them.
- `.evolve/evals/shrink-partitiongraph-fleet.md` — the TDD phase's eval contract for the PartitionGraph shrink, shipped with the cycle.
- `.evolve/evals/shrink-dispositionsfrom-committedset.md` — the TDD phase's eval contract for the DispositionsFrom shrink, shipped with the cycle.

## Design Decisions
`resolveOwningBuckets` takes its five inputs as plain parameters and returns a fresh set, so it reads no loop state implicitly and mutates nothing it is given. `answeringBuckets` is unexported and field-for-field identical to the old anonymous struct, so decoding, including partial fills on a type error, is byte-for-byte the same. The helpers the baseline calls (`splitGlobalZone`, `TransitivePackageSet`, `leastLoaded`, `only`) stay in place, so the predicates' baseline-overlay probes still compile.

## Verification
`go test -tags acs -count=1 ./acs/cycle1752` passes 16/16. That includes killing all 7 mutants of each baseline function, and differential equivalence against the baseline over an 18-scenario table (PartitionGraph) and 11,034 generated decision bodies (DispositionsFrom). From the module root, `gofmt -l .` is empty, `go vet ./...` is clean and `go test -count=1 ./...` reports 246 ok with 0 FAIL.

## Compatibility
No exported identifier, signature, JSON tag or error string changed. `offenders.json` is byte-unchanged and still lists both functions at their old allowances.

## Limitations
The allowances in `offenders.json` are left at 53 and 57, so the ratchet still permits both functions to regrow up to those ceilings until a later cycle tightens them. The third top_n card, `shrink-runcommitgate-guardcmd`, was escalated by the host (protected surface) and is not part of this build.
