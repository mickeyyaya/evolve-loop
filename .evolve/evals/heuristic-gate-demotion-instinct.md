---
score_cap:
  - criterion: "HeuristicDemotionChecker detects identical reason-template hash across 2 consecutive cycle failedApproaches entries for the same gate-id and returns a demotion verdict"
    max_if_missing: 9
    evidence: "cd go && go test -run '^TestHeuristicGateDemotion_FiringOnIdenticalReject$' -v ./internal/gatepolicy/... 2>&1 | grep -q 'PASS'"
  - criterion: "Integrity floor (ship => build AND audit AND tdd) is registered as fact-gate and CheckForDemotion never demotes it"
    max_if_missing: 10
    evidence: "cd go && go test -run '^TestHeuristicGateDemotion_IntegrityFloorNeverDemotes$' -v ./internal/gatepolicy/... 2>&1 | grep -q 'PASS'"
  - criterion: "Demotion auto-files exactly one inbox HIGH item in .evolve/inbox/ with the gate-id and reason-template hash"
    max_if_missing: 8
    evidence: "cd go && go test -run '^TestHeuristicGateDemotion_AutoFilesInboxHigh$' -v ./internal/gatepolicy/... 2>&1 | grep -q 'PASS'"
  - criterion: "Demotion is scoped to one cycle only: the gate reverts to enforce on the cycle after the shadow cycle"
    max_if_missing: 7
    evidence: "cd go && go test -run '^TestHeuristicGateDemotion_OneCycleScope$' -v ./internal/gatepolicy/... 2>&1 | grep -q 'PASS'"
  - criterion: "Single identical cycle-pair does NOT trigger demotion (requires 2 consecutive identical rejections)"
    max_if_missing: 8
    evidence: "cd go && go test -run '^TestHeuristicGateDemotion_SingleCycleNoFire$' -v ./internal/gatepolicy/... 2>&1 | grep -q 'PASS'"
  - criterion: "ReasonTemplateHash collapses same-magnitude numeric drift (7 vs 6) to identical hashes but distinguishes order-of-magnitude differences (7 vs 700)"
    max_if_missing: 10
    evidence: "cd go && go test -run '^TestReasonTemplateHash' -v ./internal/gatepolicy/... 2>&1 | grep -q 'PASS'"
  - criterion: "Composition-root wiring (FileDemotionInbox / ShouldShadow calls in cmd_cycle.go) has direct test so mutation-gate executes mutants against the wiring"
    max_if_missing: 8
    evidence: "cd go && go test -run '^TestTriageCapHeuristicDemotionWiring' -v ./cmd/evolve/... 2>&1 | grep -q 'PASS'"
---

# Eval: Heuristic gate demotion instinct (ADR-0046 Layer 2)

