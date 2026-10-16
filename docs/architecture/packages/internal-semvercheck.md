# internal/semvercheck

## Purpose

`internal/semvercheck.IsSemver` is the one allocation-free check that a string is a plain numeric semantic version. Release code (version bump, release consistency, marketplace poll) shares it instead of ad-hoc regexps.

## Design

- **Strict by design.** One compiled regexp matches numeric `X.Y.Z` only: no leading `v`, no pre-release or build metadata, because the codebase mints and validates plain release tags only.
- **A leaf.** It depends on the standard library only, so any layer can validate a version without a new dependency.

## Invariants

- Callers must not widen the accepted form. A new form is a decision for every release reader at once.
