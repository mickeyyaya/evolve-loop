# Build Explanation — Cycle 1768

## Build Binding
- Cycle: 1768
- Base SHA: e404b914d41968fc2ba4124694c46f5dd7826b7b

## Summary
`phasecoherence.Check` (80 lines), `phasecoherence.CheckArtifactNames` (97 lines) and `phasecoherence.CheckProvenance` (95 lines) were the three live offenders in `go/internal/sizeratchet/offenders.json` for this package. Each is now at or under the repo-wide 50-line function-size ratchet, reached by extracting the per-entry (or per-field) decision each function was making into a named helper, with the outer function reduced to iterating and dispatching. No behavior changed: the package's existing 14 test files pass unmodified, and `offenders.json` is untouched.

## Rationale
All three functions shared the same shape: a loop over `fs.ReadDir` entries (or, for `CheckProvenance`, a sequence of independent field comparisons) that inlined validation, lookup and violation-construction for a single entry/field per iteration. That single-entry decision is the natural extraction seam — it is already a self-contained unit of "does this one thing produce a violation," it has one exit-of-the-loop concern (`continue`/`skip`) that maps cleanly onto an early `return nil, nil` from a helper, and splitting there needs no new state to thread back into the caller beyond the accumulated violations slice. The rejected alternative was extracting the `fs.ReadDir`/setup preamble into its own helper instead: that would have shrunk the outer function by only a handful of lines (the preamble is short) while leaving the large per-entry `if`/`continue` chain — the actual 80/97/95-line bulk — untouched, so it would not have reached the 50-line limit. For `CheckProvenance`, the three independent concerns (parse the header, check its fields against `expected`, cross-check the ledger) already had no data dependency on each other beyond the parsed map and the direct tree-SHA-mismatch flag, so they split into `parseProvenanceKV`, `checkProvenanceFields` and `checkLedgerTreeSHA` without any behavior change.

## Changed Areas
- `go/internal/phasecoherence/coherence.go` — `Check` now loops calling the new `checkPersonaEntry`, which holds the per-persona profile/frontmatter/tools logic previously inlined in the loop body; the new `unpairedViolations` holds the `-reference`/`dispatch: none` exemption that used to guard the unpaired-persona violation inline.
- `go/internal/phasecoherence/artifact_coherence.go` — `CheckArtifactNames` now loops calling the new `checkArtifactNameEntry` for the per-persona frontmatter/output-format lookup, and the new `artifactNameViolation` for the two output-artifact-mismatch cases (missing `output_artifact` vs. a declared/profile basename mismatch) that used to be inlined at the bottom of the loop.
- `go/internal/phasecoherence/provenance.go` — `CheckProvenance` now calls the new `parseProvenanceKV` (header key/value extraction), `checkProvenanceFields` (phase/cycle/inputs-digest/tree-SHA comparisons against `expected`, also returning whether a direct tree-SHA mismatch already fired) and `checkLedgerTreeSHA` (the ledger-file scan and cross-check, skipped when a direct mismatch already fired).
- `go/acs/cycle1768/helpers_test.go` — pre-existing TDD-phase deliverable (ACS predicate helper) for this task; unmodified by this build.
- `go/acs/cycle1768/predicates_test.go` — pre-existing TDD-phase deliverable (ACS predicates) for this task; unmodified by this build.
- `.evolve/evals/sizeratchet-shrink-phasecoherence.md` — pre-existing TDD-phase deliverable (eval graders) for this task; unmodified by this build.

## Design Decisions
Each extracted helper takes exactly the inputs its single decision needs (`opts`, the shared `*profiles.Loader`, and the current `fs.DirEntry` for the two persona-loop helpers; the parsed map and `expected` fields for the provenance helpers) and returns either `(*Violation, error)`, `([]Violation, error)`, or a plain `[]Violation` where no I/O or config error is possible — matching each original function's own error-only-on-config-or-I/O-failure contract. No new exported identifiers were introduced (all extractions are unexported), so no ratchet-adjacent export-naming or caller-proof obligations apply beyond the existing package tests.

## Verification
`go build ./...`, `gofmt -l internal/phasecoherence/` (clean) and `go vet ./internal/phasecoherence/...` (clean) all pass. `go test -count=1 ./internal/phasecoherence/...` passes (behavior-unchanged pin, AC 1). `go test -tags acs -count=1 ./acs/cycle1768/...` passes all six ACS predicates (`TestC1768_001`–`TestC1768_007`), confirming each of the three functions now measures ≤50 lines via `sizeratchet.Walk`, `offenders.json` is byte-unchanged for the three keys, `sizeratchet.Check` reports zero violations, no baseline `_test.go` file changed, and no comment lines were added. `evolve acs suite --cycle 1768 --root ..` (run from `go/`) reports `verdict=PASS green=173 red=0 skip=53 total=226`.

## Compatibility
No exported API, CLI flag or config changed; all new identifiers are unexported package-internal helpers. `offenders.json` is untouched, so the module-wide ratchet ceiling for these three keys stays exactly where it was (80/97/95), with the shrink recorded only as slack for a future boundary-tighten cycle, per this task's explicit scope.

## Limitations
This lane only shrinks the three named functions to fit the ratchet; it does not tighten their `offenders.json` allowances (that is explicitly a separate boundary-tighten lane's job) and does not sweep the rest of the repo for other ratchet offenders outside `go/internal/phasecoherence`.
