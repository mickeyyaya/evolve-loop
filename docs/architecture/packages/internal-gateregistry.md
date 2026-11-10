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
