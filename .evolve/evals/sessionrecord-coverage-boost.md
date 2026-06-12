---
score_cap:
  - criterion: "internal/sessionrecord statement coverage is >= 90%"
    max_if_missing: 6
    evidence: "cd go && go test -coverprofile=/tmp/ev_sessionrecord_c299.cover ./internal/sessionrecord/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_sessionrecord_c299.cover | awk '/^total:/{gsub(/%/,\"\",$NF); found=1; ok=($NF+0>=90)} END{exit !(found && ok)}'"
  - criterion: "sessionrecord.RunScopeToken coverage is >= 90% — both the >8-char ULID truncation branch and the short-input path are exercised"
    max_if_missing: 7
    evidence: "cd go && go test -coverprofile=/tmp/ev_sessionrecord_c299.cover ./internal/sessionrecord/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_sessionrecord_c299.cover | awk '$2==\"RunScopeToken\"{gsub(/%/,\"\",$3); found=1; ok=($3+0>=90)} END{exit !(found && ok)}'"
  - criterion: "the sessionrecord suite is green (no test regression)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/sessionrecord/... 2>&1 | grep -q '^ok'"
---

# Eval: sessionrecord-coverage-boost

> Pins the test depth of `internal/sessionrecord`, the per-run tmux session
> registry (CB.5, concurrency campaign) that makes registry-based reaping
> structurally run-isolated. `RunScopeToken` — the session-name run namespace
> ("r" + first 8 ULID chars) shared by the bridge's `resolveSession` and the
> observer's CB.6 run-scope assertion — was at 0% (never tested), so a
> regression in the namespace rule (which gates whether a probe refuses an
> out-of-run session) could ship unseen. The `RunScopeToken >= 90%` cap forces
> the `len(runID) > 8` truncation branch to be exercised, not just the short
> path. `Append`'s open/write/close error returns were also dark (58.3%), so
> the package floor lifts them too. Source incident: cycle 299 (scout-report.md
> Task 3 — `RunScopeToken` at 0%, `Append` at 58.3%).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| coverage-floor | sessionrecord package statement coverage >= 90% | 6/10 | `go tool cover -func` total >= 90% |
| token-truncation-branch | `RunScopeToken` >= 90% (>8-char + short input) | 7/10 | `go tool cover -func` `RunScopeToken` row >= 90% |
| no-regression | sessionrecord suite stays green | 7/10 | `go test ./internal/sessionrecord/...` → `ok` |
---
