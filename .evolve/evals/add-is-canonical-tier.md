---
score_cap:
  - criterion: "IsCanonicalTier accepts the three canonical tiers (fast, balanced, deep)"
    max_if_missing: 7
    evidence: "cd go && go test -v -run 'TestIsCanonicalTier/canonical' ./internal/bridge/ 2>&1 | grep -c 'PASS: TestIsCanonicalTier/canonical' | awk '{exit ($1 < 3)}'"
  - criterion: "IsCanonicalTier rejects non-canonical inputs (empty, legacy names, whitespace, case variants)"
    max_if_missing: 8
    evidence: "cd go && go test -v -run 'TestIsCanonicalTier/negative' ./internal/bridge/ 2>&1 | grep -c 'PASS: TestIsCanonicalTier/negative' | awk '{exit ($1 < 5)}'"
  - criterion: "No regression in ./internal/bridge/ test suite"
    max_if_missing: 6
    evidence: "cd go && go test ./internal/bridge/"
  - criterion: "IsCanonicalTier is an exported package-level function in manifest.go"
    max_if_missing: 7
    evidence: "grep -qE '^func IsCanonicalTier\\(' go/internal/bridge/manifest.go"
  - criterion: "Eval file exists at .evolve/evals/add-is-canonical-tier.md"
    max_if_missing: 5
    evidence: "test -f .evolve/evals/add-is-canonical-tier.md"
---

# Eval: Add IsCanonicalTier helper to manifest.go

> Pins the public-API contract of `IsCanonicalTier(s string) bool` introduced
> in cycle 132. The helper distinguishes valid canonical tier names from the
> legacy Anthropic-tier vocabulary and general invalid strings. It is a pure
> predicate that uses the same three canonical strings (`fast`, `balanced`,
> `deep`) established by `translateV1TierKey` in ADR-0022.
>
> Source incident: cycle 131 audit C1 — TDD-engineer prompt previously did
> not mandate `.evolve/evals/` files; cycle 132 closes that gap with this
> mandatory eval artifact (Step 6b). The cycle-131 indent-anchor footgun
> (sub-test PASS lines are indented 4 spaces; `^--- PASS:` misses them) is
> codified in the evidence commands: all `go test -v` PASS counts use
> `grep -c 'PASS: TestIsCanonicalTier/'` (name-scoped, indent-immune).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| canonical-positives | fast/balanced/deep → true (3 sub-tests) | 7/10 | `go test -run canonical` + `grep -c 'PASS: TestIsCanonicalTier/canonical'` |
| negative-rejections | empty/sonnet/opus/trailing-space/FAST → false (5 sub-tests) | 8/10 | `go test -run negative` + `grep -c 'PASS: TestIsCanonicalTier/negative'` |
| no-regression | full bridge suite exits 0 | 6/10 | `go test ./internal/bridge/` |
| export-location | function in manifest.go, exported | 7/10 | `grep -qE '^func IsCanonicalTier\('` |
| eval-file-exists | this file on disk | 5/10 | `test -f .evolve/evals/add-is-canonical-tier.md` |

## Indent-Tolerant Grep Requirement (cycle-131 lesson)

All evidence commands that count sub-test PASS lines MUST use the sub-test
name form `'PASS: TestIsCanonicalTier/subgroup'` rather than the line-anchored
`'^--- PASS:'`. Go's `testing` package emits sub-test PASS lines with 4-space
indentation; `^` anchors to the start of the string, missing those lines and
under-counting. The name-scoped pattern `PASS: TestIsCanonicalTier/` matches
regardless of indentation.
