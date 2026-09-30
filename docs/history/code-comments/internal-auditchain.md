# Comment history: `internal/auditchain`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/auditchain/chainblock.go:3` — above `import (`

```text
// chainblock.go — the wire format between the auditor and the pipeline.
//
// The chain has to survive the trip from an LLM's markdown into Go, and this
// repo's history with LLM-authored machine-graded artifacts is unkind: every
// generation that kept the literal example in the prompt and the parser in Go
// drifted apart, and the drift always surfaced as a gate blaming the agent for
// producing what it was told to produce. So the example the auditor is SHOWN
// (ChainBlockExample) is the same constant the tests round-trip through the
// parser — three legs, one shape (ADR-0084 I2).
//
// Format choice: an HTML comment, one row per link, pipe-separated, with the
// finding LAST so a separator inside prose cannot shift the citation out of its
// column. A comment because the block is machine state that must not clutter
// the human report; row-per-link because a missing link has to be visible as a
// missing ROW rather than as a subtly different JSON shape.
```

### `go/internal/auditchain/chainblock_test.go:3` — above `import (`

```text
// chainblock_test.go — the wire format, and the three-legged single-sourcing
// that keeps it honest (ADR-0084 I2): the literal example the PERSONA shows the
// auditor, the Go PARSER that reads what comes back, and a REAL round trip must
// all be the same shape. Every prior generation of this repo's LLM-authored
// artifacts drifted the moment those three lived in different files.
```

### `go/internal/auditchain/chainblock_test.go:117` — above `func TestPersona_InstructsExactlyTheShapeTheParserReads(t *testing.T) {`

```text
// TestPersona_InstructsExactlyTheShapeTheParserReads is the third leg of the
// single-sourcing (ADR-0084 I2): the persona the auditor is DISPATCHED with,
// the parser, and the example must agree. Every prior generation of an
// LLM-authored graded artifact in this repo drifted the moment these lived in
// separate files, and the drift always surfaced as a gate blaming the agent for
// producing what it had been told to produce.
```

### `go/internal/auditchain/chainblock_test.go:145` — above `chainAt := strings.Index(persona, chainBlockOpen)`

```text
// And the instruction must survive compaction: operational directives live
// ABOVE the strip marker (cycle-1390–1429 lesson — a mid-file marker
// deleted the verdict rules from every dispatched audit).
```

### `go/internal/auditchain/evidence_access.go:32` — above `var judgingPhases = map[string]bool{`

```text
// judgingPhases are the phases whose OUTPUT is a judgement about work someone
// else did. Entitlement follows the act of deciding, not seniority: a producing
// phase handed the whole prior corpus is cost with no decision attached, and
// prompts that grow without bound are how the truncation incident happened.
```

### `go/internal/auditchain/shadow.go:3` — above `import "strings"`

```text
// shadow.go — the rollout stage, and the only thing a wave can actually learn
// from.
//
// ADR-0088 says shadow first, and this is what that means concretely: the chain
// is parsed, concluded against the evidence the judging phase was actually
// given, and written beside the cycle — while the phase's own verdict is
// byte-identical to what it would have been with none of this wired.
//
// The datum the soak collects is the DISAGREEMENT: where the chain and the
// narrative verdict differ, and which link the narrative was silent about.
// Enforcing before that record exists would repeat this repo's most expensive
// habit — a gate switched on against an unmeasured population, discovered to
// be wrong only after it had force-FAILed real work.
```
