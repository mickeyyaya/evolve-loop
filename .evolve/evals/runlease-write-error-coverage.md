---
score_cap:
  - criterion: "internal/runlease statement coverage is >= 95%"
    max_if_missing: 7
    evidence: "cd go && go test -coverprofile=/tmp/ev_runlease_c299.cover ./internal/runlease/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_runlease_c299.cover | awk '/^total:/{gsub(/%/,\"\",$NF); found=1; ok=($NF+0>=95)} END{exit !(found && ok)}'"
  - criterion: "runlease.Write coverage is >= 80% — the atomic tmp+rename error returns (CreateTemp/Write/Close/Rename) are exercised, not just the happy-path round-trip"
    max_if_missing: 8
    evidence: "cd go && go test -coverprofile=/tmp/ev_runlease_c299.cover ./internal/runlease/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_runlease_c299.cover | awk '$2==\"Write\"{gsub(/%/,\"\",$3); found=1; ok=($3+0>=80)} END{exit !(found && ok)}'"
  - criterion: "the runlease suite is green (no test regression)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 ./internal/runlease/... 2>&1 | grep -q '^ok'"
---

# Eval: runlease-write-error-coverage

> Pins the error-path coverage of `runlease.Write`, the single writer of the
> per-run `.lease` heartbeat file (L3.2, concurrency campaign) that the GC
> retention engine reads to classify a run dir as LIVE. `Write` performs an
> atomic tmp+rename with four distinct error returns (CreateTemp / Write /
> Close / Rename); all four were dark (52.6%) because the only test was a
> happy-path round-trip, so a latent bug in the atomic-write path that makes a
> reader see a torn lease — or makes a live run look collectable — would ship
> unseen. The `Write >= 80%` cap is load-bearing: the happy path alone caps
> near 53%, so reaching 80% requires actually driving the error returns (a
> non-writable runDir, a rename collision). Source incident: cycle 299
> (scout-report.md Task 2 — `Write` at 52.6%, "4 error return paths all
> uncovered").

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| coverage-floor | runlease package statement coverage >= 95% | 7/10 | `go tool cover -func` total >= 95% |
| write-error-paths | `Write` >= 80% (tmp/write/close/rename error returns) | 8/10 | `go tool cover -func` `Write` row >= 80% |
| no-regression | runlease suite stays green | 8/10 | `go test ./internal/runlease/...` → `ok` |
