# Eval: user-phase-persona-resolution

## Code Graders (bash commands that must exit 0)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/specrunner/... -run . -count=1 2>&1 | grep -E "^ok|PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phaseregistrar/... -run . -count=1 2>&1 | grep -E "^ok|PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestDispatchability -count=1 2>&1 | grep -E "^ok|PASS|--- PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/prompts/... -run TestAgentPhaseDirFallback -count=1 2>&1 | grep -E "^ok|PASS|--- PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/profiles/... -run TestProfileFallback -count=1 2>&1 | grep -E "^ok|PASS|--- PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./cmd/evolve/... -run TestPhaseLintNamingTwoTier -count=1 2>&1 | grep -E "^ok|PASS|--- PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestPhaseSkippedLedgerSource -count=1 2>&1 | grep -E "^ok|PASS|--- PASS"`

## Regression Evals

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./... 2>&1 | grep -E "^(ok|FAIL)" | grep -v "^ok" | wc -l | xargs -I{} test {} -eq 0`

## Acceptance Checks

- `[code]` `grep -n "PhaseDir\|phasedir\|evolve/phases.*agent.md\|agent.md.*phases" /Users/danleemh/ai/claude/evolve-loop/go/internal/prompts/prompts.go | wc -l | xargs -I{} test {} -gt 0`
- `[code]` `grep -rn "ClampDispatchable\|dispatchability\|plan.time.*clamp\|clampDispatch" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go | wc -l | xargs -I{} test {} -gt 0`
- `[code]` `grep -rn "SkipSource\|skip_source" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/ports.go | wc -l | xargs -I{} test {} -gt 0`
- `[code]` `grep -rn "two.tier\|single.word\|^[a-z].*(-[a-z]\|naming" /Users/danleemh/ai/claude/evolve-loop/go/cmd/evolve/cmd_phase_lint.go | wc -l | xargs -I{} test {} -gt 0`

## Negative Cases (must fail with non-zero exit or emit expected error)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestDispatchabilityClamp_PersonaMissing -count=1 2>&1 | grep -E "PASS|^ok"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestDispatchabilityClamp_RunnerMissing -count=1 2>&1 | grep -E "PASS|^ok"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestDispatchabilityClamp_ProfileMissing -count=1 2>&1 | grep -E "PASS|^ok"`

## Thresholds
- All checks: pass@1 = 1.0
