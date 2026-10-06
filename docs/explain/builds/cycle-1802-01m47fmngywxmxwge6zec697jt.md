# Build Explanation — Cycle 1802

## Build Binding
- Cycle: 1802
- Base SHA: 073cb31efffdbea89375464c1c12a37923b61d40

## Summary
Gate E (`unsatisfiable-predicate-shape`) now rejects a tdd deliverable at the `eval_gate` enforce stage when one of the cycle's own predicates has an `inverted-idiom` or `go-run-exit-code` finding. Each of those kinds proves the predicate red on every tree. The rejection reason names the predicate, the kind and the remedy, lists the blocking findings ahead of the five-finding cap, and ends `BLOCKING at enforce: N of the M finding(s), listed first`. The heuristic `absence-message` kind and every Gate D flaky-shape finding stay advisory, and at shadow every kind is logged with `blocking=false` and the deliverable is approved.

## Rationale
Commit dd8954ad9 landed the lint and Gate E with `block` as a constant `false`. A cycle's own unsatisfiable predicate was therefore only logged, and the 1488, 1788 and 1793 class went on to burn whole builds. The two proof kinds are the same two that `TestLintUnsatisfiablePredicates_GoAcsCorpusHasNoProofKindFinding` already fails on across the committed corpus, so making them block at tdd applies a judgment the repo already holds, at the earliest point a predicate exists. `absence-message` is a keyword heuristic, so a false positive must not cost a cycle. The Gate D classes have no per-class false-positive breakdown yet.

## Changed Areas
- `go/internal/evalgate/predicatelint.go` — a finding is now a `predicateLintFinding` (its text plus a `blocking` flag). `check` returns `block=true` only when the findings path holds a blocking finding. `findingsReason` sorts blocking findings first, so the cap never hides them, and swaps the `ADVISORY: never blocks` tail for a `BLOCKING at enforce` tail when any finding blocks.
- `go/internal/evalgate/unsatisfiable.go` — Gate E's strategy marks findings whose kind is in `unsatisfiableProofKinds` (`inverted-idiom`, `go-run-exit-code`) as blocking, keyed on the exported `evalqualitycheck` kind constants rather than new string literals.
- `go/internal/evalgate/flakyshape.go` — Gate D's strategy builds the new finding type with `blocking` left false, so its never-blocks behavior is unchanged.
- `go/internal/evalgate/predicatelint_test.go` — the byte-for-byte wording test now carries each gate's findings verdict and blocking expectation. `TestPredicateLintGate_ListsBlockingFindingsAheadOfTheCap` pins the ordering and the tail.
- `go/internal/evalgate/unsatisfiable_test.go` — the old never-blocks tests are replaced by `TestUnsatisfiableShapeGate_BlocksOnlyTheProofKinds` (the three kinds, table-driven) and `TestNewReviewer_ProofKindRejectsAtEnforceAndIsOnlyLoggedAtShadow` (the stage dial, through `NewReviewer`).
- `docs/architecture/packages/internal-evalgate.md` — the template-method, Gate E and invariant bullets now state the per-finding blocking contract and drop the constant-false claim.
- `docs/architecture/acs-predicate-quality-gate.md` — Layer 6 gains its stage paragraph (proof kinds block at enforce, the heuristic and Gate D stay advisory, and shadow logs every kind), and the pipeline diagram is updated to match.
- `docs/operations/runtime-reference.md` — the `quality-check` hand-check entry notes that Gate E rejects the proof kinds at enforce while the CLI stays advisory.

## Design Decisions
The blocking decision belongs to each finding, set by the gate's lint strategy, not to the gate as a whole. This keeps one `predicateLintGate` template for Gates D and E, and a later promotion of a single flaky class becomes a one-line strategy change. Blocking findings sort ahead of advisory ones rather than raising the cap, so the reason stays bounded. An alternative was a gate-level `blocks(kind)` hook, but it would have needed the kind re-parsed from the formatted text. No `remediator` was added. The reason already names the predicate, the kind and the lint's remedy, and a `Remediation` value also switches on contract-escalation behavior in `core`, which is outside this task.

## Verification
`go test -tags acs -count=1 ./acs/cycle1802` passes 7/7: proof kinds rejected with the finding and remedy, the proof finding named beyond the advisory cap, absence-message, flaky, clean and absent packages approved, and shadow approving every kind. `go vet ./internal/evalgate && go test -race -count=1 ./internal/evalgate` passes. In a mutant that forced the proof kinds non-blocking, the three updated unit tests failed.

## Compatibility
Shadow behavior is unchanged. At enforce, the only new rejection is a tdd deliverable whose own cycle predicate has a proof-kind finding. The committed corpus holds none, and the corpus test already enforces that. The `evolve eval quality-check -predicates` CLI is unchanged and still raises PASS→WARN.

## Limitations
`absence-message` and the Gate D classes stay advisory until their false-positive rates are measured per kind. A finding's proof holds only within the lint's scope tracker, as Layer 6 documents, so a rebinding through a pointer or callee could still produce a false block, though none exists in the corpus.
