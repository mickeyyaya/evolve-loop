---
score_cap:
  - criterion: "A lesson YAML in docs/retrospectives/ is recalled for a matching goal through the default roots"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2137_001' ./acs/cycle2137 && go test -count=1 -run 'TestKBRootsAbs_RecallsALessonFromDocsRetrospectives' ./cmd/evolve"
  - criterion: "A policy value of kb_search_paths still replaces the default list"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2137_002' ./acs/cycle2137 && go test -count=1 -run 'TestSearchPathsFromEnv_AllBranches' ./internal/research"
  - criterion: "The package note of internal/research names docs/retrospectives"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2137_003' ./acs/cycle2137"
---

# Eval: docs/retrospectives/ joins the default recall roots

> Pins the recovery retrospective C6 contract. The default list `defaultSearchPaths` in `go/internal/research/kb.go` must include `docs/retrospectives/` as a flat root, a policy `kb_search_paths` value must still replace the whole list, and the package note must name the new root. Source: inbox item recovery-retrospective-c6-recall-root, cycle 2137.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| default-root-recall | a lesson in docs/retrospectives is recalled, and the three older roots still are | 4/10 | `go test -tags acs -run TestC2137_001 ./acs/cycle2137` |
| production-caller | `kbRootsAbs`, the caller at cmd_cycle.go:576, resolves the new root | 4/10 | `go test -run TestKBRootsAbs_RecallsALessonFromDocsRetrospectives ./cmd/evolve` |
| policy-replaces | a policy value replaces the list and never adds the new root | 5/10 | `go test -tags acs -run TestC2137_002 ./acs/cycle2137` |
| note-names-root | the package note names the root | 6/10 | `go test -tags acs -run TestC2137_003 ./acs/cycle2137` |
