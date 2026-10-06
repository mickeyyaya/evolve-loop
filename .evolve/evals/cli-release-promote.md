---
score_cap:
  - criterion: "release-promote PATCHes prerelease=false only after green workflow and all assets"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1807_008 ./acs/cycle1807"
  - criterion: "release-promote never PATCHes on a red run or missing assets"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1807_(009|010|014)' ./acs/cycle1807"
  - criterion: "release-promote maps gh I/O failure and usage errors to exit 2"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1807_(011|012)' ./acs/cycle1807"
  - criterion: "release-promote works under real gh flag grammar: -R only on gh run calls, never on gh api"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1807_(008|010|013|015)' ./acs/cycle1807"
---

# Eval: evolve release-promote

> Pins the `release-promote <tag> [--rerun]` verb: no PATCH issued unless the workflow run is green and every expected asset is present, --rerun happens first, exit 0/1/2 semantics. Source: cycle 1807 inbox item cli-release-promote; audit round 1 H1 found `-R owner/repo` appended to `gh api`, which real gh rejects (`unknown shorthand flag: 'R' in -R`), so the fake gh now models gh's real flag grammar.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| happy-path | exactly one PATCH, last call | 7/10 | `go test -tags acs -run TestC1807_008 ./acs/cycle1807` |
| refusal | no PATCH when red or assets missing | 8/10 | `go test -tags acs -run 'TestC1807_(009|010|014)' ./acs/cycle1807` |
| exit-2 | gh failure / usage | 6/10 | `go test -tags acs -run 'TestC1807_(011|012)' ./acs/cycle1807` |
| real-gh-grammar | no repo flag on gh api, repo bound on gh run | 8/10 | `go test -tags acs -run 'TestC1807_(008|010|013|015)' ./acs/cycle1807` |
