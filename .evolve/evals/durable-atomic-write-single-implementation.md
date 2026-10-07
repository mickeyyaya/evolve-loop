---
score_cap:
  - criterion: "atomicwrite.Durable(path, data, mode) writes the bytes with the requested mode, handles a zero-length payload and an existing (even read-only) target, and leaves no temp file on a create or rename failure"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^TestDurable_' ./internal/atomicwrite"
  - criterion: "Every fault branch of atomicwrite, the durable writer's file sync and directory sync included, runs in a test (100% statement coverage)"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -cover ./internal/atomicwrite | grep -q 'coverage: 100.0% of statements'"
  - criterion: "storage.writeJSONAtomic, seal.writeSegment and ledgerartifacts.writeAtomically reach atomicwrite.Durable, perform no temp-create/sync/rename of their own, and keep their output, mode (0600, 0600, 0444) and file set"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1827_(008|009|010)_' ./acs/cycle1827"
  - criterion: "The three writers' packages stay green"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 ./internal/adapters/storage ./internal/ledgerartifacts ./internal/adapters/ledger"
  - criterion: "seal.writeSegment syncs its directory after the rename: it goes through atomicwrite.Durable, which fails when the directory cannot be opened to fsync"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1827_012_' ./acs/cycle1827"
---

# Eval: durable-atomic-write-single-implementation

> Pins one durable write-then-rename implementation in `go/internal/atomicwrite` (`Durable`: a mode
> argument, file fsync before close, directory fsync after rename) and the move of the three writers
> that carried their own copy onto it — `storage.writeJSONAtomic` (state.json), ledger
> `seal.writeSegment` (which never synced its directory) and `ledgerartifacts.writeAtomically` — with
> their output and modes unchanged. Source incident: the carry F5 fix round's engineering-craft audit
> (search-before-write found the third durable writer); materialized in cycle 1825, whose audited
> change failed only at ship (tree drift from another lane); cycle 1827 re-binds the contract.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| durable-contract | Durable writes bytes+mode, empty payload, existing target, no temp on failure | 3/10 | `go test -run '^TestDurable_' ./internal/atomicwrite` |
| fault-coverage | every atomicwrite fault branch runs | 4/10 | `go test -cover ./internal/atomicwrite` = 100.0% |
| three-writers-delegate | the three writers reach Durable, unchanged output/mode | 3/10 | `go test -tags acs -run 'TestC1827_(008\|009\|010)_' ./acs/cycle1827` |
| existing-tests-green | storage / ledgerartifacts / ledger suites pass | 4/10 | `go test ./internal/adapters/storage ./internal/ledgerartifacts ./internal/adapters/ledger` |
| segment-dir-sync | writeSegment's write syncs its directory | 4/10 | `go test -tags acs -run TestC1827_012_ ./acs/cycle1827` |

## Graders
- [code] `cd go && go test -count=1 ./internal/atomicwrite/... ./internal/ledgerartifacts/... ./internal/adapters/ledger/... ./internal/adapters/storage/...` exits 0, with fault-branch tests for the durable variant.
- [code] no private temp+fsync+rename copy remains in the three named writers: `TestC1827_008_`/`009_`/`010_` walk `writeJSONAtomic`, `writeSegment` and `writeAtomically` with their same-package helpers. The plain `grep -c "Sync()"` over the three files also counts `seal.go`'s `rewriteLive`, a live-file rewriter that the acceptance does not name, so it is not the binding form.
- [code] negative: fault-seam tests for fsync/rename/dir-sync failure return error and leave no temp file
- [code] edge: zero-length payload and existing target are written durably with requested mode
- [code] `cd go && go run ./cmd/commentaudit comments -base main internal/atomicwrite` exits 0
