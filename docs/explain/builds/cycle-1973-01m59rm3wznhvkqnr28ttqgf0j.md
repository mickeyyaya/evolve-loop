# Build Explanation — Cycle 1973

## Build Binding
- Cycle: 1973
- Base SHA: de5a2b7f6df50f0efd11266663a9bba11606d1ce

## Summary
A new package, `internal/retryladder`, classifies a failed try from host facts and fingerprints its reason.

## Rationale
ADR-0142 needs a deterministic class before any rung can rise. A stronger model cannot fix a spec failure (cycles 1946, 1950, 1953), so the class must come before the climb. The package imports no `core` code, so `core` can call it later without a cycle.

## Changed Areas
- `go/internal/retryladder/classify.go` — the `Class`, `Facts` and `Verdict` types and `Classify`, with the fixed priority order.
- `go/internal/retryladder/fingerprint.go` — `Fingerprint`: the violation-code set, else the normalized text.
- `go/.apicover-enforce` — enrolls the package in the repo-wide unnamed-export gate.
- `docs/architecture/packages/internal-retryladder.md` — the package page.

## Design Decisions
The normalizer duplicates five lines of regexes from `core/failure_digest.go`, because importing `core` is forbidden. Moving them to a shared package is out of scope.

## Verification
`go test -count=1 -race ./internal/retryladder` passes. `go test -tags acs ./acs/cycle1973` passes. `evolve acs suite --cycle 1973` reports red=0.

## Compatibility
The package is new and has no caller yet. No existing behavior changes.

## Limitations
No production code calls `Classify` yet. The wiring into the correction paths is a later step of the plan.
