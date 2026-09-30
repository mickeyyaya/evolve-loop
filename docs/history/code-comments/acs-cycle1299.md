# Comment history: `acs/cycle1299`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1299/predicates_test.go:3` — above `package cycle1299`

```text
// Package cycle1299 materialises the cycle-1299 acceptance criteria for the two
// fleet-scoped tasks pinned to this lane (inbox item `sentinel-parse-tail-anchor`):
//
//   - sentinel-tail-anchor-parse        → tail-anchored verdict-sentinel selection
//   - sentinel-tail-anchor-live-fixture → the cycle-1298 report as regression proof
//
// The defect. `ParseVerdictSentinelFull` selected with `FindStringSubmatch`, which
// returns the FIRST structural match in the document. A report that QUOTES the
// sentinel shape in prose therefore beat the real sentinel at the tail: a quoted
// decoy that unmarshals silently won, and a quoted decoy with elided JSON blanked
// the whole read (unmarshal failure returns ok=false instead of trying the next
// candidate). Both shapes are live in `.evolve/runs/cycle-1298/adversarial-review-report.md`
// — 5 quoted decoys + 1 real tail sentinel — which fired [bad_verdict] x3 and
// circuit-opened the contract gate enforce→advisory.
//
// Predicate strategy — every predicate below CALLS the production parser
// (`phasecontract.ParseVerdictSentinelFull`) or the production reader
// (`phasecontract.ReadFailureBlock`) and asserts on its return value. None of
// them greps sentinel.go for a magic string, so adding `FindAllStringSubmatch`
// to the source without correct tail-walk semantics does not green them (the
// cycle-85 degenerate-predicate ban).
//
//   - 001 AC1: an earlier UNPARSEABLE decoy must not blank the real tail sentinel.
//   - 002 AC1': an earlier WELL-FORMED decoy must not win either — this is what
//     separates a real tail anchor from the partial fix "scan until something
//     unmarshals".
//   - 003 AC2 (negative): all-malformed candidates still yield ok=false, so the
//     caller's legacy parser runs. Anti-gaming: a parser that returns the last
//     RAW match regardless of validity fails here.
//   - 004 AC3 (edge): zero sentinels, empty input, and an empty payload are
//     unchanged (ok=false).
//   - 005 AC4: the LIVE cycle-1298 fixture, read from testdata, parses to
//     verdict=FAIL / class=gate_bypass through the real gate-caller function.
//   - 006 AC5 (anti-vacuity): the legacy first-match selection, reconstructed in
//     the predicate over the same fixture, produces a DIFFERENT (wrong) answer —
//     proof the fixture discriminates fixed from broken.
//   - 007 wiring proof: `ReadFailureBlock`, the single production reader the
//     router / faillearn / classifier go through, surfaces the tail sentinel's
//     failure block off the fixture laid down as a phase report.
//
// Roots: the fixture is a Builder/TDD deliverable landing in the worktree, so it
// is resolved under acsassert.RepoRoot (the SOURCE root). Its absence is a
// FAILURE, not a skip.
```

### `go/acs/cycle1299/predicates_test.go:60` — above `func fixturePath(t *testing.T) string {`

```text
// fixturePath resolves the committed cycle-1298 regression fixture.
```
