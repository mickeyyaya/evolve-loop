# Build Explanation — Cycle 2345

## Build Binding
- Cycle: 2345
- Base SHA: 76abf5b42b851c60f7ec22f7b9c0d0d11395814a

## Summary
The failure fingerprint now ignores three per-cycle tokens. These are the `cycle=N` form, the `cycle-N-<ulid>` suffix and the finding count.

## Rationale
Two reasons that differ only in a cycle number gave two fingerprints. The recurrence breaker then counted one defect as many. Folding the tokens in the existing normalizer is the smallest change.

## Changed Areas
- `go/internal/core/failure_digest.go` — the cycle token accepts `=` and a 26-character ULID suffix. A new token folds `N finding(s)`.
- `go/acs/cycle2345/predicates_test.go` — seven predicates call `AssembleFailureDigest`, the production entry.
- `.evolve/evals/rebuild-pr-1252-ad3-1-fingerprint-folds.md` — the eval of the two score caps.

## Design Decisions
The word boundary before `cycle` keeps `recycle=12` distinct. Only the word `finding` folds, so a count of check failures and a file path stay in the fingerprint.

## Verification
The seven predicates pass. The `internal/core` tests pass. The ACS suite reports red=0.

## Compatibility
Stored fingerprints of old failures change once. No schema or policy key changes.

## Limitations
A 26-character suffix that is not a ULID folds too. No such suffix was seen.
