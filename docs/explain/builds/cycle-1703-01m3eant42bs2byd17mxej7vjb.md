# Build Explanation — Cycle 1703

## Build Binding
- Cycle: 1703
- Base SHA: 73a3bb4f54ee309371c670c700a8bd63fe612b53

## Summary
The TDD scope gate's multi-member branch now runs a file-scope check after the member-set reconciliation passes.
Before this change, a lane with two or more committed members returned as soon as its declared set matched, so the
file-scope advisory never ran for any multi-member lane. The new check unions every committed member's scout
`targetFiles` and advises (never blocks) only when no authored test file shares a directory with any path in that
union. Single-member lanes keep their exact advisory text.

## Rationale
The cycle-1620 reconciliation fixed set equality but its early `return` bypassed the construction-level check from
cycle 1111, which the cycle-1620 audit filed as L3. The bug-reproduction phase confirmed a complete two-member lane
authoring outside both scopes logged nothing through the enforce-stage reviewer. A union with ANY-overlap semantics
is the only reading that fits unordered multi-member work: requiring overlap with every member would flag a TDD
phase that tests one member through a shared file, and checking only the first member would depend on ordering. The
gate's contract is that every ambiguity fails open, so a member with no declared scope, which could own any file,
turns the judgement off rather than judging against a partial union.

## Changed Areas
- `go/internal/topngate/gate.go` — `tddScopeGate.check` now returns the `scope-mismatch` block unchanged and, on a
  complete reconciliation, returns `fileScopeAdvisoryMulti(...)` at `block=false`. The new helper unions the members'
  `targetFiles`, fails open when authored is empty or any member lacks a scope, and names the members, the authored
  files and the union in its reason. The overlap loop is extracted into `anyPathOverlaps` and shared with the
  single-member `fileScopeAdvisory`, whose message is unchanged.
- `go/internal/topngate/scope_reconciliation_test.go` — the TDD phase's in-package regression tests plus one builder
  test, `TestTDDScopeGate_TwoMemberWithOneUndeclaredScopeStaysSilent`, which pins the partial-scope fail-open choice.
- `docs/architecture/packages/internal-topngate.md` — the gate order and invariants now describe the union check, the
  "Known gap" bullet is removed, and a cycle-1703 history entry is added.
- `go/acs/cycle1703/predicates_test.go` — the TDD phase's acceptance predicates, driven through
  `topngate.NewReviewer(config.StageEnforce)`, committed with the build.
- `.evolve/evals/multi-member-file-scope-advisory.md` — the task's eval graders, committed with the build.

## Design Decisions
The advisory stays advisory at every stage, the same as the single-member one: a legitimate deliverable can touch a
shared helper scout never named. The reconciliation block is judged first and returned as-is, so a set mismatch never
carries a file-scope clause. The multi-member reason keeps the `file scope drift (advisory)` prefix so log searches
find both shapes. No exported API changed.

## Verification
`go vet` and `go test -race -count=1 ./internal/topngate/...` pass. The cycle-1703 ACS predicates (001–005) pass, and
the native ACS suite reports red=0. The full `go test -count=1 ./...` run is recorded in the build report.

## Compatibility
Single-member advisory text is byte-identical, pinned by `TestTDDScopeGate_SingleMemberFileScopeAdvisoryTextUnchanged`.
Multi-member lanes that authored inside any member's scope, or that have no scout scope, behave as before. The only new
output is one advisory log line for multi-member lanes that author outside every member's scope.

## Limitations
The check still treats one directory as one scope, so a test in a sibling package of a declared target is advised. A
lane where only some members declare `targetFiles` gets no file-scope judgement.
