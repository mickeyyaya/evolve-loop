---
score_cap:
  - criterion: "internal/verdictcache statement coverage stays at or above the 93% floor"
    max_if_missing: 7
    evidence: "cd go && go test -cover ./internal/verdictcache/... 2>&1 | awk -F'coverage: ' '/coverage:/{split($2,a,\"%\"); exit !(a[1]>=93)}'"
  - criterion: "(*Store).write error exits (mkdir, write-temp) remain exercised (>= 80%)"
    max_if_missing: 6
    evidence: "cd go && go test -coverprofile=/tmp/vc-eval.cover ./internal/verdictcache/... >/dev/null 2>&1 && go tool cover -func=/tmp/vc-eval.cover | awk '/\\.go:[0-9]+:[[:space:]]+write[[:space:]]/{split($NF,a,\"%\"); exit !(a[1]>=80)}'"
  - criterion: "the full verdictcache suite passes under -race (no regression)"
    max_if_missing: 6
    evidence: "cd go && go test -race -count=1 ./internal/verdictcache/... >/dev/null 2>&1"
---

# Eval: verdictcache write/Load error-branch coverage

> Pins the error-branch coverage of `internal/verdictcache` introduced in
> cycle 329 (harden). The package is the ADR-0048 Slice B content-addressed
> audit-reuse store; its degrade-to-empty and atomic-write error exits are the
> correctness boundary (a corrupt/unwritable cache must never break a cycle).
> Cycle 329 lifted coverage from a 79.5% baseline (write 63.6%, Load 76.9%,
> NewStore 66.7%) to >= 93% by exercising (*Store).write's mkdir / write-temp
> error exits, (*Store).Load's non-IsNotExist read-error arm, and NewStore's
> nil-now default. This eval is the PERMANENT regression entry that caps the
> audit score if that coverage later rots.
> Source incident: cycle 329 scout coverage survey (no prior research entry).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| package-floor | verdictcache total coverage >= 93% | 7/10 | `go test -cover ./internal/verdictcache/...` ≥ 93% |
| write-error-exits | (*Store).write coverage >= 80% (mkdir + write-temp arms) | 6/10 | `go tool cover -func` write row ≥ 80% |
| no-regression | full suite green under -race | 6/10 | `go test -race ./internal/verdictcache/...` |
