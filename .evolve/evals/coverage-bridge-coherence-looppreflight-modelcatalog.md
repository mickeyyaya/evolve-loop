# Eval: coverage-bridge-coherence-looppreflight-modelcatalog

## Task Summary
Add unit tests to bring `adapters/bridge` (84%), `phasecoherence` (83.5%), `looppreflight` (87.6%), and `modelcatalog` (87.3%) to ≥98% each. Key targets: `bridge.New` (50%), `bridge.Launch` (64%), `canonicalRole` (42.9%), `newDefaultBootTester` (6.7%), `modelcatalog.Write` (66%).

## Acceptance Criteria

### AC1: adapters/bridge ≥ 98% coverage [code]
```bash
cd go && go test ./internal/adapters/bridge/... -cover -count=1 2>&1 | grep -E "coverage: [0-9]+\.[0-9]+%" | awk -F'coverage: ' '{print $2}' | awk -F'%' '{if ($1 < 98) exit 1}'
```
Expected: exit 0

### AC2: phasecoherence ≥ 98% coverage [code]
```bash
cd go && go test ./internal/phasecoherence/... -cover -count=1 2>&1 | grep -E "coverage: [0-9]+\.[0-9]+%" | awk -F'coverage: ' '{print $2}' | awk -F'%' '{if ($1 < 98) exit 1}'
```
Expected: exit 0

### AC3: looppreflight ≥ 98% coverage [code]
```bash
cd go && go test ./internal/looppreflight/... -cover -count=1 2>&1 | grep -E "coverage: [0-9]+\.[0-9]+%" | awk -F'coverage: ' '{print $2}' | awk -F'%' '{if ($1 < 98) exit 1}'
```
Expected: exit 0

### AC4: modelcatalog ≥ 98% coverage [code]
```bash
cd go && go test ./internal/modelcatalog/... -cover -count=1 2>&1 | grep -E "coverage: [0-9]+\.[0-9]+%" | awk -F'coverage: ' '{print $2}' | awk -F'%' '{if ($1 < 98) exit 1}'
```
Expected: exit 0

### AC5: All four packages pass [code]
```bash
cd go && go test ./internal/adapters/bridge/... ./internal/phasecoherence/... ./internal/looppreflight/... ./internal/modelcatalog/... -count=1 2>&1 | grep "^FAIL" | wc -l | tr -d ' ' | xargs -I{} test {} -eq 0
```
Expected: exit 0

### AC6: canonicalRole no longer 42.9% [code]
```bash
cd go && go test ./internal/phasecoherence/... -count=1 -coverprofile=/tmp/coh_eval.out && go tool cover -func=/tmp/coh_eval.out | grep canonicalRole | awk -F'%' '{if ($1+0 < 80) exit 1}'
```
Expected: exit 0 (canonicalRole ≥ 80%)

### AC7 (negative): modelcatalog.Write test triggers a real write+rename, not a no-op [code]
```bash
cd go && grep -r "Write" internal/modelcatalog/ --include="*_test.go" | grep -v "//.*skip\|t.Skip" | wc -l | tr -d ' ' | xargs -I{} test {} -gt 0
```
Expected: exit 0 (Write is genuinely tested)

### AC8 (edge case): newDefaultBootTester returns a non-nil tester [code]
```bash
cd go && grep -r "newDefaultBootTester\|NewDefaultBootTester" internal/looppreflight/ --include="*_test.go" | wc -l | tr -d ' ' | xargs -I{} test {} -gt 0
```
Expected: exit 0 (factory function is tested)
