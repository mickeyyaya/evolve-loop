---
score_cap:
  - criterion: "internal/adapters/flock statement coverage is >= 90%"
    max_if_missing: 7
    evidence: "cd go && go test -coverprofile=/tmp/ev_flock_c299.cover ./internal/adapters/flock/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_flock_c299.cover | awk '/^total:/{gsub(/%/,\"\",$NF); found=1; ok=($NF+0>=90)} END{exit !(found && ok)}'"
  - criterion: "flock.Lock is 100% covered — every error return (MkdirAll, OpenFile, the flockFn LOCK_EX seam) and the release closure are exercised"
    max_if_missing: 8
    evidence: "cd go && go test -coverprofile=/tmp/ev_flock_c299.cover ./internal/adapters/flock/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_flock_c299.cover | awk '$2==\"Lock\"{gsub(/%/,\"\",$3); found=1; ok=($3+0>=100)} END{exit !(found && ok)}'"
  - criterion: "the flock suite is green (no test regression)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 ./internal/adapters/flock/... 2>&1 | grep -qE '^ok|no test files' && go test -count=1 ./internal/adapters/flock/... 2>&1 | grep -q '^ok'"
---

# Eval: flock-tests

> Pins the first test coverage of `internal/adapters/flock`, the BLOCKING
> cross-process file lock that serializes the concurrency campaign's two
> read-modify-write critical sections — `ledger.Append` (CA.1) and
> `storage.UpdateState` (CA.3). The package shipped at 0% coverage with a
> deliberate `var flockFn = syscall.Flock` test seam (flock.go:22) and never
> got its test file; this eval guarantees the seam stays exercised so the
> lock's error paths (MkdirAll / OpenFile / flockFn LOCK_EX) cannot silently
> rot. The `Lock = 100%` cap is the load-bearing one: because `Lock` is the
> package's only function, only injecting a flockFn error AND driving the
> MkdirAll/OpenFile error returns reaches 100% — a happy-path-only test caps
> near 77%. Source incident: cycle 299 (scout-report.md Task 1 — flock at 0%,
> error paths "completely dark").

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| coverage-floor | flock package statement coverage >= 90% | 7/10 | `go tool cover -func` total >= 90% |
| error-branch-completeness | `Lock` == 100% (flockFn seam + both error returns + release) | 8/10 | `go tool cover -func` `Lock` row == 100% |
| no-regression | flock suite stays green | 8/10 | `go test ./internal/adapters/flock/...` → `ok` |
