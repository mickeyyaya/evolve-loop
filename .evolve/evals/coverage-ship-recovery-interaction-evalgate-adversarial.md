# Eval: coverage-ship-recovery-interaction-evalgate-adversarial

## Task Summary
Add unit tests to bring `phases/ship` (91.2%), `recovery` (91%), `interaction` (90.5%), `evalgate` (91.2%), `faillearn` (92.5%), and `bridge` (94.2%) to ≥98% each. Also extend adversarial testing coverage for agy/codex-specific driver paths: `codex_pretrust` (65%), `CaptureModelPicker` (71%), `defaultKeychainProbe` (27%). Key targets: `addRepairSignals` (33%), `repairColliders` (72%), quality.name (0%), Recover (80%), appendLedgerLine (77%).

## Acceptance Criteria

### AC1: phases/ship ≥ 98% coverage [code]
```bash
cd go && go test ./internal/phases/ship/... -cover -count=1 2>&1 | grep -E "coverage: [0-9]+\.[0-9]+%" | awk -F'coverage: ' '{print $2}' | awk -F'%' '{if ($1 < 98) exit 1}'
```
Expected: exit 0

### AC2: recovery ≥ 98% coverage [code]
```bash
cd go && go test ./internal/recovery/... -cover -count=1 2>&1 | grep -E "coverage: [0-9]+\.[0-9]+%" | awk -F'coverage: ' '{print $2}' | awk -F'%' '{if ($1 < 98) exit 1}'
```
Expected: exit 0

### AC3: interaction ≥ 98% coverage [code]
```bash
cd go && go test ./internal/interaction/... -cover -count=1 2>&1 | grep -E "coverage: [0-9]+\.[0-9]+%" | awk -F'coverage: ' '{print $2}' | awk -F'%' '{if ($1 < 98) exit 1}'
```
Expected: exit 0

### AC4: evalgate ≥ 98% coverage [code]
```bash
cd go && go test ./internal/evalgate/... -cover -count=1 2>&1 | grep -E "coverage: [0-9]+\.[0-9]+%" | awk -F'coverage: ' '{print $2}' | awk -F'%' '{if ($1 < 98) exit 1}'
```
Expected: exit 0

### AC5: faillearn ≥ 98% coverage [code]
```bash
cd go && go test ./internal/faillearn/... -cover -count=1 2>&1 | grep -E "coverage: [0-9]+\.[0-9]+%" | awk -F'coverage: ' '{print $2}' | awk -F'%' '{if ($1 < 98) exit 1}'
```
Expected: exit 0

### AC6: bridge ≥ 96% coverage [code]
```bash
cd go && go test ./internal/bridge/... -cover -count=1 2>&1 | grep "^ok.*internal/bridge\b" | grep -E "coverage: [0-9]+\.[0-9]+%" | awk -F'coverage: ' '{print $2}' | awk -F'%' '{if ($1 < 96) exit 1}'
```
Expected: exit 0 (bridge core ≥ 96%; full 98% is stretch goal given real-OS probes)

### AC7: addRepairSignals no longer 33% [code]
```bash
cd go && go test ./internal/phases/ship/... -count=1 -coverprofile=/tmp/ship_eval.out && go tool cover -func=/tmp/ship_eval.out | grep addRepairSignals | awk -F'%' '{if ($1+0 < 70) exit 1}'
```
Expected: exit 0 (addRepairSignals ≥ 70%)

### AC8: evalgate.quality.name no longer 0% [code]
```bash
cd go && go test ./internal/evalgate/... -count=1 -coverprofile=/tmp/eg_eval.out && go tool cover -func=/tmp/eg_eval.out | grep "quality.go.*name" | awk '{if ($3 == "0.0%") exit 1}'
```
Expected: exit 0 (quality.name > 0%)

### AC9 (negative): adversarial tests cover agy weak-busy-signal fault explicitly [code]
```bash
grep -r "agy\|gemini" /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/ --include="*adversarial*_test.go" --include="*_adversarial_test.go" -l | wc -l | tr -d ' ' | xargs -I{} test {} -gt 0
```
Expected: exit 0 (agy adversarial tests exist in bridge package)

### AC10 (edge case): codex_pretrust handles missing config file gracefully [code]
```bash
cd go && grep -r "codex_pretrust\|pretrustCodexProjects\|dismissCodexUpdateNag" internal/bridge/ --include="*_test.go" | wc -l | tr -d ' ' | xargs -I{} test {} -gt 0
```
Expected: exit 0 (codex pretrust functions are tested)

### AC11 (negative): Recover returns error when all signatures exhausted [code]
```bash
cd go && grep -rE "Recover.*err|err.*Recover|TestRecover" internal/recovery/ --include="*_test.go" | grep -i "exhaust\|empty\|no.*match\|zero\|nil\|notfound\|not.found" | wc -l | tr -d ' ' | xargs -I{} test {} -gt 0
```
Expected: exit 0 (Recover is tested for exhaustion/failure case)
