# internal/incidentkb

> Plan: [incident knowledge base](../../plans/incident-knowledge-base-2026-11.md), component I2 and decisions D1, D2 and D11.

## Purpose

`internal/incidentkb` parses, validates, renders, and lists Markdown incident records under `docs/incidents/`. It enforces the `incident-record/1` frontmatter schema, the ten section order, and the generated status, actions, and effectiveness regions.

## Design

- **Leaf package.** `internal/incidentkb` imports only the Go standard library. It parses the YAML frontmatter subset with stdlib code so no internal or third-party import enters the leaf.
- **Renderer.** `Render` parses the frontmatter, validates fields, and fills the `status`, `actions`, and `effectiveness` marker pairs deterministically. Prose outside the markers stays unchanged, and a second call returns identical bytes.
- **Check.** `CheckBytes`, `Check`, and `CheckAll` validate frontmatter fields and verify that the filename matches `id`. They check the ten `## ` headings in order and compare each generated region against `Render`. Every failure names its field or section heading.
- **Legacy rule.** Files without frontmatter that have no date prefix or a date before `2026-11-19` are legacy. `IsLegacy` reports them without failure. Files dated on or after `2026-11-19` without frontmatter fail. `IsSkipped` skips `REGRESSION-COVERAGE-INDEX.md` and `<yyyy-mm-dd>-session-log.md`.
- **List.** `List` reads all record and legacy files, skips index and session-log files, and filters by `--status` and `--code`.

## Invariants

- The enforced `apicover` gate lists `./internal/incidentkb`, and `apicover_named_test.go` executes every exported symbol.
- `evolve issue check` and `evolve issue list` in `go/cmd/evolve/cmd_issue.go` are the production callers.
