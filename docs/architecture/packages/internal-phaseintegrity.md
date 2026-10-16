# internal/phaseintegrity

> The per-phase integrity chain that this package feeds: [ADR-0065](../adr/0065-per-phase-binary-integrity.md).

## Purpose

`internal/phaseintegrity` is the production `DigestSource` of the per-phase integrity chain. It reads the running binary and its build-commit. It also reads the agent profile or prompt, the report artifact and the worktree tree. It feeds them to `phaseblock.Compute`. It holds the sanctioned re-pin of `state.json:expected_ship_sha` after a rebuild (`RepinShipSHA`, `RepinIfDrifted`). The package is non-leaf glue: it runs git for the tree sha, and the `phaseblock` leaf stays git-free.

## Design

- **Injected provenance.** `ProvenanceVerified` asks "is this build-commit an ancestor of HEAD?". It is injected, so the decision path has no hard git dependency.
- **One drift path.** `RepinIfDrifted` is the single path that detects drift, gates on provenance and re-pins. Boot recovery calls it, and so does the step right after a successful build phase. Thus the two never diverge.
- **Source is best-effort.** An empty path field is skipped and its digest stays `""`. A profile or report file that is set but missing also gives `""`. A digest capture never blocks a cycle.
- **`GitTree` is the seam for the worktree tree sha.** The default is `git write-tree`. Tests inject a stub.
- **The tree sha is the staged index.** `TreeSHA` reflects the staged index identity, not the full working tree. A phase that does not stage its output is not reflected. It gives `""` when the phase ran with no per-cycle worktree.
- **Bounded git.** `gitTreeTimeout` bounds the `git write-tree` exec. A slow or huge worktree, or a stuck filesystem, cannot hang a phase boundary.

## Invariants

- **A re-pin needs trust.** `RepinShipSHA` re-pins in two cases only. The first case: `runningCommit` is not empty and `prov(runningCommit)` is true, which is a legitimate rebuild from committed source. The second case: the operator passed `--reset-sha`. In all other cases it refuses with `Repinned=false` and an actionable error. This keeps the anti-tamper guarantee.
- **`RepinIfDrifted` is never operator-authorized.** Only unattended code reaches it.
- **Outcomes of `RepinIfDrifted`.**
  - No pin (or an unreadable or torn state file), an absent binary, or a sha that equals the pin: `Repinned:false` and no error. No write happens and provenance is not consulted, because there is nothing to authorize. An absent binary gives no mismatch signal, so it fails open as a no-op.
  - Drift and verified provenance: `RepinShipSHA` fires. The result is `Repinned:true` with `NewSHA`.
  - Drift and unverified provenance: `Repinned:false` and an error. The pin stays untouched.
- **The target is a fixed absolute path.** The target of a security control is never a relative or traversal path from a caller. This is defense in depth.
- **A re-pin updates an existing pin. It never creates `state.json`.** `statemap.UpdateStateMap` tolerates a missing file by design. Thus the existence check comes before the read-change-write. A missing file means that there is nothing to re-pin.
- **The write goes through the `statemap` adapter.** The adapter holds the shared `<state.json>.lock` sidecar across the whole read-change-write. The re-pin is thus safe against concurrent resume and fleet writers, and it keeps every unmodelled key. A malformed file stops it before the write.
- **A stat error other than not-exist is surfaced.** `shaOrEmpty` gives `""` only for an empty path or a file that is absent. A not-yet-written artifact is not a capture failure. An error or a bad path that gave `""` records the same digest as "no artifact". That is a silent integrity gap.
- **The worktree path is absolute.** `defaultGitTree` checks this. A path with a leading dash is thus never read as a git flag.
