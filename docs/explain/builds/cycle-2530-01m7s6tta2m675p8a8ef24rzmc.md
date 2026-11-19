# Build Explanation — Cycle 2530

## Build Binding
- Cycle: 2530
- Base SHA: b32f7063f2dc92ef18c47353718b941eed47eef2

## Summary
Added the leaf package `internal/incidentkb` and wired `evolve issue check` and `evolve issue list` in `cmd/evolve` for `incident-record/1` incident records.

## Rationale
Using a stdlib-only YAML subset parser inside `internal/incidentkb` preserves the leaf invariant with no internal or third-party imports. It implements schema validation, deterministic rendering of generated regions, the ten-section check, and the pre-2026-11-19 legacy rule.

## Changed Areas
- `go/internal/incidentkb/types.go` — defines the `incident-record/1` schema types, required section headings, and list/check report types.
- `go/internal/incidentkb/yaml.go` — implements the stdlib-only frontmatter splitter and YAML subset parser.
- `go/internal/incidentkb/render.go` — implements deterministic, idempotent rendering of the `status`, `actions`, and `effectiveness` marker regions.
- `go/internal/incidentkb/check.go` — implements frontmatter, heading-order, rendered-drift, legacy, skipped-file, and list checks.
- `go/internal/incidentkb/apicover_named_test.go` — exercises every exported symbol of `internal/incidentkb`.
- `go/internal/incidentkb/incidentkb_test.go` — unit tests for block YAML, rendered results, section ordering, and unknown field errors.
- `go/cmd/evolve/cmd_issue.go` — merges `incidentkb` record checks into `evolve issue check` and adds `evolve issue list`.
- `go/cmd/evolve/cmd_issue_test.go` — adds unit tests for record checks, legacy reporting, and `issue list` filters.
- `go/cmd/evolve/main.go` — updates the `issue` help text to list `list`.
- `go/cmd/evolve/registry.go` — updates the `issue` registry summary to list `list`.
- `go/.apicover-enforce` — enrolls `./internal/incidentkb` in the repo-wide API coverage gate.
- `docs/architecture/packages/internal-incidentkb.md` — documents the purpose, design, and invariants of `internal/incidentkb`.
- `docs/architecture/packages/README.md` — indexes `internal/incidentkb`.

## Design Decisions
`Render` replaces only the byte ranges between each `<!-- GENERATED:<part> BEGIN -->` and `<!-- GENERATED:<part> END -->` marker, leaving prose outside those markers untouched so repeated runs are idempotent. `evolve issue check` merges bundle and record checks while keeping existing `incidentbundle` exit codes and outputs intact.

## Verification
`cd go && go test -count=1 ./internal/incidentkb/... ./cmd/evolve -run 'Issue|Incident|APICover'`, `make -C go apicover-enforce APICOVER_PKGS=./internal/incidentkb`, and `evolve acs suite --cycle 2530` all pass (all 17 cycle-2530 predicates pass; `green=288 red=0`).

## Compatibility
All existing `incidentbundle` evidence commands and `cmd_issue_test.go` assertions remain unchanged, and pre-2026-11-19 records under `docs/incidents/` are reported as legacy without failing `issue check`.

## Limitations
`evolve issue check` and `evolve issue list` are not class `A` in `go/internal/agentclass/table.go`. Sandboxed calls emit `[isolation]` and `[events]` lines to `stderr`, while `stdout` stays clean JSON.
