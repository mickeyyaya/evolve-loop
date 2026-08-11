# Eval: tmux socket contract coverage
## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go test ./internal/bridge ./internal/swarm -run 'TestTmuxSocketArgs|TestExecTmuxKill' -count=1`
## Regression Evals (full test suite)
- `[code]` `cd go && go test ./internal/bridge ./internal/swarm -count=1`
## Acceptance Checks
- `[code]` `cd go && EVOLVE_TMUX_SOCKET=evolve-bridge-p999 go test ./internal/bridge -run TestTmuxSocketArgs_PerRunOverride -count=1`
- `[code]` `cd go && go test ./internal/swarm -run TestExecTmuxKill_EmptySessionRefusedBeforeExec -count=1`
## Negative Cases
- `[code]` `cd go && go test ./internal/swarm -run TestExecTmuxKill_EmptySessionRefusedBeforeExec -count=1`
## Edge Cases
- `[code]` `cd go && EVOLVE_TMUX_SOCKET= go test ./internal/bridge -run TestTmuxSocketArgs_EmptyStillSelectsSocket -count=1`
## Thresholds
- All checks: pass@1 = 1.0
