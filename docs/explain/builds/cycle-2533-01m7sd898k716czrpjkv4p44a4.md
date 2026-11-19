# Build Explanation — Cycle 2533

## Build Binding
- Cycle: 2533
- Base SHA: 234383500cc7a7a113a70a9889b950ed247c6a6b

## Summary
Incident records in `docs/incidents/` now project into `kbgraph.Graph` as `incident:<id>` nodes with cycle, PR, item, and fingerprint edges. `docs/incidents/` is now a default lesson-recall root in `internal/research`.

## Rationale
Reusing `incidentkb.Dir`, `incidentkb.IsSkipped`, `incidentkb.IsLegacy`, and `incidentkb.Parse` avoids a second YAML frontmatter parser and keeps the leaf package as the one schema owner.

## Changed Areas
- `go/internal/kbgraph/kbgraph.go` — adds `KindAffected`, `KindFixedBy`, and `KindTrackedBy`, and adds `addIncidents` to `Build`. Each `incident-record/1` file creates an `incident:<id>` node with cycle, PR, item, and fingerprint edges. Legacy files are skipped and broken frontmatter returns an error.
- `go/internal/kbgraph/kbgraph_test.go` — adds unit tests for incident graph edges, duplicate signal deduplication, legacy file skipping, and broken frontmatter errors.
- `go/internal/kbgraph/apicover_named_test.go` — adds `KindAffected`, `KindFixedBy`, and `KindTrackedBy` to `TestNamedExports` so every exported constant stays exercised under `apicover`.
- `go/internal/research/kb.go` — appends `docs/incidents/` to `defaultSearchPaths` while leaving explicit `kb_search_paths` configuration exact.
- `go/internal/research/lesson.go` — extends `listLessonFiles` and `parseLessonFile` to read `.md` incident records with `incidentkb.Parse`, skip legacy and skipped files, and extract the `## Lessons` section into `Lesson.PreventiveAction`.
- `go/internal/research/incident_recall_test.go` — adds a unit test verifying keyword recall over multi-paragraph `## Lessons` text and exclusion of legacy and index files.
- `docs/architecture/packages/internal-kbgraph.md` — documents the design and invariants of `internal/kbgraph`.
- `docs/architecture/packages/internal-research.md` — documents the `docs/incidents/` default recall root and Markdown incident-record parsing.

## Design Decisions
Both `internal/kbgraph` and `internal/research` import `internal/incidentkb` rather than parsing frontmatter locally. `addIncidents` runs after `addDossier` and `addInbox` so existing cycle verdicts and inbox item titles stay intact. `parseIncidentLessonFile` checks `rec.Schema == incidentkb.Schema` so non-incident Markdown documents in other search roots never surface as incident lessons.

## Verification
Unit tests in `internal/kbgraph` and `internal/research`, `gofmt`, and `go vet` pass. The cycle 2533 ACS suite checks `evolve kb graph neighbors`, lesson recall, and search-path defaults.

## Compatibility
Existing dossier nodes, inbox item nodes, YAML lesson files, and explicit `kb_search_paths` overrides keep identical behavior.

## Limitations
Incident records without a `## Lessons` body match only on `Title`, `Fingerprint`, and `Effectiveness.Metric.Code`, and non-cycle signals such as `channel:*` do not create graph nodes.
