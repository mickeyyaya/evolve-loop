---
score_cap:
  - criterion: "internal/modelcatalog statement coverage is >= 92%"
    max_if_missing: 6
    evidence: "cd go && go test -coverprofile=/tmp/ev_modelcatalog_c322.cover ./internal/modelcatalog/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_modelcatalog_c322.cover | awk '/^total:/{gsub(/%/,\"\",$NF); found=1; ok=($NF+0>=92)} END{exit !(found && ok)}'"
  - criterion: "store.Write coverage is >= 85% — the tmp.Write/tmp.Sync/tmp.Close error-and-cleanup exits are exercised"
    max_if_missing: 7
    evidence: "cd go && go test -coverprofile=/tmp/ev_modelcatalog_c322.cover ./internal/modelcatalog/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_modelcatalog_c322.cover | awk '$2==\"Write\"{gsub(/%/,\"\",$3); found=1; ok=($3+0>=85)} END{exit !(found && ok)}'"
  - criterion: "the modelcatalog suite is green (no test regression)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/modelcatalog/... 2>&1 | grep -q '^ok'"
---

# Eval: modelcatalog-write-error-paths

> Pins the test depth of `store.Write` in `internal/modelcatalog`, the atomic
> (temp-file + rename) persister for the model-routing cache. At cycle-322's
> baseline `Write` was 66.7% covered: the happy round-trip plus the mkdir /
> CreateTemp / Rename error exits (the latter three via filesystem tricks in
> store_errors_test.go) were hit, but the three exits that operate on the temp
> file *after* it is opened — `tmp.Write`, `tmp.Sync`, `tmp.Close` — were dark.
> Those branches carry the atomic-write invariant: on any temp-file failure the
> partially-written temp MUST be closed and removed so a crash never leaves a
> torn cache. They are unreachable by filesystem tricks (a freshly-created
> regular file does not fail write/fsync/close portably), so a small `createTemp`
> seam over `os.CreateTemp` was added in cycle 322 to inject failures. The
> package `>= 92%` cap and the `Write >= 85%` cap force those error-and-cleanup
> branches to stay exercised; a regression that drops the cleanup or stops
> wrapping the error would reopen the gap. The `json.MarshalIndent` error arm is
> unreachable for a plain `Catalog` struct, which is why the Write ceiling is
> ~96%, not 100%. Source incident: cycle 322 (scout-report.md Task 1 —
> `store.Write` at 66.7%, package at 87.3%).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| coverage-floor | modelcatalog package statement coverage >= 92% | 6/10 | `go tool cover -func` total >= 92% |
| write-error-paths | `store.Write` >= 85% (tmp.Write/Sync/Close cleanup exits) | 7/10 | `go tool cover -func` `Write` row >= 85% |
| no-regression | modelcatalog suite stays green | 7/10 | `go test ./internal/modelcatalog/...` → `ok` |
---
