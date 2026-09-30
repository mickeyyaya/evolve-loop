# Comment history: `acs/cycle1303`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1303/predicates_test.go:3` — above `package cycle1303`

```text
// Package cycle1303 materialises the cycle-1303 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	T1 centralize-releasepreflight-verdict-parsing
//	(todo-id sentinel-parse-tail-anchor)
//
// What the cycle must deliver. `go/internal/releasepreflight/releasepreflight.go`
// hand-rolls a SECOND verdict-sentinel scanner (`machineVerdictRE` +
// `markerVerdict`, lines 582-608) instead of calling the project's single source
// of truth, `phasecontract.ParseVerdictSentinelFull` (`sentinel.go:90-98`). The
// duplicate happens to be last-match-wins, so it does not reproduce the
// cycle-1298 first-match bug — but it is missing the SSOT's placeholder-echo
// guard (`isPlaceholderEcho`, `sentinel.go:62-74`, cycle-603 class). A
// Deliverable-Contract example sentinel captured from scrollback into an
// audit artifact is therefore still authoritative to release-preflight, in
// exactly the one call site that never adopted the centralised fix. This cycle
// deletes the duplicate and delegates to the SSOT.
//
// Predicate strategy — every predicate exercises real behaviour through the
// REAL production caller, never a source grep (the cycle-85 degenerate-predicate
// ban). `markerVerdict`/`extractVerdict` are unexported, so this external
// package can only reach them the way the release pipeline does: through
// `releasepreflight.Run`, whose step 4 reads the ledger's newest auditor
// artifact and gates the release on the verdict it parses out of it. Every
// predicate below drives `Run` over a temp repo whose audit-report body is the
// variable under test, and asserts on the release decision that comes back.
// A predicate that called a parser directly would pass on dead code; these stay
// RED until releasepreflight.go's own step-4 path actually reads through
// phasecontract.
//
//   - 001 is the crux and the NEGATIVE case: a real FAIL sentinel followed by a
//     contract-example placeholder echo declaring PASS must NOT release. Today
//     the duplicate scanner takes the placeholder (last valid marker wins, no
//     placeholder guard) and the release is allowed — RED.
//   - 002 is the opposite polarity, and is what makes 001 impossible to satisfy
//     by bolting a reject onto the duplicate: a SOLE placeholder-echo marker is
//     not a marker at all, so the prose verdict below it must govern and the
//     release must proceed. Only true delegation to ParseVerdictSentinelFull
//     produces both 001 and 002.
//   - 003 is the differential ORACLE for the delegation itself: over a corpus of
//     artifact bodies, the verdict release-preflight acts on must equal what
//     phasecontract.ParseVerdictSentinelFull returns for the same bytes. This is
//     AC1 stated behaviourally — two parsers that agree on every input the
//     oracle can distinguish are one parser.
//   - 004 is the AC2 regression guard: the PASS/WARN/FAIL mapping, strict-mode
//     WARN rejection, marker-is-authoritative precedence and prose fallback all
//     keep their current outcomes. Expected pre-existing GREEN — it must stay
//     green through the swap.
//   - 005 is the AC4 suite gate: the two owning packages, each shelled as ONE
//     named package (never a /... sweep), per the flaky-predicate-shape rules.
//
// Fixture note: the repo fixture is hand-rolled rather than borrowed from
// go/test/fixtures so that the ledger line, the artifact path and the artifact
// body are all visible at the point of use — the artifact body IS the variable
// under test in every predicate here.
```

### `go/acs/cycle1303/predicates_test.go:79` — above `func placeholderSentinel(verdict string) string {`

```text
// placeholderSentinel renders the shape that only ever comes from a Deliverable
// Contract's own printed example echoed into captured scrollback: a v2 sentinel
// whose failure block still holds literal angle-bracket placeholder tokens
// (cycle-603). phasecontract rejects it; the duplicate scanner does not.
```

### `go/acs/cycle1303/predicates_test.go:142` — above `const realFailThenPlaceholderEcho = "# Audit — cycle 99\n\n" +`

```text
// realFailThenPlaceholderEcho is the cycle-603 shape landing in the one call
// site that never adopted the guard: the auditor's genuine FAIL, then the
// Deliverable Contract's own printed PASS example captured from scrollback.
```

### `go/acs/cycle1303/predicates_test.go:163` — above `func TestC1303_001_placeholder_echo_cannot_override_real_fail(t *testing.T) {`

```text
// TestC1303_001_placeholder_echo_cannot_override_real_fail is the NEGATIVE case
// and the wiring proof: a genuine FAIL sentinel trailed by a contract-example
// placeholder echo must still block the release. The duplicate scanner in
// releasepreflight takes the last JSON-parsable marker with no placeholder
// guard, so today the echoed PASS wins and the release is allowed — the exact
// re-opening of the closed cycle-603 class that centralising on
// phasecontract.ParseVerdictSentinelFull prevents.
```
