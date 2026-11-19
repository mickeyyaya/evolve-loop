---
score_cap:
  - criterion: "issue check passes on the current tree"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2530_001_' ./acs/cycle2530"
  - criterion: "issue check fails on a new docs/incidents file with no frontmatter, after a passing control"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC2530_002_|TestC2530_009_)' ./acs/cycle2530"
  - criterion: "a hand edit of the rendered Actions, status or Effectiveness part fails with the field name, and a prose edit still passes"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC2530_003_|TestC2530_004_|TestC2530_005_|TestC2530_006_)' ./acs/cycle2530"
  - criterion: "a missing section heading and a bad frontmatter field fail with their names"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC2530_007_|TestC2530_008_)' ./acs/cycle2530"
  - criterion: "issue list reports records and legacy files as JSON and filters by status and code"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC2530_010_|TestC2530_011_|TestC2530_012_|TestC2530_017_)' ./acs/cycle2530"
  - criterion: "incidentkb.Render is deterministic and idempotent, and refuses a document it cannot render"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC2530_013_|TestC2530_014_)' ./acs/cycle2530"
  - criterion: "incidentkb is a leaf and graduates into the repo-wide apicover gate"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC2530_015_|TestC2530_016_)' ./acs/cycle2530"
---

# Eval: incidentkb record schema, renderer, legacy rule, issue check and list (I2)

> Pins the first slice of the incident knowledge base (plan
> `docs/plans/incident-knowledge-base-2026-11.md`, component I2, commit
> 66daeccd3). A record is a Markdown file with `incident-record/1`
> frontmatter. The status line, the Actions section and the Effectiveness
> section are rendered between GENERATED markers. `evolve issue check` parses
> each record, re-renders it and refuses a difference by field name. A file with
> no frontmatter and a date on or after 2026-11-19 fails. Older files are
> legacy. Cycle 2530 wrote the predicates.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| current-tree | check passes on the current tree and reports its legacy files | 4/10 | `TestC2530_001_` |
| negative-new-file | a new file with no frontmatter fails | 5/10 | `TestC2530_002_`, `TestC2530_009_` |
| negative-hand-edit | a hand edit of a rendered part names the field | 4/10 | `TestC2530_003_` to `TestC2530_006_` |
| negative-schema | a missing heading or a bad field names itself | 6/10 | `TestC2530_007_`, `TestC2530_008_` |
| list | JSON rows, legacy flag and filters | 6/10 | `TestC2530_010_` to `TestC2530_012_`, `TestC2530_017_` |
| render | the seam the fixtures and the check share | 6/10 | `TestC2530_013_`, `TestC2530_014_` |
| new-package | leaf and apicover graduation | 7/10 | `TestC2530_015_`, `TestC2530_016_` |
