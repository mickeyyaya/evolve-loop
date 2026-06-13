---
score_cap:
  - criterion: "internal/clihealth statement coverage is >= 90%"
    max_if_missing: 6
    evidence: "cd go && go test -coverprofile=/tmp/ev_clihealth_c316.cover ./internal/clihealth/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_clihealth_c316.cover | awk '/^total:/{gsub(/%/,\"\",$NF); found=1; ok=($NF+0>=90)} END{exit !(found && ok)}'"
  - criterion: "clihealth.firstLine coverage is 100% — both the newline-found path and the no-newline/empty-string edge are exercised"
    max_if_missing: 7
    evidence: "cd go && go test -coverprofile=/tmp/ev_clihealth_c316.cover ./internal/clihealth/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_clihealth_c316.cover | awk '$2==\"firstLine\"{gsub(/%/,\"\",$3); found=1; ok=($3+0>=100)} END{exit !(found && ok)}'"
  - criterion: "clihealth.truncateRunes coverage is 100% — both the below-limit path and the multi-byte rune-boundary truncation are exercised"
    max_if_missing: 7
    evidence: "cd go && go test -coverprofile=/tmp/ev_clihealth_c316.cover ./internal/clihealth/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_clihealth_c316.cover | awk '$2==\"truncateRunes\"{gsub(/%/,\"\",$3); found=1; ok=($3+0>=100)} END{exit !(found && ok)}'"
  - criterion: "the clihealth suite is green (no test regression)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/clihealth/... 2>&1 | grep -q '^ok'"
---

# Eval: clihealth-zero-coverage

> Pins the test depth of `internal/clihealth`, the durable bench store for
> transient CLI-family outages (cycle-283 forensics: a codex quota wall was
> re-classified on every dispatch but never remembered, so every codex-routed
> phase re-burned a 5-15min boot all night). Four functions — `Benchable` (the
> closed-set "is this pattern worth benching the whole family" gate), the
> `NewBenchEntry` composition hub, and its `firstLine`/`truncateRunes` evidence
> helpers — were at 0% coverage, so a regression in the bench-decision or
> evidence-truncation logic could ship unseen. The package `>= 90%` cap forces
> all four to be exercised; the `firstLine`/`truncateRunes` 100% caps force their
> distinctive edges — the empty-string / no-newline path and the multi-byte
> rune-boundary truncation (wall text opens with the '■' glyph, which a byte
> slice would corrupt). Source incident: cycle 316 (scout-report.md Task 1 —
> `Benchable`, `NewBenchEntry`, `firstLine`, `truncateRunes` all at 0.0%).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| coverage-floor | clihealth package statement coverage >= 90% | 6/10 | `go tool cover -func` total >= 90% |
| empty-string-edge | `firstLine` 100% (newline-found + empty/no-newline) | 7/10 | `go tool cover -func` `firstLine` row == 100% |
| rune-boundary-edge | `truncateRunes` 100% (below-limit + multi-byte truncation) | 7/10 | `go tool cover -func` `truncateRunes` row == 100% |
| no-regression | clihealth suite stays green | 7/10 | `go test ./internal/clihealth/...` → `ok` |
---
