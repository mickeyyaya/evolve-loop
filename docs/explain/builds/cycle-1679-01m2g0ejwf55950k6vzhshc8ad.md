# Build Explanation — Cycle 1679

## Build Binding
- Cycle: 1679
- Base SHA: ca0a99268fc4ddf14b88523bfb70a609f1de1a5f

## Summary
The cross-artifact metamorphic invariant stack now carries both halves of its
acceptance contract: the advisory four-verifier aggregate (landed as code in
cycle-1676 and preserved on this branch) plus the experience record its own inbox
record requires, and the one real defect the cycle-1676 adversarial audit found in
it is fixed.

## Rationale
The inbox record `crossartifact-invariant-stack` names two deliverables in one
`fix` field — the advisory invariant stack, and "DOCS per 3.8 into
docs/research/deliverable-alignment-2026-08/README.md" — and its `connects_to`
lists both homes. Only the code half had landed, so the item's acceptance stayed
unmet while the portfolio README kept describing a shipped mechanism as unbuilt
work (`partial (coherence)` in the §4 layer model, `NEW — filed 0.85` in the §5
ranked portfolio). Writing the §6.4 record and refreshing exactly those two rows
is the smallest change that satisfies the unmet half, and the smaller of the two
candidate readings: the alternative — treating the record as satisfied by the code
and closing the item — would leave a landed L3 mechanism undiscoverable from the
portfolio it was filed against, which is the scattered-invariant problem the move
exists to end.

The same cycle applies the parent audit's prescription rather than deferring it.
The referenced-paths verifier resolved a cited evidence path by joining it to a
root and stat'ing the result; because `filepath.Join` cleans `../` segments away,
a citation with enough climbing segments landed on a real file outside both roots
and reported `ok`. A containment check before the stat is a five-line fix whose
absence silently weakens one of the four verifiers, so it belongs in the cycle that
documents them, not in a successor.

## Changed Areas
- `docs/research/deliverable-alignment-2026-08/README.md` — adds §6.4, the
  Issue/Gap/Solution/Measured experience record for item rank 5 in the shape §6.1–6.3
  use, and refreshes the two rows that contradicted on-disk reality: the §4 L3
  Verification row (the stack is landed advisory, no longer `partial`) and the §5
  rank-5 queue state (landed, with the production caller named). The record states
  explicitly that the live false-positive rate is still unmeasured, because that
  figure is the only thing that can graduate an invariant to blocking.
- `go/internal/coherence/crossartifact.go` — the advisory aggregate over a cycle's
  own artifacts: verdict agreement between the embedded sentinel and the standalone
  `acs-verdict.json`, an independent recount of the claimed suite counts, existence
  of every cited evidence path, and forward phase/provenance order. Changed this
  cycle: `resolvesUnder` rejects a cited path that escapes its root before stat'ing
  it, so a `../`-climbing citation can no longer resolve onto a file the lane never
  wrote.
- `go/internal/coherence/crossartifact_test.go` — behavioural coverage for the four
  classes over real on-disk fixtures, plus the escape case added this cycle, which
  fails against the pre-fix `resolvesUnder` with `status = ok, want violated`.
- `go/internal/core/crossartifact_invariants.go` — the single seam that records the
  aggregate into `<workspace>/crossartifact-invariants.json`, best-effort and loud:
  a write failure warns and is otherwise ignored, because an advisory observer must
  never be able to fail a cycle.
- `go/internal/core/cyclerun.go` — the one production caller, inside `finalizeCycle`
  and deliberately ahead of the ADR-0072 verdict-coherence floor, bound to the lane
  worktree rather than the project-root argument.
- `go/internal/core/crossartifact_invariants_wiring_test.go` — pins that seam
  through the real `finalizeCycle`: the artifact is emitted, a violation never
  blocks, the lane worktree is the bound tree, and the ADR-0072 floor is unchanged
  by the advisory running beside it.
- `go/acs/cycle1679/predicates_test.go` — this cycle's eight acceptance predicates
  (TDD-owned, unmodified here): 001-003 pin the landed code half so it cannot rot,
  004-005 pin the documentation half this cycle writes, and 006-008 were added across
  the cycle's two audit-repair rounds — 006 that the added eval and the predicate
  grading it agree in the shipped tree, 007 that every added Go test package is green
  before the repo-contract backstop sees it, 008 that this document's own arithmetic
  matches the tree it describes.
- `go/acs/cycle1676/predicates_test.go` — the parent attempt's predicates, preserved
  with the work they grade rather than dropped on continuation.
- `.evolve/evals/crossartifact-invariant-stack.md` — the permanent regression eval
  for the item. Its four `score_cap` criteria cover both halves of the contract, and
  a `## Acceptance Criteria (code-graded)` section carries five executable `[code]`
  graders so the eval is behavioural rather than frontmatter-only; every grader is
  narrowed with `-run` to named tests, because sweeping `./internal/core` whole is the
  known cycle-1173/1175/1178 false-red shape.
