---
score_cap:
  - criterion: "ratchet check exits 1 on an over-limit function or a raw git init outside the baseline"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1787_00[789]' ./acs/cycle1787"
  - criterion: "ratchet check exits 0 on the real module and offender lists stay untouched"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1787_01[12]' ./acs/cycle1787"
  - criterion: "ratchet check size|rawgit honors --root, runs only the named ratchet, and exits 2 on an unknown selector or trailing argument"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1787_01[567]' ./acs/cycle1787"
  - criterion: "the packages' repo-wide gate tests run through the same Scan the command runs"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1787_01[89]' ./acs/cycle1787"
---

# Eval: ratchet check verb

> `evolve ratchet check [size|rawgit] [--root DIR]` runs sizeratchet and rawgitratchet through one shared scan function (cycle 1787). The lane must not edit offenders.json or baseline.json. Audit round 1 of cycle 1787 found the gate fail-open: `check size --root <violating fixture>` stopped flag parsing at `size`, dropped `--root`, and printed clean with exit 0. It also found that the package gate tests composed their own scan instead of calling Scan.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fixture-over-allowance | exits 1 | 7/10 | `TestC1787_007`, `TestC1787_008` |
| real-module-clean | exits 0 | 6/10 | `TestC1787_011` |
| selector-fail-closed | `size`/`rawgit` honor --root, junk exits 2 | 8/10 | `TestC1787_015`..`017` |
| one-scan-function | gate tests reach Scan (coverage) | 6/10 | `TestC1787_018`, `TestC1787_019` |
