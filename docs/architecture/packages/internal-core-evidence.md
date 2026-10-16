# internal/core/evidence

## Purpose

`internal/core/evidence` implements the commit-as-evidence trailer of ADR-0027. It is the provenance block appended to the commit of a phase on the branch of the per-cycle worktree. A phase has delivered if and only if it produced a commit that carries a well-formed `Evolve-Phase` trailer with the challenge token of the cycle. Completion is "HEAD advanced and the trailer verified". It is never "a file appeared at the polled path".

## Design

- **Pure.** The package uses the stdlib only and imports nothing from the bridge, orchestrator or git layers. The completion detector (bridge) and the commit emitter (orchestrator) can both depend on it without an import cycle.
- **Keys.** The `Evolve-*` keys are the namespace of this project. `Challenge-Token` reuses the per-launch challenge token that the bridge already mints.
- **`Trailer`.** `Phase` and `Challenge` are load-bearing for completion verification. The artifact fields are forensic provenance: which deliverable the commit evidences.
- **`Build`.** It renders a leading blank line, then one `Key: Value` line for each non-empty field, in a stable order. Deterministic output gives reproducible commits and is cache friendly. `Cycle` is emitted when it is above 0.
- **`Parse`.** It reads the trailing block of the message: the contiguous run of trailer lines at the end. It walks backward from the end. Unknown keys are ignored, and missing keys leave the zero value. A message without a trailer block gives a zero `Trailer`.
- **Key grammar.** A key is a non-empty run of `[A-Za-z0-9-]` (`cutTrailer`, `isKeyByte`). `isTrailerLine` accepts prose such as `see: http://...` only loosely. The backward contiguous-run scan in `Parse` is what bounds the block.

## Invariants

- **The block is contiguous.** `Build` emits no blank line between trailer lines, because a blank line ends the trailer run that `Parse` reads. This is what makes `Parse` round-trip `Build`.
- **Body colons are not trailers.** An earlier body line that contains a colon is never read as a trailer, because only the final contiguous run counts.
- **`Verify` fails closed.** It needs a non-empty phase and a non-empty expected token, and both must match. An empty expected token is rejected, so a missing token can never satisfy verification. A trailer with an empty `Phase` builds a block that `Verify` can never accept, so every emitter must set `Phase`.