- `go/internal/phases/ship/repocontract.go` — the ship-time repo-contract gate now
  hands `go test` an explicit `-timeout 20m` (`repoContractTestTimeout`) instead of
  leaving every pack on Go's 10m default, and the argv construction is split into
  `repoContractTestArgs` so the flag is assertable without exec'ing the toolchain.
  This is the fix for THIS cycle's own first ship failure: the wiring test above
  enrolled `./internal/core` into the added-test backstop for the first time, that
  package measured 355.8s under fleet load in the run that did pass, and when the
  10m deadline wins instead, the timeout panic makes `go test -json` emit a fail
  event for the running test and for every `t.Parallel()` test still paused — the 19
  named `internal/core` "failures" (18 of them parallel) the gate classed a real
  contract RED on green code.
- `go/internal/phases/ship/repocontract_importers.go` — the scanner-pack and
  importer-backstop log lines now echo the `-timeout` they actually pass, so the
  `ship-repocontract-scan.log` an auditor reads still transcribes the real command.
- `go/internal/phases/ship/repocontract_addedtest_defects_test.go` — two regression
  tests for that flag: the argv carries `-timeout` ahead of the package operands with
  real headroom over Go's 10m default, and the tag-grouped argv shape the `//go:build
  acs` predicate packages depend on is unchanged. Both are RED against the pre-fix
  argv and against a no-headroom value.
- `.evolve/inbox/consumed/2026-08-04T07-11-00Z-crossartifact-invariant-stack.json` — the queue record for the item this lane is bound to, moved into its consumed position.
  The removal half of that move is already on main in this cycle's base (`ca0a9926`,
  "claimed inbox items leave the root", which deletes the root-queue copy at
  `.evolve/inbox/2026-08-04T07-11-00Z-crossartifact-invariant-stack.json`),
  so what this diff carries is the add half: the record stamped with `consumed.at` /
  `via: ship` and the released cycle-1676 continuation. It is bookkeeping, not behaviour —
  it is what keeps the item from being re-dispatched to a second lane while this one still
  holds it, and keeps the 1676 continuation auditable now that the root copy is gone.
- `docs/private/research/archived-2026-09-14/unshipped-build-explanations/cycle-1676-01m2ffk2tswkga3btqsgps7cxv.md` — the parent attempt's never-published explanation document, moved here out of the published `docs/explain/builds/` tree so this cycle publishes exactly one cycle-owned document while the superseded narrative stays auditable rather than deleted.

## Design Decisions
The aggregate stays advisory. A finding changes no verdict and raises no system
failure, and the record is written on every cycle including all-ok ones: a
false-positive rate that is never recorded can never be evidenced, and that
evidence is the only door to graduating any of these four verifiers to blocking.
Absence is a third status rather than a violation — a missing or malformed artifact
reports `indeterminate`, so the check cannot manufacture its own flake rate.

Containment is checked before the stat, not after, and by `filepath.Rel` against
the root rather than by string inspection of the citation, so the rule holds for
any spelling that cleans to an escape. Absolute citations are left to
`filepath.Join`'s existing behaviour (they are re-rooted, not escapes) rather than
adding a second rejection branch with no failing case behind it.

## Verification
`go/internal/coherence/crossartifact_test.go` is 14/14 PASS, including the new
escape case proven RED against the pre-fix source. The seam's four wiring tests
through the real `finalizeCycle` are 4/4 PASS. This cycle's eight acceptance
predicates are 8/8 PASS, the documentation half and the two audit-repair rounds'
additions included. The `internal/core` package — the one this cycle's added wiring
test enrols into the ship-time repo-contract added-test backstop — is green in the
lane worktree, as is the native cycle ACS suite, and `internal/coherence` covers all
of its exported symbols under the apicover gate. The gate's own `-timeout` fix is
pinned by two tests in `internal/phases/ship`, each demonstrated RED against the
pre-fix argv and against a no-headroom deadline before being taken green.

## Compatibility
No exported signature, artifact schema, gate, flag, or policy default changes. The
new advisory artifact is additive beside a cycle's existing workspace files, and no
consumer of `crossartifact-invariants.json` is required. The behavioural change is
confined to one direction of one verifier: an evidence citation that escapes both
roots now reports `violated` instead of `ok` — advisory in both cases, so no cycle
outcome moves.

## Limitations
The live false-positive rate of the four verifiers is still uncounted, so none of
them can graduate to blocking yet; accumulating that evidence needs a run of cycles,
not a code change. The documented prescription to make the `flagreaders` and
`noorphan` regression walks tolerate an EPERM directory is not applied here: both
files are pipeline control plane, which a cycle may not edit, and they need a
human-gated manual ship.
