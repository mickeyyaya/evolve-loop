# knowledge-base/ — runtime state, not documentation

This directory is a **runtime write surface**, not part of the documentation
tree. `cycles/` receives per-cycle dossier JSON committed by loop lanes
(`go/cmd/evolve/cmd_loop_outcome.go`), and boot recovery deliberately excludes
this root from quarantine. Do not add documentation here.

All documentation lives under [`docs/`](../docs/index.md). The research notes
that previously lived in `knowledge-base/research/` moved to
[`docs/research/`](../docs/research-index.md) (2026-08-05; see
[docs/MOVED.md](../docs/MOVED.md)).
