# Build Explanation — Cycle 1736

## Build Binding
- Cycle: 1736
- Base SHA: b22dea3b419d4bbffffb4237936ca5b5d6d64a23

## Summary
The four oversized `internal/skillcheck` functions (`Run` 113 lines, `commandDiffs` 71, `ManifestProblems` 69, `collectSkillFacts` 51) now fit the 50-line `sizeratchet.MaxLines` limit, and their allowances are deleted from `offenders.json`. The change is a behavior-preserving extraction, pinned first by new characterization tests.

## Rationale
The size ratchet only shrinks: once a function fits the limit, its allowance has to be deleted so the function cannot grow back. Pure extraction is the smallest change that gets each function under 50 lines. Each moved block keeps its code and comments verbatim, so the audit's mutation anchors and comment audit still line up with the baseline. Before any extraction, characterization tests were added and run against the baseline. They kill all 20 behavior mutants that the existing suite let survive, so the refactor is checked by tests that fail when behavior changes.

## Changed Areas
- `go/internal/skillcheck/skillcheck.go` — `Run` delegates the three projection surfaces (phase-facts, command stubs, Codex manifests) to `projectSurfaces`. `projectCommandDiffs` now handles both the command-stub and Codex write/report loops, which were identical apart from an orphan branch that Codex diffs never take. `writeGenerated` replaces three copies of the atomic-write-and-announce block. `collectSkillFacts` delegates contract resolution to `phaseContract`. Write and remove failures come back as wrapped errors, and `Run` prints them with the same `"%v\n"` format, so the output bytes do not change.
- `go/internal/skillcheck/manifest.go` — `ManifestProblems` delegates step 1 (declared skills) to `declaredSkillProblems` and step 3 (agents) to `agentProblems`. Step 2 stays inline. The step comments stay at the call sites.
- `go/internal/skillcheck/commands.go` — `commandDiffs` delegates the orphan scan to `orphanCommandDiffs`. The orphan-reap guards and the sort stay where they were.
- `go/internal/skillcheck/characterization_test.go` — adds characterization tests that pin the manifest problem wording, sort order and infra-error contract; `Run`'s WARN prefix, drift and remedy lines, orphan reap line, check-OK line and write-mode exit code; `collectSkillFacts`' artifact-name suppression and write-target label; and `commandDiffs`' skip, name-fallback, non-markdown and orphan-ordering rules.
- `go/internal/sizeratchet/offenders.json` — deletes the four `internal/skillcheck` allowances.
- `go/acs/cycle1736/predicates_test.go` — the TDD phase's acceptance predicates for this lane (not edited by Build).
- `.evolve/evals/sizeratchet-shrink-skillcheck.md` — the TDD phase's eval contract for this lane (not edited by Build).

## Design Decisions
Codex diffs go through the same `projectCommandDiffs` loop as command diffs. `codexManifestDiffs` never sets `orphan`, so the loop's non-orphan branches match the old Codex loop exactly, and one loop replaces two copies. None of the new helpers has a doc comment, because the package's comment contract forbids adding comment lines. None is exported.

## Verification
- `go test -count=1 ./internal/skillcheck` passes.
- `go vet` is clean.
- All ten cycle-1736 ACS predicates pass: the size limit, the offender removal, the module-wide ratchet check, the frozen baseline tests, docs identical to baseline, no comments added or deleted, and all 20 behavior mutants killed.
- The 20 mutants were confirmed killed on the baseline code before any extraction.

## Compatibility
Exported signatures, CLI output bytes, exit codes and error texts do not change.

## Limitations
This lane changes only the size of these four functions. Offender allowances for other packages are left as they are.
