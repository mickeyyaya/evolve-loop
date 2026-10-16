# internal/phaseblock

## Purpose

`internal/phaseblock` models each pipeline phase as a content-addressed record, an "agent block" (ADR-0065). The `Digest` of a phase covers the binary that ran it and the build commit of that binary. It also covers the agent profile and prompt, the report artifact and the worktree tree. Each digest chains to the `Combined` value of the previous phase. The chain replaces the single pipeline-level binary pin as the integrity record.

## Design

- **Leaf package.** It depends on the standard library only. The `DigestSource` and `Provenance` seams are injected, so the package does not import core, ship or git. This keeps it unit-testable and free of import cycles.
- **Content address.** `combine` is a sha256 over the integrity fields and the chain link. It separates the fields with NUL bytes, so a concatenation is unambiguous.
- **Run metadata is not hashed.** `RunID` and `CompletedAt` are not part of `Combined`. A phase that is regenerated with identical content gives an identical digest.
- **Empty report or tree.** `ReportSHA` and `TreeSHA` return an empty string and a nil error when the phase has no report or no worktree.
- **Two independent checks in `Verify`.** Chain integrity (the recomputed `Combined` and the back-pointer) catches a record that was changed afterwards. Provenance catches a binary swap that nobody can verify. A cycle that ran under one binary is the common clean path and needs no provenance.
- **Resume under a rebuild.** When more than one binary took part, the running binary and every differing phase binary need a build commit that `Provenance` accepts. That is an ancestor of HEAD.
- **Empty chain.** `ErrEmptyChain` means no per-phase integrity was recorded (a checkpoint from before the field). The caller falls back to the legacy pipeline check. It is not tamper.

## Invariants

- `Combined` is a function of the integrity fields only. Pinned by the idempotence tests in `digest_test.go`.
- An empty running-binary sha never passes. A chain of records with empty shas must not pass the single-binary fast path, because an unknown binary identity is a tamper posture. Pinned by `verify_test.go`.
- The primitives hold no shared mutable state, so `Compute` and `Verify` are safe from any goroutine. A digest is recorded at the post-phase chokepoint from whatever goroutine ran the phase, and `Verify` runs at ship. Pinned by `concurrency_test.go` under `-race`.
- A chain whose back-pointer is wrong but whose own `Combined` is consistent still fails on the link check. Pinned by `verify_test.go`.

## Findings

- None open.
