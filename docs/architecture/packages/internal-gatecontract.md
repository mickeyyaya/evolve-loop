# internal/gatecontract

## Purpose

`internal/gatecontract` holds the types of a gate entry and the words of the gate contract. The design is in [gate-contract.md](../gate-contract.md) section 6.

## Design

- `Entry` names a gate, its kind, its files, its owners, its inputs, its stage and its run function.
- `Input` holds the repository root and the base. `Verdict` holds the reds and the notes.
- `Run` returns an error when the gate did not judge. An error is not a red, and it is not a pass.
- `TrustClasses`, `Kinds` and `Stages` each return a copy of one list. A caller cannot change the list.
- The package imports the standard library only. It holds no gate.

## Invariants

- `TrustClasses` is the one home of the class words. The inventory test of `internal/profiles` reads it.
- The package is on the protected surface as a gate guarantee.
