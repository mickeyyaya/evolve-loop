---
score_cap:
  - criterion: "a new predicate that only matches source text (FileContains or FileMatchesRegex) is refused by the lint"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2499_001|TestC2499_002|TestC2499_004' ./acs/cycle2499"
  - criterion: "a predicate that runs the code passes the lint"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2499_003|TestC2499_005|TestC2499_007' ./acs/cycle2499"
  - criterion: "the production reviewer refuses a source-text predicate at enforce and only observes it at shadow"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2499_006|TestC2499_008' ./acs/cycle2499"
---

# Eval: evalqualitycheck refuses source-text-only predicates (T5)

> Pins `evalqualitycheck.LintSourceTextPredicates` and the evalgate reviewer gate that calls it,
> from plan `docs/plans/test-quality-audit-2026-11.md`. A cycle predicate whose only load-bearing
> assertion greps source text passes once the magic string exists, whatever the code does
> (cycle-85 failure class). Source: cycle 2499, inbox item test-audit-t5-no-source-text-predicates.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| refuse | FileContains-only and FileMatchesRegex-only predicates are findings | 4/10 | `go test -tags acs -run 'TestC2499_001|TestC2499_002|TestC2499_004' ./acs/cycle2499` |
| accept | subprocess, exec, direct call and mixed predicates are clean | 5/10 | `go test -tags acs -run 'TestC2499_003|TestC2499_005|TestC2499_007' ./acs/cycle2499` |
| wiring | the reviewer gate refuses at enforce, observes at shadow | 5/10 | `go test -tags acs -run 'TestC2499_006|TestC2499_008' ./acs/cycle2499` |
