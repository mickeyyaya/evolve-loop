# Package `internal/stageevidence`

`internal/stageevidence` checks shadow-stage evidence chains, computes decision version digests, and scores stage readiness.

## Exports

- `Kind` (`KindDecision`, `KindLate`, `KindOpen`, `KindClose`) classifies evidence records. `KindOpen` and `KindClose` are the markers of a session.
- `ScopeKind` (`ScopeCycle`, `ScopePool`, `ScopeFleet`, `ScopeCurator`), `ScopeKey`, `CycleScope` and `ParseScope` name the unit that one seal covers.
- `Recorder`, `RecorderConfig`, `NewRecorder` and `WorkingPath` write the evidence of one session into the working file of a scope.
- `Seal`, `SealInput`, `BuildSeal`, `VerifyStored`, `ParseSeal` and the cause codes build and check the commitment that a seal makes.
- `PreverifiedChain` builds a chain that a ledger reader has verified already.
- `Class` (`ClassNone`, `ClassFalseStop`, `ClassEscape`, `ClassConflict`) classifies scored faults.
- `ErrBrokenChain`, `ErrEmptyChain`, `ErrLedgerBroken`, `ErrCorruptStore`, `BrokenChainError`, and `ErrorCode` represent chain failures as typed values.
- `Record`, `Outcome`, `Chain`, `Case`, and `Result` define the JSON-serializable chain and verdict data.
- `Scorer`, `DefaultScorer`, and `ScorerFor` map each stage key to a named decision classifier.
- `Link`, `VerifyChain`, `Score`, and `Verify` seal, validate, and score evidence chains.
- `ChainReader` and `FileChain` abstract chain storage for fixture files and future ledger adapters.
- `DecisionVersion` hashes the `go.mod` module and toolchain lines, the module-local decider closure, and the policy subtree.

## Rules

- `internal/stageevidence` imports `gatecontract`, `regressiontia`, and `sysexec`, and never imports `core`, `ship`, or `gateregistry`.
- `VerifyChain` checks the chain of each scope key on its own. The first record of a scope has an empty `prev`.
- `FileChain` calls `VerifyChain` on the whole file before it filters records for one stage key.
- The recorder never fails a cycle. A write error calls `OnFault`, and the close marker then carries the fault, so the seal marks the scope `incomplete`. The file lock comes from the caller.
- The recorder raises the counter of a key before it writes. A counter above the count of records is a gap that the seal reports.
- The seal drops a torn line and counts it. It never repairs a line.
- `BuildSeal` checks that a prior seal is a prefix of the working file. A history that was rewritten fails with `BrokenChainError`.
- The reasons for the scope key, the markers and the seal are in the design `docs/plans/shadow-contract-k2-k4-2026-11.md` (PR #1674) and in `docs/architecture/shadow-stage-contract.md` section 10.2.
- `Score` ignores `KindLate` records, records of other versions, and cycles with `Incomplete` outcomes.
- `Score` sets `Ready` to true only when decisions meet `max(MinDecisions, 1)`, faults stay within limits, and `Conflicts` equals zero.
- `DecisionVersion` reuses `regressiontia.ModuleLocalReach` to find module-local imports of the decider closure.
