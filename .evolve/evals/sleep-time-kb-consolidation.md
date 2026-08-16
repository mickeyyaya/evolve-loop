# Eval: Sleep-time KB consolidation — typed recall bound + novelty-gated lesson writes

Pins the two seams cycle-1494 materialised for the `sleep-time-kb-consolidation`
inbox item, after the premise-challenge gate probed the originally planned seams
FALSE (`research.maxResults` feeds the ADVISOR's `recallForPlan`, never Scout;
memo does not write the lessons corpus — `faillearn.WriteArtifacts` does).

Two durable contracts:

1. **Recall bound is typed policy, default HELD at 5.** `policy.ResearchConfig().RecallK`
   resolves the bound, clamps operator typos (0/negative/absurd ⇒ 5), the KB
   enforces it as a strict PREFIX of the existing deterministic ranking, and the
   value is resolved from policy at the composition root — not a compiled literal.
   The held default is load-bearing: lowering it silently narrows advisor
   failure-recall, a phase-integrity regression the goal forbids.
2. **Novelty gate lives on the real Go lesson-write seam.** `faillearn.WriteArtifacts`
   suppresses a near-duplicate lesson (the inbox item's "identical observation
   twice → one write"), still writes materially different failures, and is
   non-destructive in the presence of corpus rot.

## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go test -count=1 ./internal/policy -run 'TestResearchConfig'`
- `[code]` `cd go && go test -count=1 ./internal/research -run 'TestFileKB.*Recall'`
- `[code]` `cd go && go test -count=1 ./internal/faillearn -run 'TestWriteArtifacts.*Novelty'`

## Regression Evals (full test suite)
- `[code]` `cd go && go test -count=1 ./internal/policy ./internal/research ./internal/faillearn ./internal/core`

## Acceptance Checks
- `[code]` `cd go && go test -tags acs -count=1 ./acs/cycle1494 -run 'TestC1494_00[1234]_' # recall bound: default 5 held, clamped, enforced as a top-k prefix, legacy constructor unchanged`
- `[code]` `cd go && go test -tags acs -count=1 ./acs/cycle1494 -run 'TestC1494_005_KBCompositionRootDerivesRecallFromPolicy$' # wiring: cmd/evolve/cmd_cycle.go core.WithKB(...) receives a policy-RESOLVED recall, never a literal`
- `[code]` `cd go && go test -tags acs -count=1 ./acs/cycle1494 -run 'TestC1494_007_NoveltyGateSuppressesNearDuplicateLesson$' # the inbox item's literal regression, driven through the production writer`
- `[code]` `cd go && go test -tags acs -count=1 ./acs/cycle1494 -run 'TestC1494_008_NoveltyGateRetainsDistinctLesson$' # negative: a suppress-everything gate must fail here`
- `[code]` `cd go && go test -tags acs -count=1 ./acs/cycle1494 -run 'TestC1494_009_NoveltyGateMalformedCorpusEntryIsNonDestructive$' # edge/OOD: corpus rot must neither suppress the new lesson nor mutate the rotten file`
- `[code]` `cd go && go test -count=1 -race ./internal/faillearn ./internal/research`
- `[model]` Rubric: "The recall bound is resolved from .evolve/policy.json at the composition root with the default held at 5 (no narrowing of advisor recall), and the novelty gate is applied inside the production lesson-write path rather than in a helper only tests call." — threshold: >= 80

## Adversarial Cases
- **Negative:** a gate that suppresses every write passes the duplicate test and fails `TestC1494_008` (distinct failure must still land).
- **Negative:** a recall of 0/-1 must fall back to 5, never return zero lessons — a zero-recall KB silently disables advisor recall memory.
- **Edge/OOD:** an unparseable `*.yaml` neighbour in the lessons dir must not suppress the incoming lesson and must not be rewritten or deleted.
- **Cheapest gaming fake:** adding `ResearchConfig()` plus `NewFileKBWithRecall` while leaving `cmd_cycle.go` calling `research.NewFileKB(...)` — dead config that every unit test still greens. `TestC1494_005` fails that fake by asserting the composition root's `core.WithKB` argument carries a call-expression recall argument.
- **Second gaming fake:** gating near-duplicates in a new helper that the production writer never calls. `TestC1494_007` fails it by driving `faillearn.WriteArtifacts` itself and counting files on disk.

## Thresholds
- All checks: pass@1 = 1.0
