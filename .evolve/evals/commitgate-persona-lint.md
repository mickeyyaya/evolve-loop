# Eval: commitgate-persona-lint

Tests that the commit gate for `--class cycle` runs persona lint (phasecoherence.Check +
CheckArtifactNames) and blocks on violations, while a clean personas+profiles tree passes.

## Code Graders (bash commands that must exit 0)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/ship/... -run TestCommitGate_CyclePersonaLint -v 2>&1 | grep "PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/ship/... -run TestPersonaLint_ViolationBlocks -v 2>&1 | grep "PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/ship/... -run TestPersonaLint_CleanTreePasses -v 2>&1 | grep "PASS"`
- `[code]` `grep -r "runPersonaLint\|PersonaLint\|phasecoherence" /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/ship/ --include="*.go" -l | grep -c "." | awk '{if($1>=1) exit 0; else exit 1}'`

## Regression Evals (full test suite)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/ship/... 2>&1 | tail -3 | grep "^ok"`

## Acceptance Checks

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go build ./... 2>&1 | wc -c | awk '{if($1==0) exit 0; else exit 1}'`

## Negative Graders (gaming check — violations must block, not warn)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/ship/... -run TestPersonaLint_ViolationBlocks -v 2>&1 | grep -v "FAIL"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/ship/... -run TestPersonaLint_BypassEnvSkipsLint -v 2>&1 | grep "PASS"`
