# Option 3 — Move the check into core, ahead of ship

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

The orchestrator probes the lane after normalize and before it dispatches ship. A diverged lane goes to
the rebase path with its staged edits intact, so ship never sees the shape (E2). The console owns the
core files.

## Causal chain to the goal metric

Core already holds the base SHA and the lane worktree, so the probe needs no ship side effect (E2, E4).
The scan is skipped by construction for a lane that main has passed.

## Quantified expected effect

One scan saved per diverged lane, with the same size uncertainty as Option 2 (A2). The ship code stays
unchanged.

## Cost and time-to-effect

A new core seam plus a reachability test through the loop entry point. More than one console cycle.

## Top risks and early detection

1. The rebase path is the recovery that discards a HEAD == base lane (E3), so the saving needs the
   staged-content classifier of Option 2 (A1), and two classifiers (core and recovery) can disagree. *Detection:* one shared function and a table test.
2. The seam covers the sequential loop but not fleet mode. *Detection:* a fleet-mode reachability test.
