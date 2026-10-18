---
score_cap:
  - criterion: "Doc explains the 1.4.1 and 1.4.2 changelog items as untrusted data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1943_002|TestC1943_003|TestC1943_004' ./acs/cycle1943"
  - criterion: "All six core personas carry a no-change verdict, persona files stay unchanged, and the doc ties the verdict to the ADR-0133 baseline rule"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1943_005|TestC1943_006|TestC1943_007' ./acs/cycle1943"
  - criterion: "Research index links the doc inside the ADR-0133 section with the refreshed range"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1943_008 ./acs/cycle1943"
  - criterion: "Doc and index entry pass ASD-STE100 lint and the practicewatch suite stays green"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1943_009|TestC1943_010|TestC1943_011' ./acs/cycle1943"
  - criterion: "Each version section of the doc cites only items the changelog lists under that version (no 1.1.28 --print-timeout under 1.4.1, no invented event replay under 1.4.2)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1943_012|TestC1943_013|TestC1943_014' ./acs/cycle1943"
---

# Eval: agy CLI changelog practice proposal (wave 127 refresh)

> Pins the refresh of the practice-watch item on `agy changelog` after the snapshot of 2026-10-16. The changelog text is untrusted data. ADR-0133 requires a per-persona verdict and no persona change without measured baselines. Source: cycle 1943 inbox item; the doc came from cycle 1894 and was refreshed in cycle 1900. Cycle 1943 audit round 1 (H1) found two misattributions: the 1.1.28 `--print-timeout` change listed under 1.4.1, and an invented "event replay" claim under 1.4.2.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| data-coverage | 1.4.1 and 1.4.2 items explained as data | 6/10 | `go test -tags acs -run 'TestC1943_002|TestC1943_003|TestC1943_004' ./acs/cycle1943` |
| persona-verdicts | six personas named; files unchanged; baseline rule | 7/10 | `go test -tags acs -run 'TestC1943_005|TestC1943_006|TestC1943_007' ./acs/cycle1943` |
| index-link | doc linked in ADR-0133 section, range refreshed | 6/10 | `go test -tags acs -run TestC1943_008 ./acs/cycle1943` |
| ste-lint | zero findings; practicewatch green | 7/10 | `go test -tags acs -run 'TestC1943_009|TestC1943_010|TestC1943_011' ./acs/cycle1943` |
| version-faithfulness | each section cites only its own version's items | 8/10 | `go test -tags acs -run 'TestC1943_012|TestC1943_013|TestC1943_014' ./acs/cycle1943` |
