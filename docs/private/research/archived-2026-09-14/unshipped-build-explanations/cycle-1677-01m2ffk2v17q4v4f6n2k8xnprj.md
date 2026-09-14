# Build Explanation — Cycle 1677

## Build Binding
- Cycle: 1677
- Base SHA: 83a019aa0fa7fe6b50d91b2bb84070805a3cab45

## Summary
A successful `evolve ledger verify` now states which history it accepted — either that it validated every line from genesis, or the identity of the epoch anchor its strict walk resumed from — instead of printing one `OK: chain intact` string for both claims.

## Rationale
The epoch-anchor resolver landed in cycle-1191 and is correct: a chain whose adjudicated damage sits behind an eligible operator seal verifies, and a break past that seal still fails. What was missing is observability at the only surface an operator sees. "I checked all 141,688 lines" and "I deliberately trusted a 136,000-line prefix an operator signed off on" are different claims about different amounts of evidence, and printing them identically is the shape that let the ledger-1740 damage stay invisible as long as it did.

The provenance is derived from the ledger rather than described in prose, so it cannot drift: two ledgers sealed at different lines report different anchors, and a chain with no anchor reports none. It is produced by the same single walk that performed the validation rather than by a second, separately resolved lookup, so the scope reported is necessarily the scope verified — a second resolution could disagree with the first if the file changed between them, which would make the reassurance worse than silence.

The anchor's `entry_seq` is read from the resolved line, not from `ledger-anchor.json`'s `anchor_seq` field. This is not a hypothetical distinction: on the live ledger the sidecar records `anchor_seq=113890` while an in-band operator seal has since moved the effective anchor to `entry_seq=136212`. Reporting the sidecar's number would misname the trusted prefix by roughly 22,000 lines, and would do so in the reassuring direction.

## Changed Areas
- `go/internal/adapters/ledger/anchor.go` — adds the `VerifiedScope` type naming what a verification accepted, and makes the existing anchor resolver return the resolved line's own `entry_seq` alongside its SHA; the seq is taken during the pass that already hashes every line, so no second traversal of the chain is introduced.
- `go/internal/adapters/ledger/ledger.go` — splits `Verify` into a thin wrapper over a new `VerifyScope`, which performs the same single walk and returns the anchor it resumed from; a failed walk or tip check returns the zero scope, so a rejected chain can never hand back a sealed-prefix claim.
- `go/internal/adapters/ledger/seal.go` — gives the deep path the same treatment via `VerifyDeepScope`, with `VerifyDeep` reduced to a wrapper, so both production verification paths report identical provenance.
- `go/cmd/evolve/cmd_ledger.go` — `runLedgerVerify` renders the scope on the success line through a new `verifiedFrom` helper, naming the anchor by `entry_seq` and full line SHA so the operator can carry the pair straight to `evolve ledger anchor --line-sha`.
- `go/internal/adapters/ledger/verify_scope_test.go` — covers the full-strict case, the in-band-seal case, the sidecar-versus-line seq regression, default-and-deep parity, and the refusal case where a forged tail must yield an error and no scope.
- `go/internal/adapters/ledger/seal_role_trust_test.go` — updates the two existing resolver call sites for the widened return; the role-trust assertions themselves are unchanged.
- `.evolve/evals/ledger-verify-seal-anchor.md` — records the TDD-owned score caps for this contract.
- `go/acs/cycle1677/predicates_test.go` — supplies the TDD-owned acceptance predicates driving the real CLI.

## Design Decisions
The `core.Ledger` port keeps its `Verify(ctx) error` signature, so the guard and orchestrator callers that only need a verdict are untouched; the scope-returning methods are additions on `*FileLedger` for the one caller that renders it. The resolver keeps the name `effectiveAnchorSHA` and its SHA-first return value despite now also reporting a seq, because that symbol is named in `knowledge/architecture/state-and-ledger.md` as the documented tip-witness re-derivation path and renaming it would leave that documentation describing a symbol that no longer exists.

Two alternatives were rejected. Resolving the anchor independently inside `cmd_ledger.go` would have left the production code untouched, but it costs a second SHA-256 pass over a 44 MB file and opens a window in which the printed provenance is not the one that was verified. Reporting only the anchor's line SHA would have satisfied the acceptance criteria with a smaller diff, but `entry_seq` is the identifier the operator's own commands speak, and for an in-band seal it is recorded nowhere else. No new package, policy knob, or environment flag was added.

## Verification
The five cycle-1677 ACS predicates drive the real `evolve` binary and pass: a sealed prefix verifies and names its anchor, two fixtures differing only in seal identity produce two correct and non-interchangeable outputs while a no-anchor chain claims none, a break one line past the anchor still exits 2 as a broken chain, the live 141,688-line ledger verifies and reports its scope without being mutated, and `--deep` reports the same provenance as the default path. Unit tests in the ledger package cover the same behaviors at the seam. The native ACS suite reports `green=170 red=0` over 223 predicates, and `gofmt`, `go vet`, and the ledger and `cmd/evolve` package suites are clean.

## Compatibility
Exit codes, the `BROKEN` line, the `OK:` prefix, and the scope words that precede it are unchanged; the provenance is appended after an em dash, so a reader matching the existing prefix still matches. `Verify` and `VerifyDeep` keep their signatures and behavior, and the `core.Ledger` port is untouched. A ledger with no anchor prints its full-strict claim and is otherwise byte-identical in behavior to before.

## Limitations
The provenance reports the anchor the walk resumed from, not how many lines sit behind it; an operator who wants that count still derives it from the seq. A stale or tampered sidecar anchor continues to fail loudly inside the walk rather than being reported as a scope, since such a verification cannot succeed in the first place.
