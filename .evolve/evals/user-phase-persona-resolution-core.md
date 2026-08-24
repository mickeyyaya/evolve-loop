---
score_cap:
  - criterion: "AgentForPhase resolves .evolve/phases/<name>/agent.md before agents/<name>.md (Mode 1 fix)"
    max_if_missing: 6
    evidence: "cd go && go test -run 'TestAgentForPhase_PhaseDirWins|TestAgentForPhase_FallsBackToAgentDir|TestMode1_PhaseDirPersonaResolution' ./internal/prompts/"
  - criterion: "clampDispatchable drops undispatchable phases at plan-apply time, never crashes, never drops the floor (Mode 2 fix)"
    max_if_missing: 6
    evidence: "cd go && go test -run 'TestNoCrash_PersonalessPhaseInPlan|TestClampPlan_UndispatchableDroppedWithWarn|TestClampPlan_PreflightFailureDropsPhase|TestClampDispatchable_NeverDropsFloor' -timeout 60s ./internal/core/"
  - criterion: "profiles.Resolve falls back name -> default-<role>.json -> zero Profile without error (Mode 3 fix)"
    max_if_missing: 7
    evidence: "cd go && go test -run 'TestResolve_NameHit|TestResolve_RoleFallback|TestResolve_ZeroProfileDefault' ./internal/profiles/"
  - criterion: "Registrar rejects persona/artifact name conflicts and warns on absent artifact reference (Mode 4 fix)"
    max_if_missing: 7
    evidence: "cd go && go test -run 'TestRegister_ArtifactNameConflictRejected|TestRegister_ArtifactNameAbsentWarns' ./internal/phaseregistrar/"
---

# Eval: user-phase-persona-resolution-core

> **RE-SCOPE (2026-08-24, cycle-1551 fix):** the landed design resolves this
> class differently — registration-seam demotion to catalog:"on-demand"
> (`demotePersonalessSpecs` inside `discoverUserSpecsClamped`) + dispatch-time
> fail-soft (`core.ErrAgentDocMissing` → `optionalInfraSkip`) + the tracked-menu
> guard test. `agents/<name>.md` remains the ONLY persona source for disk specs;
> the phase-local `agent.md` resolution mode (Loader.AgentForPhase) specified
> below is NOT implemented and must not land without first retiring one of the
> existing mechanisms (three-for-one-class ceiling). See
> docs/incidents/2026-08-24-personaless-menu-phase-lane-kill.md.


## Summary
Verifies that the four dispatch-safety sub-fixes land correctly:
1. `AgentForPhase` tries `.evolve/phases/<name>/agent.md` before `agents/<name>.md`
2. `clampDispatchable` drops undispatchable phases at plan-apply time without crashing
3. `profiles.Resolve` falls back name → role-default → zero Profile
4. `PersonaArtifactDrift` check wired into the registrar

## Criteria

### AC-001: AgentForPhase two-path resolution [code]
```bash
cd "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go"
go test ./internal/prompts/... \
  -run "TestAgentForPhase_PhaseDirWins|TestAgentForPhase_FallsBackToAgentDir|TestMode1_PhaseDirPersonaResolution" \
  -v 2>&1
# PASS if exit 0 and all named tests PASS
```

### AC-001 structural: AgentForPhase method exists [code]
```bash
grep -q "func (l \*Loader) AgentForPhase(" \
  "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go/internal/prompts/prompts.go" \
  && echo PASS || echo "FAIL: AgentForPhase not found"
```

### AC-002: clampDispatchable no-crash on personaless phase [code]
```bash
cd "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go"
go test ./internal/core/... \
  -run "TestNoCrash_PersonalessPhaseInPlan|TestClampPlan_UndispatchableDroppedWithWarn|TestClampDispatchable_NeverDropsFloor" \
  -v -timeout 60s 2>&1
```

### AC-002 structural: clampDispatchable call-site exists [code]
```bash
grep -q "o.clampDispatchable(" \
  "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go/internal/core/orchestrator.go" \
  && echo PASS || echo "FAIL: clampDispatchable call not found"
```

### AC-003: profiles.Resolve fallback chain [code]
```bash
cd "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go"
go test ./internal/profiles/... \
  -run "TestResolve_NameHit|TestResolve_RoleFallback|TestResolve_ZeroProfileDefault" \
  -v 2>&1
```

### AC-003 structural: Resolve method exists [code]
```bash
grep -q "func (l \*Loader) Resolve(" \
  "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go/internal/profiles/profiles.go" \
  && echo PASS || echo "FAIL: Resolve not found"
```

### AC-004: PersonaArtifactDrift wired in registrar [code]
```bash
cd "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go"
go test ./internal/phaseregistrar/... \
  -run "TestRegister_ArtifactNameConflictRejected|TestRegister_ArtifactNameAbsentWarns" \
  -v 2>&1
```

### Negative: agent failing mid-pipeline (regression guard) [code]
```bash
cd "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go"
go test ./internal/prompts/... -v -timeout 30s 2>&1
# All tests must pass; the prior crash path (ErrNotExist on phase-dir persona) must not regress
```

### Full suite regression [code]
```bash
cd "${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel)}/go"
go test ./internal/prompts/... ./internal/core/... ./internal/profiles/... ./internal/phaseregistrar/... \
  -timeout 120s 2>&1 | tail -20
```
