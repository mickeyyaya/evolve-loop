# Eval: phasecoherence-canonical-role-coverage

> Cycle-319 baseline: `canonicalRole` 42.9% (all named branches missed — existing
> test sends only capitalized inputs that fall to the default), `dispatchNone`
> 75%, `Check` 79.1%; package total 85.2%. Fix the existing
> `canonical_role_amplification_test.go` to send exact lowercase literals matching
> each switch branch, and add a `dispatchNone` test scenario.

## Acceptance Criteria

### AC1: canonicalRole switch branches all covered — lowercase literals hit named cases [code]

```bash
cd go && go test ./internal/phasecoherence/... -run TestCanonicalRole -v -count=1
```

Expected: `PASS`; tests verify every switch case: `canonicalRole("scout")=="scout"`, `canonicalRole("builder")=="builder"`, `canonicalRole("build")=="builder"`, `canonicalRole("auditor")=="auditor"`, `canonicalRole("audit")=="auditor"`, `canonicalRole("intent")=="intent"`, `canonicalRole("memo")=="memo"`, and the default path.

### AC2: canonicalRole("BUILDER") hits the default branch, not the named case (edge — confirms case-sensitivity) [code]

```bash
cd go && go test ./internal/phasecoherence/... -run TestCanonicalRole -v -count=1 2>&1 | grep -E "(PASS|FAIL)"
```

Expected: `PASS`

### AC3: dispatchNone uncovered path exercised [code]

```bash
cd go && go test ./internal/phasecoherence/... -run TestDispatchNone -v -count=1
```

Expected: `PASS`; test covers the path where `dispatchNone` returns true — no persona content available for the expected phase.

### AC4: Coverage floor — phasecoherence ≥ 88% (cycle-319 lesson: set just below delivered, baseline 85.2%) [code]

```bash
cd go && go test ./internal/phasecoherence/... -cover -count=1 2>&1 | grep "coverage:"
```

Expected: `coverage: 88.0%` or higher.

### AC5: No regressions [code]

```bash
cd go && go test ./internal/phasecoherence/... -count=1
```

Expected: `ok` with zero failures.
