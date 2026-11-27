# Package `internal/stageevidence`

`internal/stageevidence` checks shadow-stage evidence chains, computes decision version digests, and scores stage readiness.

## Exports

- `Kind` (`KindDecision`, `KindLate`, `KindOpen`, `KindClose`) classifies evidence records. `KindOpen` and `KindClose` are the markers of a session.
- `ScopeKind` (`ScopeCycle`, `ScopePool`, `ScopeFleet`, `ScopeCurator`), `ScopeKey`, `CycleScope` and `ParseScope` name the unit that one seal covers.
- `Recorder`, `RecorderConfig`, `NewRecorder` and `WorkingPath` write the evidence of one session into the working file of a scope.
- `Seal`, `SealInput`, `BuildSeal`, `VerifyStored`, `ParseSeal` and the cause codes build and check the commitment that a seal makes.
- `PreverifiedChain` builds a chain that a ledger reader has verified already.
- `Class` (`ClassNone`, `ClassFalseStop`, `ClassEscape`, `ClassConflict`) classifies scored faults.
- `ErrBrokenChain`, `ErrEmptyChain`, `ErrLedgerBroken`, `ErrCorruptStore`, `BrokenChainError`, and `ErrorCode` represent chain failures as typed values. `ErrorCodeUntyped` is the code of an error that is none of the four typed ones.
- `Record`, `Outcome`, `Chain`, `Case`, and `Result` define the JSON-serializable chain and verdict data.
- `Scorer`, `DefaultScorer`, and `ScorerFor` map each stage key to a named decision classifier.
- `Link`, `VerifyChain`, `Score`, and `Verify` seal, validate, and score evidence chains.
- `ChainReader` and `FileChain` abstract chain storage for fixture files and future ledger adapters.
- `DecisionVersion` hashes the `go.mod` module and toolchain lines, the module-local decider closure, and the policy subtree.

## Rules

- `internal/stageevidence` imports `gatecontract`, `regressiontia`, and `sysexec`, and never imports `core`, `ship`, or `gateregistry`.
- `VerifyChain` checks the chain of each scope key on its own. The first record of a scope has an empty `prev`.
- `FileChain` calls `VerifyChain` on the whole file before it filters records for one stage key.
- `Case`, `Result.FirstID`, `Result.LastID` and `Result.IncompleteScopes` use scope-neutral names. `IncompleteScopes` holds the scope labels, such as `cycle-7` and `pool-7`, so equal numbers of two scope kinds stay apart.
- The recorder never fails a cycle. A write error calls `OnFault`, and the close marker then carries the fault, so the seal marks the scope `incomplete`. The file lock comes from the caller.
- The recorder raises the counter of a key before it writes. A counter above the count of records is a gap that the seal reports.
- The seal drops a torn line and counts it. It never repairs a line.
- `BuildSeal` checks that a prior seal is a prefix of the working file. A history that was rewritten fails with `BrokenChainError`.
- The reasons for the scope key, the markers and the seal are in the design `docs/plans/shadow-contract-k2-k4-2026-11.md` (PR #1674) and in `docs/architecture/shadow-stage-contract.md` section 10.2.
- `Score` ignores `KindLate` records, records of other versions, and cycles with `Incomplete` outcomes.
- `Score` sets `Ready` to true only when decisions meet `max(MinDecisions, 1)`, faults stay within limits, `Conflicts` equals zero, and `NotReady` is empty. A decision that carries `would_hold` in an outcome whose `RoutesMeasured` is false sets `NotReady` to `conflicts_unmeasured`.
- `DecisionVersion` reuses `regressiontia.ModuleLocalReach` to find module-local imports of the decider closure.

## Flip records and the check (K4c)

- `flip.go` defines two ledger record kinds. `stage-flip` holds `{v, key, value, origin, policy_digest, reason}`. `stage-genesis` holds `{v, keys, policy_digest, count}` and an optional `reason`. `Encode` and `ParseFlip` or `ParseGenesis` convert them. A parse error wraps `ErrMalformedFlipRecord`.
- `FlipRecords` is the read port. It returns a `FlipSet`: each flip record and the first genesis record. The ledger adapter implements it.
- `Check` is a pure function of one `CheckInput`: the effective stages, a `FlipSet` and `GenesisRequired`. It gives a `Mode` and the `Findings`. It never reads a file.
  - No genesis and `GenesisRequired` false: `inactive`. Each key at `enforce` with no record is a finding.
  - Genesis present and `GenesisRequired` false: `report-only`.
  - Genesis present and `GenesisRequired` true: `active`.
  - No genesis and `GenesisRequired` true: `refuse`. The first finding has the key `genesis`.
- A `grandfather` record counts only when its key is in the genesis `keys`. A `verified` record counts for its own key. A record whose value is not `enforce` counts for nothing.
- `ErrGenesisRequired` names decision D18. `ErrUnresolvedStageKey` names a key that has no effective stage.
