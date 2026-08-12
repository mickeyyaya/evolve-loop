---
score_cap:
  - criterion: "A successful salvage returns a Result whose Content is the repaired bytes, and those bytes re-verify clean through the production verifier"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1442_001_SalvagedResultCarriesTheRepairedBytes ./acs/cycle1442"
  - criterion: "When ArtifactPath is set, a salvage that approves persists the repaired bytes to that path, so the artifact on disk re-verifies clean after the production gate (Reviewer.Review) approves it"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1442_002_SalvagePersistsRepairedBytesToTheArtifact ./acs/cycle1442"
  - criterion: "A REFUSED salvage still mutates nothing — neither Result.Content nor the artifact on disk"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1442_003_RefusedSalvageMutatesNothing ./acs/cycle1442"
  - criterion: "The deliverable package's own suite covers post-salvage Content and stays green"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestSalvage ./internal/deliverable"
---

# Eval: Salvage must persist the bytes it approved

> Pins the post-salvage byte-identity contract of `salvageVerdictWith`
> (`go/internal/deliverable/salvage_extract.go`). Cycle-1441's audit returned FAIL
> with defect H1 (HIGH): the function re-verifies a locally-`repaired` string,
> and on success returns a `Result` struct-copied from the ORIGINAL — flipping
> only `OK` and `Violations`, discarding `repaired` entirely. The contract gate
> therefore reported `OK: true` over a byte stream that was never the byte stream
> it verified, and the production caller (`reviewer.go:138`) discards the salvaged
> `Result` outright, so a downstream phase re-reading `ArtifactPath` saw the
> malformed original. The eval keeps both halves nailed down permanently: the
> in-memory `Content` and the on-disk artifact must both carry the approved bytes,
> and a REFUSED salvage must still change nothing (the anti-overreach half — the
> fix must not become an unconditional write).
>
> Source incident: cycle 1441 (`.evolve/runs/cycle-1441/audit-fail-reason.json`, H1),
> continued as cycle 1442 task `fix-salvage-content-persistence`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| content-carries-repaired | Salvaged `Result.Content` re-verifies clean | 8/10 | `go test -tags acs -run TestC1442_001... ./acs/cycle1442` |
| artifact-persisted | On-disk artifact re-verifies clean after the real gate approves | 8/10 | `go test -tags acs -run TestC1442_002... ./acs/cycle1442` |
| refusal-mutates-nothing | Refused salvage leaves memory and disk byte-identical | 7/10 | `go test -tags acs -run TestC1442_003... ./acs/cycle1442` |
| package-regression | `TestSalvage*` in the owning package green | 6/10 | `go test -run TestSalvage ./internal/deliverable` |
