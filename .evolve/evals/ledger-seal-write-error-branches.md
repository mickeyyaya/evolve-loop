---
score_cap:
  - criterion: "writeSegment os.CreateTemp error branch is exercised (coverage >= 66%)"
    max_if_missing: 6
    evidence: "cd go && go test -coverprofile=/tmp/ev_ledger_seal_c331.cover ./internal/adapters/ledger/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_ledger_seal_c331.cover | awk '$2==\"writeSegment\"{gsub(/%/,\"\",$3); found=1; ok=($3+0>=66)} END{exit !(found && ok)}'"
  - criterion: "the three seal write-error tests exist and PASS (CreateTempError carries the new branch)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -v -run 'TestWriteSegment_MkdirError|TestWriteSegment_CreateTempError|TestRewriteLive_CreateTempError' ./internal/adapters/ledger/... 2>&1 | grep -q '^--- PASS: TestWriteSegment_CreateTempError'"
  - criterion: "the ledger suite is green (no test regression)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/adapters/ledger/... 2>&1 | grep -q '^ok'"
---

# Eval: ledger-seal-write-error-branches

> Pins the test depth of the OS write-error branches in `seal.go`, the ledger
> sealing subsystem of `internal/adapters/ledger`. At cycle-331's baseline
> `writeSegment` was 62.5% covered: the happy path, the `os.MkdirAll` error, and
> the final `os.Rename` error were already hit, but the `os.CreateTemp` failure
> arm (`return fmt.Errorf("ledger seal: tmp: %w", ...)`) was dark. That arm
> carries the atomic-write invariant — if the segment temp file cannot be created
> in the ledger-segments directory, sealing MUST fail loudly rather than silently
> drop a segment. cycle-331 covers it by chmod'ing the parent directory read-only
> so `os.CreateTemp` returns EACCES. `rewriteLive`'s analogous CreateTemp arm was
> already covered, but the named `TestRewriteLive_CreateTempError` pins that
> behavior against future regressions of the live-rewrite path.
>
> SCOPE NOTE: the gzip-writer, `tmp.Sync`, and `tmp.Close` error arms of
> `writeSegment`/`rewriteLive` are UNREACHABLE by filesystem tricks (a freshly
> opened regular file does not fail write/fsync/close portably — the same finding
> the cycle-322 modelcatalog eval documents), so ~66.7% is writeSegment's
> practical ceiling. The cap is 66%, not the scout's optimistic 75%. The
> package-total floor is intentionally NOT re-pinned here — the tracked
> `ledger-seal-io-coverage.md` eval already owns the package coverage cap (>=85%).
> Source incident: cycle-331 ledger write-error coverage campaign.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| createtemp-branch | writeSegment CreateTemp error arm covered (>=66%) | 6/10 | `go tool cover -func \| awk writeSegment>=66` |
| named-tests-pass | the three seal write-error tests exist & PASS | 7/10 | `go test -v -run 'TestWriteSegment_*\|TestRewriteLive_CreateTempError' \| grep PASS` |
| no-regression | the ledger suite stays green | 7/10 | `go test ./internal/adapters/ledger/...` |
