---
score_cap:
  - criterion: "the core testaudit baseline holds no single-file-source-pin, single-file-ast-pin or literal-field-count row"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2552_001_BaselineHoldsNoStructurePinRows|TestC2552_002_AnalyzerFindsNoStructurePinInCoreTests' ./acs/cycle2552"
  - criterion: "each replacement of a source pin survives a behavior-preserving rename of the code it used to read"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2552_005|TestC2552_007|TestC2552_010|TestC2552_016' ./acs/cycle2552"
  - criterion: "each claim of a removed pin fails a test when a behavior mutant replaces the code it described"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2552_004|TestC2552_006|TestC2552_008|TestC2552_009|TestC2552_01[1-5]' ./acs/cycle2552"
  - criterion: "the retry ladder guard keeps its alias and shape cases"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2552_003 ./acs/cycle2552"
---

# Eval: replace single-file source and AST pins with behavioral tests (T7)

> Pins the T7 contract of cycle 2552: five tests in `internal/core` read production source text to prove a structure claim. A rename or a reformat fails them, and a real regression that keeps the text passes them. Each claim must move to a test that runs the code, or the pin must go with a named covering test. Source incident: the test-quality audit plan, `docs/plans/test-quality-audit-2026-11.md`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| baseline-clean | no pin row in the core baseline | 4/10 | `TestC2552_001`, `TestC2552_002` |
| rename-survival | replacement passes a behavior-preserving rename | 5/10 | `TestC2552_005`, `007`, `010`, `016` |
| mutant-kill | replacement fails a behavior mutant | 5/10 | `TestC2552_004`, `006`, `008`, `009`, `011` to `015` |
| guard-kept | ladder guard keeps its shape cases | 6/10 | `TestC2552_003` |
