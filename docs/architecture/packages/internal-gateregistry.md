# internal/gateregistry

## Purpose

`internal/gateregistry` lists each gate that has a typed entry, and each pack pattern that has none. The design is in [gate-contract.md](../gate-contract.md) section 6.

## Design

- `Entries` returns a fresh table of six entries: `testdup-ratchet`, `size-ratchet`, `raw-git-ratchet`, `repo-contract-floor`, `unit-test-floor` and `cover-changed-floor`. Each is at `enforce`.
- The three floor entries have the kind `phase-floor`. Each runs a `core.FloorJudge` through `core.FloorEntryRun`. The judge and the build floor call the same check function.
- One `floorSpec` value gives the name, the files, the judge and the verified lists of a floor entry. The subject of each is `go/`.
- `Input.Base` is the base commit of a floor entry. A failed read of the tree or of the base is an error. A lane-only red is a red.
- `Unregistered` returns the 15 pack patterns that no entry covers. The set may only shrink.
- The run function of each ratchet entry joins `go` to `Input.Root`. A floor entry gives `Input.Root` to `core` as the worktree, and `core` finds the module.
- Each run function has a read step and a judge step.
- An error of the read step returns an error and no verdict. A red of the judge step returns a `Verdict` with reds and no error.
- The note of `rawgitratchet.BoundTestFiles` goes to `Notes`.

## Invariants

- No pipeline package imports this package in G1. It imports `core` and `phases/ship` for the floor entries. Its tests and the later parity harness read it.
- A gate package never imports this package. This package never imports `repocontract` outside a test file.
- The real-tree test of each seed gate calls its run function from an external test package.
- The tests of this package that read the tree are in `repocontract.TreeReadingTests()`.

## Stage entries (K1a)

- `StageEntries` returns one `stage-feature` entry for each stage key of the typed policy schema (41 keys). Each entry has a `Shadow` descriptor and no seed `Stage`. `Entries` stays unchanged, so the real-tree gate tests never run a stage entry.
- A seed table `stageSeeds` holds the data: the key, the signal code, the emit site, the decision fields and the closure. The policy subtree is the key without its last segment, and a test pins it. A test pins the populated seed rows and the copy of their slices. The signal codes that exist as constants (`core.CodeLadderDecision`, `ship.CodeHoldShadow`) are referenced, not repeated. `Decide`, `Enforce` and `Score` are nil; K1b wires them.
- An emit site is `gatecontract.EmitSite{Path, Func, Ref}`. A function value cannot name an unexported function of another package. So the test parses the file and checks four facts:
  - The function exists in a non-test file.
  - A non-test file of the same directory calls it.
  - It uses `Ref`.
  - It holds each decision field as a string literal.

  This differs from the design text, which names function values: a string can break on a rename, and the call check closes the dead-code gap. K1b adds a runtime spy through the port.
- The conformance tests walk the policy struct by reflection, never the live `policy.json`. They fail for a `(key, rule)` pair that is not in `testdata/stage_violations.json`.
- The baseline lists 88 pairs: 41 `string-typed-stage` (the stage constants are not unified in this change), and the missing signals, unregistered codes and emit sites. `TestViolationBaselineOnlyShrinks` requires the count to equal `baselineCeiling`, rejects a duplicate and rejects a pair that is no longer a violation. A fix removes its pairs and lowers the ceiling.
- Two spec keys have no stage field in the schema and no entry: the ship scan skip and `gates.main_red_stop`.
