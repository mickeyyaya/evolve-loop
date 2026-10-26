# Contract single-sourcing — the recipe for machine-graded agent artifacts

> ADR: [0084 invariant I2](adr/0084-gate-integrity-invariants.md). Reference
> implementation: `go/internal/phases/audit/defect_ledger_schema_singlesource_test.go`
> (#422). Further implementations: `go/internal/core/disposition_gate_singlesource_test.go`,
> `go/internal/evalqualitycheck/vacuity_test.go` (#426).

## Problem

An LLM agent must AUTHOR an artifact, and Go code then PARSES it. This is a producer/consumer contract. The two halves of the contract live in different media: prose instructions and a Go struct. Prose drifts silently.

The 2026-08-09 incident counted 14+ such contracts with no binding test. Two of them were live bugs:

- a placeholder pseudo-JSON "example" that failed its own fail-hard gate;
- a quality gate that was vacuous for 281 evals, because the documented format and the scanner disagreed.

## Approach — the three-legged test

When you add or change a machine-graded artifact:

1. **Literal example in the authoring instructions.** The agent's prompt
   (`agents/<name>.md` or the skill section) shows a COMPLETE, LEGAL document.
   Use real values, never `<int>`/`"A | B | C"` placeholders. Vocabulary/field
   rules go in prose AROUND the example, not inside it.
2. **Go-side mirror.** The parser's package carries the same document as a
   const (for example, `dispositionSchemaExample`). Place it next to the reader
   that it must satisfy.
3. **Three-legged test** in the parser's package:
   - Leg A: extract the example block from the instruction file (anchor or
     heading + fenced-block regex). Assert that it is JSON/YAML-equal to the Go
     const. Prose drift fails CI.
   - Leg B: feed the const through the PRODUCTION reader/gate. Use the fixture
     context that the gate cross-checks (for example, a matching failure-digest).
     Assert that the const passes. An illegal example fails CI.
   - Leg C (when the gate has anti-gaming semantics): assert that nobody can
     trivially replay the documented example to defeat the gate. For example,
     the disposition digest cross-check rejects a copy-pasted fingerprint. A
     grader bullet inside an illustration fence does not count as a command.

## Rules of thumb

- The test lives with the CONSUMER (parser/gate package). The consumer owns
  the contract.
- If the instructions render the example inside an illustration fence
  (```` ```markdown ````), the test unwraps the fence. The test then scans what
  an authored file will actually contain.
- Tolerant parsing is allowed only where meaning is unambiguous (string OR
  array-of-strings). It must stay fail-closed for every other shape. Each
  accepted shape gets its own negative-boundary test.
- Cost note: a literal example adds ~200–400 prompt tokens per dispatch of the
  authoring phase. One cycle that fails on a guessed schema costs ~2M tokens.
- It is recommended to flag a new contract without a three-legged test at review. The
  diff-review skill and the auditor persona both carry this lens.

## Known untied contracts (queued)

`routing-plan.json` mint block (two divergent examples), `triage-decision.json`
(7 readers, drifted both directions), `reflection.yaml` /
`handoff-retrospective.json` / `predicate_quality` (instructed but reader dead
or absent — delete-or-revive sweep). See the inbox items filed from the
2026-08-09 postmortem.
