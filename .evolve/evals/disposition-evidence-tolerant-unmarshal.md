---
score_cap:
  - criterion: "A defect-dispositions.json whose `evidence` is a JSON array of resolvable citations is READ by the gate and honoured as a closure (PASS), not rejected as an unparseable document"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -v -run '^TestClassify_DispositionEvidenceArrayShapeAccepted$' ./internal/phases/audit | grep -q '^--- PASS: TestClassify_DispositionEvidenceArrayShapeAccepted'"
  - criterion: "The single-string `evidence` shape in production use today keeps working after the tolerance change"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -v -run '^TestClassify_DispositionEvidenceStringShapeAccepted$' ./internal/phases/audit | grep -q '^--- PASS: TestClassify_DispositionEvidenceStringShapeAccepted'"
  - criterion: "Shape tolerance does not become claim tolerance: an array of citations that resolve to no file still blocks, and blocks on RESOLUTION rather than on parsing"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -v -run '^TestClassify_DispositionEvidenceArrayShapeUnresolvableStillBlocks$' ./internal/phases/audit | grep -q '^--- PASS: TestClassify_DispositionEvidenceArrayShapeUnresolvableStillBlocks'"
  - criterion: "An empty `evidence` array on a FIXED claim is treated as no evidence and blocks"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -v -run '^TestClassify_DispositionEvidenceEmptyArrayOnFixedStillBlocks$' ./internal/phases/audit | grep -q '^--- PASS: TestClassify_DispositionEvidenceEmptyArrayOnFixedStillBlocks'"
  - criterion: "An `evidence` value that is neither string nor array of strings is still rejected outright — never silently degraded to empty (cycle-1285 F2 no-silent-degrade posture)"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -v -run '^TestClassify_DispositionEvidenceObjectShapeStillBlocks$' ./internal/phases/audit | grep -q '^--- PASS: TestClassify_DispositionEvidenceObjectShapeStillBlocks'"
  - criterion: "The audit verdict path carries no regression from the decoder change"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/phases/audit"
---

# Eval: Disposition `evidence` accepts a JSON array, not only a string

> Pins the wire contract of `<workspace>/defect-dispositions.json`'s `evidence`
> field at the gate seam. `defectDispositionDoc.Evidence` is typed `string`
> (`go/internal/phases/audit/defect_ledger.go:89`), so an auditor who cited two
> files as a JSON array had its ENTIRE disposition document rejected by
> `encoding/json` — `cannot unmarshal array into Go struct field
> .dispositions.evidence of type string` — and the cycle blocked on
> "unparseable" despite the work being done and cited. Source incident:
> cycle-1399 (inbox `defect-disposition-contract-unsatisfiable`); cycles 1397
> and 1400 died on the neighbouring absent-file branch. #419 (`fdc9c3e3`)
> tolerated a *decorated* cite STRING and does not touch the JSON type, so it
> did not close this defect.
>
> The caps are asymmetric on purpose. Missing the array-acceptance criterion is
> the reported outage (cap 3). Missing either anti-gaming criterion is worse
> than the outage — a decoder that accepts an array and stops checking whether
> the citations resolve converts a blocked cycle into a laundered one, which is
> the exact failure the defect ledger exists to prevent (caps 4 and 3).
>
> Cosmetic satisfaction is explicitly out of contract: joining array elements
> into one `"a.go:1; b.go:2"` string parses but is not a path, so
> `evidenceResolves` (`defect_ledger.go:267`) rejects it and the operator-visible
> behaviour is unchanged. Every criterion below is asserted at the verdict
> returned by `hooks{}.Classify`, never at the decoder, so a join cannot pass it.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| array-accepted (crux) | array of resolvable cites ⇒ PASS, no "unparseable" | 3/10 | `go test -run TestClassify_DispositionEvidenceArrayShapeAccepted` |
| string-regression | the shape in production use keeps working | 5/10 | `go test -run TestClassify_DispositionEvidenceStringShapeAccepted` |
| anti-gaming/unresolvable | array of dead cites still blocks, on resolution | 4/10 | `go test -run TestClassify_DispositionEvidenceArrayShapeUnresolvableStillBlocks` |
| edge/empty-array | `[]` on FIXED is no-evidence, blocks | 5/10 | `go test -run TestClassify_DispositionEvidenceEmptyArrayOnFixedStillBlocks` |
| no-silent-degrade | object shape still rejected outright | 3/10 | `go test -run TestClassify_DispositionEvidenceObjectShapeStillBlocks` |
| no-regression | whole audit verdict path stays green | 6/10 | `go test ./internal/phases/audit` |
