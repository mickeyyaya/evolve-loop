# internal/gateregistry

## Purpose

`internal/gateregistry` lists each gate that has a typed entry, and each pack pattern that has none. The design is in [gate-contract.md](../gate-contract.md) section 6.

## Design

- `Entries` returns a fresh table of three entries: `testdup-ratchet`, `size-ratchet` and `raw-git-ratchet`. Each is at `enforce`.
- `Unregistered` returns the 15 pack patterns that no entry covers. The set may only shrink.
- Each run function joins `go` to `Input.Root`. It has a read step and a judge step.
- An error of the read step returns an error and no verdict. A red of the judge step returns a `Verdict` with reds and no error.
- The note of `rawgitratchet.BoundTestFiles` goes to `Notes`.

## Invariants

- No pipeline package imports this package in G1. Its tests and the later parity harness read it.
- A gate package never imports this package. This package never imports `repocontract` outside a test file.
- The real-tree test of each seed gate calls its run function from an external test package.
- The tests of this package that read the tree are in `repocontract.TreeReadingTests()`.
