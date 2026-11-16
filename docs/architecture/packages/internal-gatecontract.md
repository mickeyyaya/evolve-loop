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

## Shadow descriptor (K1a)

- `Entry.Shadow` is a pointer to `Shadow`. It holds the policy key, the signal code and the `Decide`, `Enforce` and `Score` function values. It also holds the decision fields, emit sites, `Promotion` numbers, closure and policy subtree. An entry without a shadow contract has `Shadow == nil`.
- `KindStageFeature` is the fourth kind.
- `Entry.EffectiveStage` takes a policy lookup. It returns the policy value for an entry that has a policy key, and the seed `Stage` otherwise.
