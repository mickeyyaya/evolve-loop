---
score_cap:
  - criterion: "IsLegacyTier accepts the three legacy Anthropic tier names (haiku, sonnet, opus)"
    max_if_missing: 7
    evidence: "cd go && go test -v -run 'TestIsLegacyTier/positive' ./internal/bridge/ 2>&1 | grep -c 'PASS: TestIsLegacyTier/positive' | awk '{exit ($1 < 3)}'"
  - criterion: "IsLegacyTier rejects non-legacy inputs (empty, canonical names, whitespace, case variants)"
    max_if_missing: 8
    evidence: "cd go && go test -v -run 'TestIsLegacyTier/negative' ./internal/bridge/ 2>&1 | grep -c 'PASS: TestIsLegacyTier/negative' | awk '{exit ($1 < 4)}'"
  - criterion: "No regression in ./internal/bridge/ test suite"
    max_if_missing: 6
    evidence: "cd go && go test ./internal/bridge/"
  - criterion: "IsLegacyTier is an exported package-level function in manifest.go"
    max_if_missing: 7
    evidence: "grep -qE '^func IsLegacyTier\\(' go/internal/bridge/manifest.go"
  - criterion: "Eval file exists at .evolve/evals/add-is-legacy-tier.md"
    max_if_missing: 5
    evidence: "test -f .evolve/evals/add-is-legacy-tier.md"
---

# Eval: Add IsLegacyTier helper to manifest.go

> Pins the public-API contract of `IsLegacyTier(s string) bool` introduced
> in cycle 133. The helper identifies the three Anthropic-era legacy tier names
> (`haiku`, `sonnet`, `opus`) that `translateV1TierKey` maps to the canonical
> `fast/balanced/deep` vocabulary. It is the semantic inverse of `IsCanonicalTier`
> (cycle 132), completing the two-predicate API for tier-name classification.
>
> Source incident: cycle 131 audit C1 — TDD-engineer prompt previously did not
> mandate `.evolve/evals/` files; cycles 132 and 133 both include mandatory eval
> artifacts (Step 6b). The cycle-131 indent-anchor footgun (sub-test PASS lines
> are indented 4 spaces; `^--- PASS:` misses them) is codified in the evidence
> commands: all `go test -v` PASS counts use `grep -c 'PASS: TestIsLegacyTier/'`
> (name-scoped, indent-immune).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| legacy-positives | haiku/sonnet/opus → true (3 sub-tests) | 7/10 | `go test -run positive` + `grep -c 'PASS: TestIsLegacyTier/positive'` |
| non-legacy-negatives | empty/balanced/trailing-space/SONNET → false (4 sub-tests) | 8/10 | `go test -run negative` + `grep -c 'PASS: TestIsLegacyTier/negative'` |
| no-regression | full bridge suite exits 0 | 6/10 | `go test ./internal/bridge/` |
| export-location | function in manifest.go, exported | 7/10 | `grep -qE '^func IsLegacyTier\('` |
| eval-file-exists | this file on disk | 5/10 | `test -f .evolve/evals/add-is-legacy-tier.md` |

## Indent-Tolerant Grep Requirement (cycle-131 lesson)

All evidence commands that count sub-test PASS lines MUST use the sub-test
name form `'PASS: TestIsLegacyTier/subgroup'` rather than the line-anchored
`'^--- PASS:'`. Go's `testing` package emits sub-test PASS lines with 4-space
indentation; `^` anchors to the start of the string, missing those lines and
under-counting. The name-scoped pattern `PASS: TestIsLegacyTier/` matches
regardless of indentation.
