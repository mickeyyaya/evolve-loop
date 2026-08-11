---
score_cap:
  - criterion: "NormalizeTierAlias is an exported package-level function in manifest.go"
    max_if_missing: 7
    evidence: "grep -qE '^func NormalizeTierAlias\\(' go/internal/bridge/manifest.go"
  - criterion: "NormalizeTierAlias maps canonical tiers (fast/balanced/deep) to themselves"
    max_if_missing: 7
    evidence: "cd go && go test -v -run 'TestNormalizeTierAlias/canonical' ./internal/bridge/ 2>&1 | grep -c 'PASS: TestNormalizeTierAlias/canonical' | awk '{exit ($1 < 3)}'"
  - criterion: "NormalizeTierAlias maps legacy aliases (haiku→fast, sonnet→balanced, opus→deep)"
    max_if_missing: 8
    evidence: "cd go && go test -v -run 'TestNormalizeTierAlias/legacy' ./internal/bridge/ 2>&1 | grep -c 'PASS: TestNormalizeTierAlias/legacy' | awk '{exit ($1 < 3)}'"
  - criterion: "NormalizeTierAlias passes through unrecognized strings verbatim"
    max_if_missing: 6
    evidence: "cd go && go test -v -run 'TestNormalizeTierAlias/passthrough' ./internal/bridge/ 2>&1 | grep -c 'PASS: TestNormalizeTierAlias/passthrough' | awk '{exit ($1 < 2)}'"
  - criterion: "No regression in non-tmux bridge tests"
    max_if_missing: 6
    evidence: "cd go && go test -run 'Test[^R]' ./internal/bridge/"
  - criterion: "Eval file exists at .evolve/evals/add-normalize-tier-alias.md"
    max_if_missing: 5
    evidence: "test -f .evolve/evals/add-normalize-tier-alias.md"
---

# Eval: Add NormalizeTierAlias to manifest.go

> Pins the public-API contract of `NormalizeTierAlias(s string) string` introduced
> in cycle 135. The function is an exported wrapper around the unexported
> `translateV1TierKey` — making the canonical tier-normalization logic available
> to callers outside the bridge package without duplicating the three-entry
> haiku/sonnet/opus → fast/balanced/deep mapping.
>
> Source incident: cycle 134 was reset due to challenge-token protocol failures
> (C1: scout used placeholder token; C2: Builder omitted challenge-token header in
> build-report.md) and a P005 predicate scope bug (H1: `go test ./internal/bridge/`
> pulled in TestRealTmux_* pre-existing failures). Cycle 135 re-executes with
> corrected protocol handling and the P005 scope fix (`-run 'Test[^R]'`).
>
> The cycle-131 indent-anchor footgun (sub-test PASS lines are indented 4 spaces;
> `^--- PASS:` misses them) is codified in all evidence commands: PASS counts use
> `grep -c 'PASS: TestNormalizeTierAlias/<subgroup>'` (name-scoped, indent-immune).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| export-location | function in manifest.go, exported | 7/10 | `grep -qE '^func NormalizeTierAlias\('` |
| canonical-passthrough | fast/balanced/deep → unchanged (3 sub-tests) | 7/10 | `go test -run canonical` + `grep -c 'PASS: TestNormalizeTierAlias/canonical'` |
| legacy-translation | haiku/sonnet/opus → canonical (3 sub-tests) | 8/10 | `go test -run legacy` + `grep -c 'PASS: TestNormalizeTierAlias/legacy'` |
| verbatim-passthrough | custom strings unchanged (2 sub-tests) | 6/10 | `go test -run passthrough` + `grep -c 'PASS: TestNormalizeTierAlias/passthrough'` |
| no-regression | non-tmux bridge suite exits 0 | 6/10 | `go test -run 'Test[^R]' ./internal/bridge/` |
| eval-file-exists | this file on disk | 5/10 | `test -f .evolve/evals/add-normalize-tier-alias.md` |

## Indent-Tolerant Grep Requirement (cycle-131 lesson)

All evidence commands that count sub-test PASS lines use the sub-test name form
`'PASS: TestNormalizeTierAlias/<subgroup>'` rather than the line-anchored
`'^--- PASS:'`. Go's `testing` package emits sub-test PASS lines with 4-space
indentation; `^` anchors miss those lines and under-count. The name-scoped pattern
matches regardless of indentation depth.
