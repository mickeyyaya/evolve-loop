# internal/cycleidentity

> Plan: [oss-harness-architectures-2026-11](../../plans/oss-harness-architectures-2026-11.md) item W2.2. Spec: [harness-edge-contracts](../harness-edge-contracts.md) section 3. Callers: `internal/core` (the cycle record and the resume check) and `cmd/evolve` (the wiring).

## Purpose

`internal/cycleidentity` gives the identity of a cycle. A resume compares the identity that the cycle recorded at its start with the identity of the pipeline now. When the two differ, the pipeline changed during the pause, and the resume must not continue unchecked.

## Design

- **`Identity`.** It holds three digests: `graph_hash`, `contract_hash` and `policy_hash`.
- **`Sources`, `Live` and `Compute`.** `Sources` names the project root and the phase contracts. `Live` fills the contracts from `phasecontract.Contracts`. `Compute` reads the files and gives the `Identity`.
- **Graph.** The graph digest covers `docs/architecture/phase-registry.json`. The file holds the phase order and the phase registry.
- **Contract.** The contract digest covers the phase contracts, sorted by phase name. A change to a section, a verdict or an artifact name changes it.
- **Policy.** The policy digest covers `.evolve/policy.json`, except its top-level `resume` block. That block holds the stage of this check and the checkpoint age. They control the resume, not the pipeline. If the digest includes them, the operator flip of `resume.identity.stage` to `enforce` itself refuses each paused cycle. Pinned by `TestCompute_TheResumeBlockOfThePolicyIsOutsideThePolicyPart`.
- **Normal form.** A JSON file is parsed and written again with sorted keys before the digest. White space and key order do not change a digest. A file that is not JSON is hashed as it is. A missing file digests as empty bytes. A file that cannot be read returns an error.
- **`Diff` and `Check`.** `Diff` lists the parts that differ. `Check` returns a `*MismatchError` that names each part. The error wraps `ErrMismatch`.
- **`Encode` and `Decode`.** The ledger record carries the identity as a JSON string. `Decode` of an empty string returns `ErrNoIdentity`, which is the case of a cycle that started before W2.2.

## Invariants

- `Compute` reads files only. It never writes.
- A part name is one of `graph`, `contract` or `policy`.
- The package has no dependency on `internal/core`.

## How core uses it

See [internal-core](internal-core.md#resume-identity-w22).
