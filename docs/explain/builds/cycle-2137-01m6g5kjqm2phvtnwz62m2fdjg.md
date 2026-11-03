# Build Explanation — Cycle 2137

## Build Binding
- Cycle: 2137
- Base SHA: 2ba8b562b1084663a97bd8ddc2d8a0c718cce348

## Summary
The default search paths for knowledge-base recall now include `docs/retrospectives/`.

## Rationale
Retrospective lessons provide recovery insights that the recall memory must surface during goal evaluation.

## Changed Areas
- `go/internal/research/kb.go` — appends `docs/retrospectives/` to the default search paths constant.
- `docs/architecture/packages/internal-research.md` — documents the updated default search roots in the package architecture note.

## Design Decisions
The default roots append `docs/retrospectives/` as a flat root without change to directory traversal. Policy configuration through `kb_search_paths` still replaces the full list.

## Verification
ACS predicates for cycle 2137 pass with all 6 predicates green. Package tests and cycle seam tests in `cmd/evolve` pass.

## Compatibility
`SearchPathsFromEnv` preserves precedence for policy overrides. It preserves flat root search.

## Limitations
Nested subdirectories under `docs/retrospectives/` are ignored because root traversal remains flat.
