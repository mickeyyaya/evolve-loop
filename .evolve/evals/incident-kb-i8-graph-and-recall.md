---
score_cap:
  - criterion: "kbgraph lists the cycle, PR and item of an incident record as neighbors of incident:<id>"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2533_001|TestC2533_002|TestC2533_003' ./acs/cycle2533"
  - criterion: "legacy and unparsable incident files are handled without a phantom node or a silent skip"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2533_004 ./acs/cycle2533"
  - criterion: "evolve kb graph neighbors incident:<id> prints the cycle, PR and item edges and fails on an unknown record"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2533_006 ./acs/cycle2533"
  - criterion: "lesson recall finds an incident record by a word of its Lessons section and ignores legacy files"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2533_007|TestC2533_008' ./acs/cycle2533"
  - criterion: "docs/incidents/ is one of the default lesson-recall search paths"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2533_009|TestC2533_010' ./acs/cycle2533"
---

# Eval: kbgraph incident nodes and edges, and docs/incidents as a lesson-recall root (I8)

> Pins component I8 of the incident knowledge base plan (D7, docs/plans/incident-knowledge-base-2026-11.md). The graph reads incident records through incidentkb and adds the node kind incident with edges affected (cycle), recurs-as (fingerprint), fixed-by (PR) and tracked-by (item). Lesson recall adds docs/incidents/ to its roots and finds a record by a word of its Lessons section. Source: cycle 2533.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| graph-edges | incident neighbors hold cycle, PR, item, fingerprint | 4/10 | `go test -tags acs -run TestC2533_001 ./acs/cycle2533` |
| graph-join | incident item joins the inbox item node; shared PR joins two incidents | 4/10 | `go test -tags acs -run 'TestC2533_002\|TestC2533_003' ./acs/cycle2533` |
| graph-negative | legacy file makes no node; broken record fails the build | 6/10 | `go test -tags acs -run TestC2533_004 ./acs/cycle2533` |
| cli | the real kb graph neighbors verb prints the edges | 5/10 | `go test -tags acs -run TestC2533_006 ./acs/cycle2533` |
| recall | a lesson word finds only its record | 4/10 | `go test -tags acs -run 'TestC2533_007\|TestC2533_008' ./acs/cycle2533` |
| recall-root | docs/incidents/ is a default search path | 5/10 | `go test -tags acs -run TestC2533_009 ./acs/cycle2533` |
