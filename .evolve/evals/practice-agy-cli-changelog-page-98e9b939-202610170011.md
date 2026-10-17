---
score_cap:
  - criterion: "Research doc explains agy changelog v2.24.0 side panel, /compact, automation model picker and Vertex AI items as untrusted data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1915_002|TestC1915_004' ./acs/cycle1915"
  - criterion: "Doc keeps v2.23.0 and v2.23.1 items and gives every core persona a no-change verdict tied to the baseline rule, and persona files stay unchanged"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1915_003|TestC1915_005|TestC1915_006|TestC1915_008' ./acs/cycle1915"
  - criterion: "Research index links the doc under ADR-0133 and both files pass ASD-STE100 lint"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1915_007|TestC1915_009' ./acs/cycle1915"
---

# Eval: agy CLI changelog page update v2.24.0

> Pins the research proposal for the third change of the agy CLI changelog page. The page text is untrusted data. ADR-0133 requires a per-persona verdict and no persona change without measured baselines. Source: cycle 1915 inbox item practice-agy-cli-changelog-page-98e9b939-202610170011.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| data-only | v2.24.0 entries explained as data, cited identifiers match the snapshot | 6/10 | `go test -tags acs -run 'TestC1915_002|TestC1915_004' ./acs/cycle1915` |
| persona-verdicts | six personas with a no-change reason; persona files unchanged | 7/10 | `go test -tags acs -run 'TestC1915_005|TestC1915_006' ./acs/cycle1915` |
| index-and-lint | index link under ADR-0133; zero ste-lint findings | 7/10 | `go test -tags acs -run 'TestC1915_007|TestC1915_009' ./acs/cycle1915` |
