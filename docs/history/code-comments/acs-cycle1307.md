# Comment history: `acs/cycle1307`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1307/predicates_test.go:3` — above `package cycle1307`

```text
// Package cycle1307 materialises the cycle-1307 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	sentinel-parse-tail-anchor → anchor evolve-verdict sentinel extraction at the
//	TAIL (last parseable candidate) in ONE shared implementation used by both
//	phasecontract sentinel parsing and deliverable verification, carrying the
//	cycle-1298 regression fixture (5 quoted decoys + 1 real tail sentinel), AND
//	documenting the rule in the operator-facing contract doc.
//
// State this cycle inherited (scout-report): the CODE half is already shipped on
// this branch — ParseVerdictSentinelFull walks candidates from the end
// (sentinel.go:90-98) and the fixture + unit tests are green. The predicates
// below therefore split into two groups on purpose:
//
//   - 001-004 are BEHAVIOURAL guards on the shipped semantics. They are expected
//     PRE-EXISTING GREEN and exist so a Builder that touches sentinel.go this
//     cycle (five sibling lanes received the identical inbox item — collision is
//     live) cannot silently regress first-match selection.
//   - 005 is the RED one: docs/architecture/deliverable-contract.md, the
//     connects_to target the inbox item named, has ZERO mention of tail-anchored
//     selection. That is this cycle's outstanding work.
//
// Predicate strategy — every behavioural predicate drives the real function or
// the real production entry point (deliverable.Verify) and asserts on its return
// value, never a source-grep of production code (the cycle-85 degenerate-
// predicate ban). 001 additionally carries an ANTI-VACUITY arm: it recomputes
// the OLD first-match selection over the same live fixture and asserts it gives
// the WRONG answer, so the predicate cannot pass on a repo where the fix was
// reverted. 004 is the wiring proof: it reaches the parser through
// deliverable.Verify — the entry the contract gate actually calls — not through
// phasecontract directly, so a tail-anchored parser with no production reader
// still fails it.
```

### `go/acs/cycle1307/predicates_test.go:50` — above `const fixtureRelPath = "go/internal/phasecontract/testdata/cycle1298-quoted-decoys.md"`

```text
// fixtureRelPath is the cycle-1298 regression artifact the inbox item demanded
// be captured: the real adversarial-review report whose prose quotes the
// sentinel shape five times before emitting the genuine FAIL at the tail.
```

### `go/acs/cycle1307/predicates_test.go:79` — above `func TestC1307_001_TailAnchoredSelectionOnLiveCycle1298Fixture(t *testing.T) {`

```text
// TestC1307_001_TailAnchoredSelectionOnLiveCycle1298Fixture is the crux
// behavioural predicate: the shared parser, run over the LIVE cycle-1298 report,
// must return the genuine tail verdict (FAIL, class gate_bypass) rather than any
// of the five decoys its prose quotes. The second arm proves the predicate is
// not vacuous — the retired first-match selection returns a DIFFERENT verdict on
// these exact bytes, so this test can only pass on a tail-anchored parser.
```

### `go/acs/cycle1307/predicates_test.go:110` — above `func TestC1307_002_MalformedEarlierDecoyDoesNotBlankTail(t *testing.T) {`

```text
// TestC1307_002_MalformedEarlierDecoyDoesNotBlankTail is the elided-JSON shape
// that circuit-opened the gate in cycle-1298: an earlier candidate that does not
// unmarshal must be SKIPPED, not treated as fatal for the whole read.
```

### `go/acs/cycle1307/predicates_test.go:209` — above `func TestC1307_005_ContractDocDocumentsTailAnchoring(t *testing.T) {`

```text
// TestC1307_005_ContractDocDocumentsTailAnchoring is this cycle's RED predicate.
// The operator SSOT must explain the tail-anchoring rule, name the incident that
// motivated it, and point at the regression fixture — otherwise the next author
// of a sentinel reader re-derives first-match selection and re-breaks the gate.
//
// acs-predicate: config-check — the deliverable of this criterion IS operator
// prose, so document content is the system under test, not a proxy for it.
```
