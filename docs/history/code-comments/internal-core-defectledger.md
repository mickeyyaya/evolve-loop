# Comment history: `internal/core/defectledger`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## ADR-0124 Q2: the ledger leaf declares reportdoc

### `go/internal/core/defectledger/importgraph_test.go:3` — above `import (`

```text
// importgraph_test.go — the package is a leaf under core (ADR-0103 unit 09
// §2): stdlib plus the five named internal packages, never internal/core,
// carryover or phasecontract (the compiler is the cycle guard; this is the
// leaf-ness declaration — signalcenter/importgraph_test.go idiom). The leaf
// also never writes stderr: its failure modes are codes, not prose lines.
```
