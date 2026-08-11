# Eval: Assert Realizer LaunchFlags Bypass

## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go test -v -run=TestRealizeFor_RealManifests_NoCrossCLILeak ./internal/bridge/...`

## Regression Evals (full test suite)
- `[code]` `cd go && go test -run=TestRealizeFor ./internal/bridge/...`

## Acceptance Checks
- `[code]` `grep -q 't.Run("claude-tmux"' go/internal/bridge/realizer_realmanifest_test.go`
- `[code]` `grep -q 't.Run("agy-tmux"' go/internal/bridge/realizer_realmanifest_test.go`
- `[code]` `grep -q 't.Run("codex-tmux"' go/internal/bridge/realizer_realmanifest_test.go`
- `[code]` `grep -q 't.Run("ollama-tmux"' go/internal/bridge/realizer_realmanifest_test.go`

## Thresholds
- All checks: pass@1 = 1.0
