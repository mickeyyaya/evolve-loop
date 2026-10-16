---
score_cap:
  - criterion: "evolve bridge promote-rule writes a corpus-clean rule at stage shadow and names its id and path (exit 0)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1880_00[124]_' ./acs/cycle1880"
  - criterion: "a regex that matches a healthy-pane corpus line is refused with exit 1 naming the line, and nothing is written"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1880_003_' ./acs/cycle1880"
  - criterion: "missing flags, unknown flags and a pattern under the minimum length exit 10 and write nothing"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1880_005_' ./acs/cycle1880"
  - criterion: "bridge help lists promote-rule and runtime-reference.md documents add-rule and promote-rule"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1880_006_' ./acs/cycle1880"
  - criterion: "the changed Go files add no comments"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1880_007_' ./acs/cycle1880"
---

# Eval: add `evolve bridge promote-rule`

> Pins the CLI entry point to `interaction.PromoteRule` (`go/internal/interaction/rulepromote.go:77`).
> Before cycle 1880 only tests called it, so a rule learned from a stall was added unchecked by
> `bridge add-rule` or not at all. The predicates drive the built `evolve` binary, so they prove
> the production caller reaches the seam. Source: inbox item cli-bridge-promote-rule (2026-09-30), cycle 1880.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| promote-positive | corpus-clean rule written at stage shadow, id and path printed | 8/10 | `TestC1880_001/002/004` |
| corpus-negative | corpus match refused, exit 1, line named, nothing written | 8/10 | `TestC1880_003` |
| usage-negative | bad invocations exit 10, nothing written | 6/10 | `TestC1880_005` |
| docs | help and runtime-reference list the verb | 4/10 | `TestC1880_006` |
| comment-zero | no added comments | 3/10 | `TestC1880_007` |
