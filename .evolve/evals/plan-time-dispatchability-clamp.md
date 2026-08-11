# Eval: plan-time-dispatchability-clamp

## Goal
Validate that the plan-time dispatchability clamp prevents all four documented drift modes from crashing mid-pipeline.

## Acceptance Criteria

### AC-1: Phase-dir persona resolution — agents/ fallback [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/prompts/... -run TestAgent_PhaseDir -v
```
Expected: PASS. A phase whose persona lives at `.evolve/phases/<name>/agent.md` resolves correctly; `agents/evolve-<name>.md` absence is not fatal.

### AC-2: Plan-time clamp drops undispatchable phase with WARN [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestClampPlan_UndispatchableDroppedWithWarn -v
```
Expected: PASS. A plan containing a phase whose persona+runner+profile tuple fails the dispatchability predicate is dropped with a WARN, not a panic or a hard FAIL that aborts the cycle.

### AC-NEGATIVE: Mid-pipeline crash eliminated (Mode 2 regression) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestNoCrash_PersonalessPhaseInPlan -v
```
Expected: PASS. Simulates a plan that includes a spec-verify-style phase with no registered runner at boot; the orchestrator must survive with phase_skipped, not an `invalid phase: no runner registered` panic.

### AC-3: Profile fallback name→role→archetype-default [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/profiles/... -run TestProfileFallback_NameToArchetype -v
```
Expected: PASS. Profile resolution for a phase name with no `.evolve/profiles/<name>.json` falls back to role→archetype-default without exit=10.

### AC-4: Contract-vs-persona artifact-name consistency check [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phaseregistrar/... -run TestContractArtifactConsistency -v
```
Expected: PASS. A persona that references a hardcoded artifact name diverging from the phasecontract's declared output is flagged at plan-time clamp; the specific case `plan-review.md` vs `plan-review-report.md` must be detectable.

### AC-5: Two-tier naming lint enforced in phases validate [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./... -run TestTwoTierNamingLint -v
```
Expected: PASS. A user/minted phase named with a single word (e.g. `"reviewer"`) is rejected by `evolve phases validate`; grandfathered builtins `tester` and `build-planner` emit WARN-only.

### AC-EDGE: Empty plan after clamp emits human-readable warning [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestClampPlan_AllDropped_WarnEmitted -v
```
Expected: PASS. If the clamp drops all advisor-selected phases, the orchestrator emits a structured warning and falls back to the static state machine rather than producing a zero-phase cycle silently.

### AC-FULL-BUILD: No regressions [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./... 2>&1 | tail -5
```
Expected: All packages PASS, no new failures.
