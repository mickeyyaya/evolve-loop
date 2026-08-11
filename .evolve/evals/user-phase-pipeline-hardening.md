---
score_cap:
  - criterion: "ValidateUserSpec enforces two-tier naming: single-word user phase names rejected, multi-word kebab accepted"
    max_if_missing: 7
    evidence: "cd go && go test -run 'TestValidateUserSpec_SingleWordRejected|TestValidateUserSpec_MultiWordWithDigitsOK|TestValidateUserSpec_TrailingHyphenRejected|TestValidateUserSpec_AdversarialRejections' ./internal/phasespec/"
  - criterion: "LedgerEntry.Source (omitempty) + phase_skipped entries carry psmas|router|content attribution"
    max_if_missing: 7
    evidence: "cd go && go test -run 'TestPhaseSkipped_SourceAttribution|TestLedgerEntry_SourceOmittedWhenEmpty' -timeout 60s ./internal/core/"
  - criterion: "ResolveRegistryPath prefers .evolve/phase-registry.json over docs/architecture/phase-registry.json"
    max_if_missing: 6
    evidence: "cd go && go test -run 'TestResolveRegistryPath_PrefersEvolveRegistry|TestResolveRegistryPath_FallsBackToDocsRegistry|TestResolveRegistryPath_EvolveDirExistsNoFile|TestResolveRegistryPath_NeitherFileExists|TestResolveRegistryPath_EmptyRoot' ./internal/config/"
  - criterion: ".evolve/phase-registry.json exists and pins dynamic_routing=advisory through the real Load path"
    max_if_missing: 6
    evidence: "cd go && go test -run 'TestRealRegistry_EvolveAdvisoryPinned' ./internal/config/"
---

# Eval: user-phase-pipeline-hardening

## Summary
Verifies four pipeline-hygiene sub-fixes:
5. `ValidateUserSpec` enforces two-tier naming (single-word rejected)
6. `LedgerEntry.Source` field + `phase_skipped` entries carry skip-source attribution
7. `ResolveRegistryPath` prefers `.evolve/phase-registry.json`, falls back to `docs/architecture`
8. `.evolve/phase-registry.json` created with `dynamic_routing=advisory`

## Criteria

### AC-005: Two-tier naming lint [code]
```bash
cd "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go"
go test ./internal/phasespec/... \
  -run "TestValidateUserSpec_SingleWordRejected|TestValidateUserSpec_MultiWordWithDigitsOK|TestValidateUserSpec_TrailingHyphenRejected|TestValidateUserSpec_AdversarialRejections" \
  -v 2>&1
```

### AC-005 structural: userPhaseNameRE exists in validate.go [code]
```bash
grep -q "userPhaseNameRE" \
  "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go/internal/phasespec/validate.go" \
  && echo PASS || echo "FAIL: userPhaseNameRE not found"
```

### AC-005 negative: single-word name must be rejected [code]
```bash
cd "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go"
go test ./internal/phasespec/... -run "TestValidateUserSpec_SingleWordRejected" -v 2>&1
# Must PASS (test expects an error for single-word name)
```

### AC-006: LedgerEntry.Source field + phase_skipped attribution [code]
```bash
cd "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go"
go test ./internal/core/... \
  -run "TestPhaseSkipped_SourceAttribution|TestLedgerEntry_SourceOmittedWhenEmpty" \
  -v -timeout 60s 2>&1
```

### AC-006 structural: Source field in LedgerEntry with omitempty [code]
```bash
grep -q 'Source.*string.*json:"source,omitempty"' \
  "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go/internal/core/ports.go" \
  && echo PASS || echo "FAIL: LedgerEntry.Source field missing"
```

### AC-007: ResolveRegistryPath fallback chain [code]
```bash
cd "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go"
go test ./internal/config/... \
  -run "TestResolveRegistryPath_PrefersEvolveRegistry|TestResolveRegistryPath_FallsBackToDocsRegistry|TestResolveRegistryPath_NeitherFileExists" \
  -v 2>&1
```

### AC-007 structural: ResolveRegistryPath function exists [code]
```bash
grep -q "func ResolveRegistryPath(" \
  "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go/internal/config/config.go" \
  && echo PASS || echo "FAIL: ResolveRegistryPath not found"
```

### AC-008: .evolve/phase-registry.json has dynamic_routing=advisory [code]
```bash
REGISTRY="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/.evolve/phase-registry.json"
[ -f "${REGISTRY}" ] || { echo "FAIL: .evolve/phase-registry.json missing"; exit 1; }
grep -q '"dynamic_routing"' "${REGISTRY}" && grep -q '"advisory"' "${REGISTRY}" \
  && echo PASS || echo "FAIL: .evolve/phase-registry.json missing advisory dynamic_routing"
```

### AC-008 negative: "0" (old off value) must not remain as sole dynamic_routing value [code]
```bash
REGISTRY="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/.evolve/phase-registry.json"
# The value must be "advisory", not "0" or "off"
val=$(python3 -c "import json,sys; d=json.load(open('${REGISTRY}')); print(d.get('config',{}).get('dynamic_routing','MISSING'))")
[ "${val}" = "advisory" ] && echo PASS || echo "FAIL: dynamic_routing=${val}, want advisory"
```

### Full suite regression [code]
```bash
cd "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go"
go test ./internal/phasespec/... ./internal/core/... ./internal/config/... \
  -timeout 120s 2>&1 | tail -20
```
