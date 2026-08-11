# Eval: builder-trajectory-compression
## Code Graders (bash commands that must exit 0)
- `[code]` `go test -v github.com/mickeyyaya/evolve-loop/go/internal/phases/build -run TestBuild_TrajectoryCompression`
## Regression Evals (full test suite)
- `[code]` `go test ./internal/...`
## Acceptance Checks
- `[code]` `go test -v github.com/mickeyyaya/evolve-loop/go/internal/phases/build -run TestBuild_TrajectoryCompression`
## Thresholds
- All checks: pass@1 = 1.0
