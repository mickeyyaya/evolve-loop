# Evolve Loop Runner
1. Verify clean worktree state (git status, check for stale checkpoints)
2. Launch dispatcher with explicit log file: `./dispatch.sh --log evolve.log &`
3. Tail log for 30s; confirm cycle 1 completes before returning
4. Report PID, log path, and first-cycle metrics
