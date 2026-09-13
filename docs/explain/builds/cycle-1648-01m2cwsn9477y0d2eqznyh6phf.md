# Build Explanation — Cycle 1648

## Build Binding
- Cycle: 1648
- Base SHA: 4dd162febbcbec14da4e2d54bbd250fe2db50a36

## Summary
The new read-only `evolve audit calibration` command turns committed cycle dossiers and runtime audit shadows into a deterministic Markdown report of narrative-versus-gate outcomes, overrides, defect classes, and exclusions.

## Rationale
The existing dossier and audit-chain schemas already preserve the required evidence, so the implementation reads those types directly and adds one small aggregation package. Invalid or incomplete pairs are recorded as exclusions instead of being silently dropped, because silent omission would inflate apparent agreement.

## Changed Areas
- `.evolve/evals/auditor-calibration-report.md` — defines executable acceptance criteria for CLI reachability, outcome separation, exclusions, determinism, and API coverage.
- `.evolve/inbox/2026-07-30T09-02-00Z-auditor-calibration-report.json` — removes the completed task record from the active inbox as the source side of its lifecycle move.
- `.evolve/inbox/consumed/2026-07-30T09-02-00Z-auditor-calibration-report.json` — preserves that task record under the consumed inbox so completion history remains available without leaving the task dispatchable.
- `go/.apicover-enforce` — enrolls the new internal package in the repository-wide exported-API coverage gate.
- `go/acs/cycle1648/harness_test.go` — supplies the real-CLI fixture corpus and report matchers used by the cycle predicates.
- `go/acs/cycle1648/predicates_test.go` — exercises all acceptance criteria through the production dispatcher and real corpus.
- `go/cmd/evolve/cmd_audit_calibration.go` — validates the nested command and paths, resolves project-root defaults, generates the report, and writes it atomically.
- `go/cmd/evolve/cmd_audit_calibration_test.go` — proves invalid-root failure and deterministic Markdown through the CLI dispatcher.
- `go/cmd/evolve/main.go` — documents `audit calibration` and its explicit path flags in help output.
- `go/cmd/evolve/registry.go` — registers the top-level `audit` command in the production dispatcher.
- `go/internal/auditcalibration/apicover_named_test.go` — names and executes the package's exported `Generate` API for the API-coverage gate.
- `go/internal/auditcalibration/auditcalibration.go` — samples canonical run directories, validates and aggregates artifact pairs, and renders stable Markdown tables.
- `go/internal/auditcalibration/auditcalibration_test.go` — covers missing/malformed pairs and the narrative-PASS force-override edge case.

## Design Decisions
The sample is defined by canonical `cycle-N` run directories because that is where an auditor narrative can exist; dossier-only records are outside the sample. A deterministic gate is FAIL exactly when `overrode_by` is non-empty, while chain and shipped verdicts remain separate report columns. The package exposes only `Generate`; command parsing and atomic persistence stay at the CLI boundary.

## Verification
Builder unit tests cover malformed and missing artifacts, the force-override case, invalid roots, and repeated byte-identical output. Cycle ACS predicates exercise the real dispatcher, fixture matrix, real cycle-1640 evidence, API coverage, and regression slices.

## Compatibility
The change is additive and read-only. Existing dossier and audit-shadow producers are unchanged, legacy shadows without `shipped_verdict` fall back to the dossier's final verdict, and explicit corpus directories override project-root defaults.

## Limitations
The report describes observed evidence and intentionally makes no persona-policy recommendation from a single cycle. Defect classes use the existing reason prefix before the first colon; unstructured reasons remain distinct full-text classes.
