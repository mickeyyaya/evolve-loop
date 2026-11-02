---
score_cap:
  - criterion: "Each advisor seat prompt carries an artifact anchor, so a recorded advisor transcript resolves to the transcript source"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2098_001_|TestC2098_004_' ./acs/cycle2098"
  - criterion: "A transcript of another artifact or a prompt with no anchor stays unattributed"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2098_002_|TestC2098_003_' ./acs/cycle2098"
  - criterion: "The tokenusage package note records the agy cache-write field finding"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2098_005_' ./acs/cycle2098"
---

# Eval: advisor rows resolve from the transcript

> Pins inbox item claude-advisor-rows-measured (plan T9, cycle 2098, continuation of cycles 2061 and 2086). The advisor prompt itself names its deliverable behind the `Artifact path: ` anchor, so attribution does not depend on the adapter footer alone. The negative cases keep attribution tied to the artifact path. The agy cache-write verdict lives in the package note.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| advisor-anchor | Four advisor seats resolve to a measured transcript, one anchor each | 6/10 | `go test -tags acs -run 'TestC2098_001_|TestC2098_004_' ./acs/cycle2098` |
| no-misattribution | Other artifact and anchorless prompt stay unattributed | 7/10 | `go test -tags acs -run 'TestC2098_002_|TestC2098_003_' ./acs/cycle2098` |
| agy-finding | Package note gives the agy cache write a field verdict | 5/10 | `go test -tags acs -run 'TestC2098_005_' ./acs/cycle2098` |
