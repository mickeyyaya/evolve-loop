# Eval: declared-semantics-rejection

Tests that `fail_if_signal` in a phase spec without a Stage-3 signal bus triggers a
loud authoring-time FAIL (not a silent WARN). The specrunner must reject the artifact
with a named violation rather than silently continuing.

## Code Graders (bash commands that must exit 0)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/specrunner/... -run TestFailIfSignal_WithoutSignalBus_ReturnsFail -v 2>&1 | grep "PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/specrunner/... -run TestFailIfSignal 2>&1 | grep -v "FAIL"`
- `[code]` `grep -n "\"error\"\|VerdictFAIL" /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/specrunner/specrunner.go | grep -i "fail_if_signal\|FailIfSignal" | grep -c "." | awk '{if($1>=1) exit 0; else exit 1}'`

## Regression Evals (full test suite)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/specrunner/... 2>&1 | tail -3 | grep "^ok"`

## Acceptance Checks

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go build ./... 2>&1 | wc -c | awk '{if($1==0) exit 0; else exit 1}'`

## Negative Graders (gaming check — silent WARN must NOT pass)

- `[code]` `grep -n '"warning"' /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/specrunner/specrunner.go | grep -i "fail_if_signal" | wc -l | awk '{if($1==0) exit 0; exit 1}'`
