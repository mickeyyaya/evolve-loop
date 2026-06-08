# Evolve Loop Runner
1. Verify clean worktree state (`git status`); check `.evolve/cycle-state.json` for an unfinished cycle (an unfinished cycle has `cycle_id > lastCycleNumber` in `.evolve/state.json` — resume with `evolve loop --resume` or seal with `evolve cycle reset`).
2. Launch the loop in the background with logging — the canonical runner is the Go binary (the old `./dispatch.sh` was removed in the v12 Go migration):
   `go/evolve loop --goal-text "<goal>" --cycles <N> --batch-cap-usd <cap> > evolve.log 2>&1 &`
   Decide `<N>`/`<cap>` yourself (don't put run-size to a vote). A pre-batch readiness gate runs first (`EVOLVE_SKIP_PREFLIGHT[_BOOT]=1` to bypass).
3. Tail `evolve.log` for ~30s; confirm cycle 1's scout phase boots and progresses. A full cycle runs several minutes, so report live status rather than blocking on completion.
4. Report PID, log path, and first-cycle progress/metrics.
