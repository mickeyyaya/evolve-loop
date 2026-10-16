# Build Explanation — Cycle 1894

## Build Binding
- Cycle: 1894
- Base SHA: 7a6b8e8a40d7b2819a9ea89b10deec1c9e5c938d

## Summary
The new research document explains the recent agy CLI changelog updates as data and evaluates each core persona under ADR-0133. The build also fixes `evolve docs ste-lint`. A relative path that is absent from the current directory now resolves against `--project-root`.

## Rationale
The research document treats the agy changelog output as untrusted data. This prevents prompt injection and keeps the measured baseline that ADR-0133 requires.

Predicate `TestC1894_005_DocPassesSteLint` caused the ste-lint fix. The predicate runs ste-lint with the current directory set to `go/`, and it gives the repo-relative path `docs/research/practice-agy-cli-changelog-2026-10.md` with `--project-root` set to the repo root. Before the fix, ste-lint resolved that path against `go/` only, so it did not find the file. The fix makes ste-lint use `--project-root` as the base for such a path, which agrees with the meaning of that flag.

Rejected alternative: an absolute path in the predicate. The predicate is owned by the TDD phase, so the build must not edit it. Also, an absolute path only hides the defect, and other callers that run ste-lint outside the repo root keep the failure.

## Changed Areas
- `docs/research/practice-agy-cli-changelog-2026-10.md` — documents the agy changelog updates as data and gives the ADR-0133 persona evaluation.
- `go/cmd/evolve/cmd_docs.go` — `explicitSteTargets` now tries `filepath.Join(root, p)` when a relative path `p` is absent from the current directory, so predicate 005 can lint the document from `go/`.

## Design Decisions
The core phase personas do not change in this cycle. They wait for the baseline measurements of milestone M2 of the prompt diet.

The ste-lint fallback has three conditions. The path is relative, it is absent from the current directory, and the joined project-root path exists. A path that is present in the current directory keeps its old resolution, so the current directory has priority.

## Verification
The ACS predicates of cycle 1894, the practicewatch test suite and ste-lint verify the deliverable. The unit test `TestExplicitSteTargets_FallsBackToProjectRootWhenNotInCwd` in `go/cmd/evolve/cmd_docs_test.go` covers the fallback.

## Compatibility
The build changes one CLI behavior. Before, `evolve docs ste-lint <relative-path>` failed when the path was absent from the current directory. Now, ste-lint also tries the path against `--project-root`, and it lints the file if the file is there. A path that is present in the current directory, an absolute path, or a path that is absent from both locations keeps its old result. No API, interface or flag changes.

## Limitations
The prompt diet reductions for evolve-builder wait for milestone M3, after the baseline measurements.