> Pins the ADR-0046 Layer 2 structural fix: heuristic gates (floor counts,
> prose classifiers) that reject with a byte-identical reason TEMPLATE across
> 2 consecutive cycles are treated as gate defects, not work defects, and are
> demoted to shadow for the next cycle only. The integrity floor
> (ship => build AND audit AND (tdd unless trivial)) is fact-class and NEVER
> demotable. Implementation rides a new pure package `go/internal/gatepolicy`
> keyed on gate-id + reason-template hash from failedApproaches. One-cycle
> scope + auto-filed inbox defect keeps the behavior ungameable.
>
> Cycle-306 lesson: ReasonTemplateHash must be MAGNITUDE-SENSITIVE. Same-magnitude
> numeric drift (7 vs 6, cap 5 vs 5) collapses to one template so demotion fires
> on a genuinely-stuck gate. But order-of-magnitude differences (7 vs 700) must
> hash differently — erasing all digits wholesale is wrong. Use digit-count buckets
> (#1 for single-digit, #2 for two-digit, #3 for three-digit etc.) so "7 vs 6"
> collapses and "7 vs 700" stays distinct.
>
> Cycle-306 M1 lesson: the composition-root wiring (the thin cmd_cycle.go calls
> to FileDemotionInbox / ShouldShadow) must have a direct test so the mutation-gate
> actually executes mutants against it — not just the pure gatepolicy functions.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| demotion-fires | identical-reject × 2 consecutive → shadow | 9/10 | `TestHeuristicGateDemotion_FiringOnIdenticalReject` PASS |
| integrity-never-demotes | fact-gate (integrity floor) never demoted | 10/10 | `TestHeuristicGateDemotion_IntegrityFloorNeverDemotes` PASS |
| auto-file-inbox | demotion writes exactly 1 inbox HIGH item | 8/10 | `TestHeuristicGateDemotion_AutoFilesInboxHigh` PASS |
| one-cycle-scope | shadow reverts after one cycle | 7/10 | `TestHeuristicGateDemotion_OneCycleScope` PASS |
| single-not-fire | single identical cycle does NOT fire | 8/10 | `TestHeuristicGateDemotion_SingleCycleNoFire` PASS |
| magnitude-sensitive-hash | 7 vs 6 collapse; 7 vs 700 distinct | 10/10 | `TestReasonTemplateHash` PASS (adversarial from cycle-306) |
| wiring-mutation-coverage | cmd_cycle.go demotion calls have direct test | 8/10 | `TestTriageCapHeuristicDemotionWiring` PASS |

## Acceptance Criteria

### C1: Demotion fires on 2 identical cycle rejections [code]
```bash
cd go && go test -run '^TestHeuristicGateDemotion_FiringOnIdenticalReject$' -v ./internal/gatepolicy/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

**Replay corpus:** cycles 301+302 failedApproaches — both emitted `"triage overpacked: N committed coverage floors exceed cap M"` with same-magnitude N/M. Template hash must collapse across both so demotion fires.

**Negative case — gaming fake:** A checker that always returns demotion regardless of history cannot pass `TestHeuristicGateDemotion_SingleCycleNoFire` (single identical cycle must NOT fire).

### C2: Integrity floor never demotes regardless of repetition [code]
```bash
cd go && go test -run '^TestHeuristicGateDemotion_IntegrityFloorNeverDemotes$' -v ./internal/gatepolicy/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

### C3: Demotion auto-files exactly one inbox HIGH item [code]
```bash
cd go && go test -run '^TestHeuristicGateDemotion_AutoFilesInboxHigh$' -v ./internal/gatepolicy/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

**Edge case:** calling demotion twice with the same gate-id must still produce exactly one inbox file (idempotent filename).

### C4: One-cycle-scoped shadow [code]
```bash
cd go && go test -run '^TestHeuristicGateDemotion_OneCycleScope$' -v ./internal/gatepolicy/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

### C5: Single identical cycle does NOT fire [code]
```bash
cd go && go test -run '^TestHeuristicGateDemotion_SingleCycleNoFire$' -v ./internal/gatepolicy/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

### C6: Magnitude-sensitive hash (adversarial from cycle-306) [code]
```bash
cd go && go test -run '^TestReasonTemplateHash' -v ./internal/gatepolicy/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

**Specifically:** `ReasonTemplateHash("triage overpacked: 7 committed floors exceed cap 6")` must equal `ReasonTemplateHash("triage overpacked: 6 committed floors exceed cap 5")` (same-magnitude collapse for demotion). AND `ReasonTemplateHash("declared 7 floors exceeds cap 6")` must NOT equal `ReasonTemplateHash("declared 700 floors exceeds cap 600")` (order-of-magnitude → distinct).

**Gaming fake named:** digit-erasure (`\d+` → `#`) incorrectly collapses the second pair — the negative test `TestReasonTemplateHash_DistinguishesSemanticallyDifferentNumericThresholds` fails.

### C7: Composition-root wiring has direct mutation-testable coverage [code]
```bash
cd go && go test -run '^TestTriageCapHeuristicDemotionWiring' -v ./cmd/evolve/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

## Grader type summary
- C1–C7: `[code]` — all criteria are executable Go test assertions
- No `[model]` or `[human]` graders needed; behavior is deterministic
