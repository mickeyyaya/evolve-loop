# Build Explanation — Cycle 2644

## Build Binding
- Cycle: 2644
- Base SHA: 71a88079e78510116fdc348f01aa71f51aaa9464

## Summary
A boot-time binary refresh now arms the wave handoff before `syscall.Exec`, and refuses to rebuild from a plane `HEAD` that `origin/main` does not hold.

## Rationale
`bootBinaryRefresh` replaces the process image in place with the same PID. The new image reads `ResumeWaves=0` unless the boundary-refresh marker records the wave count first. Checking `origin/main` before the rebuild stops the loop from compiling unshipped local commits into the shared plane binary.

## Changed Areas
- `go/internal/loopchain/marker.go` — exports `ArmHandoff(attemptPath, Claim, wavesDone)` and extracts `writeAttempt` so `ArmHandoff` and `Refresher.recordAttempt` share one JSON marker writer.
- `go/cmd/evolve/cmd_loop_boot_refresh.go` — adds `bootRefreshHeadShippedFn` to refuse rebuilds when `origin` exists and `HEAD` is ahead of `origin/main`. It calls `loopchain.ArmHandoff` before `beforeReExec` and `bootRefreshExecFn`.

## Design Decisions
`ArmHandoff` takes `loopchain.Claim` to match `TakeHandoff`. It writes `RunningCommit` from the pre-exec binary commit so `handsOffTo` distinguishes the old image from the new one. When `cfg.ResumeWaves == 0`, `bootBinaryRefresh` skips the marker write. When `ArmHandoff` fails, it returns before `beforeReExec` and `bootRefreshExecFn` so the old image boots as-is. `defaultBootRefreshHeadShipped` checks `git remote get-url origin` first so synthetic test directories without `origin` proceed. Repositories with `origin` require `git merge-base --is-ancestor <head> origin/main` to exit 0.

## Verification
`gofmt -l`, `go vet ./internal/loopchain/... ./cmd/evolve/...`, `go test -cover -count=1 ./internal/loopchain/`, `go test -count=1 ./cmd/evolve` boot-refresh and handoff tests, `go test -tags acs -count=1 ./acs/cycle2644`, and `evolve acs suite --cycle 2644` verify the change.

## Compatibility
`TakeHandoff`, `Refresher.Refresh`, and `boundary-refresh-attempt.json` keep their existing schema and behavior. Callers with `ResumeWaves == 0` or test fixtures without an `origin` remote behave identically to prior builds.

## Limitations
The shipped check tests ancestry against `origin/main` using the plane checkout's remote-tracking ref and does not run `git fetch` itself.
