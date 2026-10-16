# internal/digest

## Purpose

`internal/digest` projects a role-scoped subset out of an instruction document. That document is the single source of truth. Phase instruction skill files are injected whole into every phase. A narrow role (scout, for example) never acts on most of the ship-gate and audit content in them. A role-scoped digest sends that role only what it needs. The runner calls it from `internal/phases/runner/preparation.go`.

## Design

- **Authors tag the sections.** A block opens with `<!-- digest:role=scout -->` and closes with `<!-- /digest -->`. The role list is comma-separated, and `ProjectDigest` trims the space around each entry.
- **`ProjectDigest` concatenates the matching blocks.** A block tagged for other roles, and any prose outside a marker pair, is excluded.
- **`Materialize` classifies the projection.** The outcome is `matched`, `no_match` (nothing tagged for the role) or `malformed` (the projector returned an error). It never falls back to the source.
- **`NewShadowRecord` derives size facts** (`FullBytes`, `DigestBytes`) and `Parity`. `Parity` is true only for a non-blank, matched digest that is smaller than the full source. It does not change the live input.

## Invariants

- **No fallback to the full source.** A role with no matching block gets an empty, non-nil digest. It does not get the full source, and it does not get an error. So a pass-through implementation cannot satisfy this contract by accident. Pinned by the `no matching block yields empty, not full source` and `source with no markers yields empty digest` cases of `TestProjectDigest`.
- **A broken marker is a hard error, never a silent truncation.** An opening tag with no ` -->` terminator, or with no `<!-- /digest -->` before the end of the file, returns an error. Pinned by the `unterminated marker is a hard error` case of `TestProjectDigest` and by `TestProjectDigest_AnOpeningTagWithNoTerminatorIsAHardError`.
- **A role list is a set.** One block serves every role it lists. Pinned by the `multi-role marker matches every listed role` case.
- **Only the tagged role's content survives.** Pinned by the `extracts only the tagged role's content` case. `TestAPICoverNamedExports` drives the happy path, the negative path and the error path through `ProjectDigest`.
- **The digest is smaller than the source on a realistic document.** Pinned by the `byte reduction on an excluded-heavy fixture` case.
- **Outcome classification and parity.** Pinned by `TestMaterializeAndShadowRecord`.

## Findings

- The first increment of this projector (role-scoped instruction digests) is in the [comment history archive](../../history/code-comments/internal-digest.md).
