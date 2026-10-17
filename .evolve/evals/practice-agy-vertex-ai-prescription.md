---
score_cap:
  - criterion: "The v2.24.0 section names Vertex AI and the project-access condition"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1920_00[1-4]' ./acs/cycle1920"
  - criterion: "TestC1915_002 requires Vertex AI in the v2.24.0 section"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1920_005|TestC1915_002' ./acs/cycle1920 ./acs/cycle1915"
---

# Eval: Vertex AI prescription for the v2.24.0 changelog section

> Pins carryover d4657d4f721a2442d1f7a22c760371245: the v2.24.0 section of the agy changelog research page explains Vertex AI and its project-access condition as data, and TestC1915_002 requires it. Source incident: cycle 1919 audit prescription.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| doc-sentence | Vertex AI sentence with project access, 25 words or fewer | 6/10 | `go test -tags acs -run TestC1920_00[1-4] ./acs/cycle1920` |
| suite-list | TestC1915_002 lists Vertex AI | 7/10 | `go test -tags acs -run TestC1920_005 ./acs/cycle1920` |
