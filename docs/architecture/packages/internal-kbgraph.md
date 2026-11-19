# internal/kbgraph

> Plan: [incident knowledge base](../../plans/incident-knowledge-base-2026-11.md), component I8 and decision D7.

## Purpose

`internal/kbgraph` projects committed cycle dossiers, inbox items, and incident records into a read-only in-memory graph.

## Design

- **Graph build.** `Build` reads committed dossiers, `.evolve/inbox/` JSON items, and `docs/incidents/` Markdown records into sorted adjacency lists.
- **Incident records.** `addIncidents` reuses `incidentkb.Dir`, `incidentkb.IsSkipped`, `incidentkb.IsLegacy`, and `incidentkb.Parse` with no second parser.
- **Incident edges.** Each schema record adds an `incident:<id>` node with `affected` edges to `cycle:<N>`, `recurs-as` to `fingerprint:<fp>`, `fixed-by` to `pr:<N>`, and `tracked-by` to `item:<id>`.
- **Signal filter.** Only signals shaped as `cycle:<N>:<seq>` with a positive cycle number create `cycle` nodes; channel and other signals create no cycle node.
- **Legacy and errors.** Legacy files and skipped index or session-log files add no node; unparsable schema records fail `Build` and name the file path.
- **Bounded neighborhood.** `Neighbors` walks up to `MaxHops` (2) and stops at `MaxNodes` (40).

## Invariants

- The enforced `apicover` gate lists `./internal/kbgraph`, and `apicover_named_test.go` exercises every exported symbol including `KindAffected`, `KindFixedBy`, and `KindTrackedBy`.
- `evolve kb graph neighbors` and `evolve kb graph recurrences` in `go/cmd/evolve/cmd_kb.go` are the production callers.
