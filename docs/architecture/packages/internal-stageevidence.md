# Package `internal/stageevidence`

`internal/stageevidence` checks shadow-stage evidence chains, computes decision version digests, and scores stage readiness.

## Exports

- `Kind` (`KindDecision`, `KindLate`) classifies evidence records.
- `Class` (`ClassNone`, `ClassFalseStop`, `ClassEscape`, `ClassConflict`) classifies scored faults.
- `ErrBrokenChain`, `ErrEmptyChain`, `BrokenChainError`, and `ErrorCode` represent chain failures as typed values.
- `Record`, `Outcome`, `Chain`, `Case`, and `Result` define the JSON-serializable chain and verdict data.
- `Scorer`, `DefaultScorer`, and `ScorerFor` map each stage key to a named decision classifier.
- `Link`, `VerifyChain`, `Score`, and `Verify` seal, validate, and score evidence chains.
- `ChainReader` and `FileChain` abstract chain storage for fixture files and future ledger adapters.
- `DecisionVersion` hashes the `go.mod` module and toolchain lines, the module-local decider closure, and the policy subtree.

## Rules

- `internal/stageevidence` imports `gatecontract`, `regressiontia`, and `sysexec`, and never imports `core`, `ship`, or `gateregistry`.
- `FileChain` calls `VerifyChain` on the whole file before it filters records for one stage key.
- `Score` ignores `KindLate` records, records of other versions, and cycles with `Incomplete` outcomes.
- `Score` sets `Ready` to true only when decisions meet `max(MinDecisions, 1)`, faults stay within limits, and `Conflicts` equals zero.
- `DecisionVersion` reuses `regressiontia.ModuleLocalReach` to find module-local imports of the decider closure.
