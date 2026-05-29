# Incident: cycle-141 — builder wrote `build-report.md` to the worktree cwd (driver never polled it) → `exit 81` artifact timeout → converged work discarded

> **Window:** 2026-05-29 · **Status:** ROOT CAUSE VERIFIED + FIXED + SHIPPED (`e5883ab`, CI green). · **Severity:** HIGH — a fully converged, committed build (`3580efc`) was thrown away over an artifact-path mismatch; the cycle-138→140 EGPS fix could not be validated end-to-end because the cycle died two phases before audit.
> Companion to the [cycle-138→140 EGPS-verdict incident](cycle-138-140-egps-verdict-not-generated-in-autonomous-loop.md) (cycle-141 was its validation cycle) and the [regression coverage index](REGRESSION-COVERAGE-INDEX.md). Reopens the same class as the cycle-108 `workspace/`-subdir fallback.

## 1. Executive summary

| Field | Value |
|---|---|
| **Symptom** | cycle-141 build phase reported `bridge: launch exit=81: core: bridge artifact timeout`, `FinalVerdict=SKIPPED`. No `build-report.md` in the workspace; no `build-stdout.log`; the cycle-141 worktree was torn down. A coincident `stall_no_output` observer event at the same 600s mark made it *look* like a stall-kill. |
| **Root cause (primary)** | The builder runs with **cwd = the worktree** (`driver_tmux_repl.go:77`), and the build prompt names the artifact by **bare relative path** ("Write `build-report.md`") — the `$ARTIFACT_PATH` token never survives into the resolved prompt. So the agent wrote `build-report.md` into the **worktree**, but `artifactReady` (`driver_common.go`) polled only the canonical `<workspace>/build-report.md` and the single fallback `<workspace>/workspace/<base>`. The worktree was never searched → the artifact was never "ready" → the artifact-wait `StopReviewer` paused at the 2nd 300s interval (pane gone static after the agent idled) → `ExitArtifactTimeout` (81). The work survived only on branch `cycle-141` (`3580efc`); the uncommitted `build-report.md` died with the worktree. |
| **Root cause (secondary, latent)** | The auto-spawn observer (ADR-0030) stall check watched **only** `<phase>-stdout.log` growth. For `*-tmux` drivers, live output goes to the **tmux scrollback** and reaches the stdout-log only on clean exit, so a working tmux agent's stdout-log stays flat → false `stall_no_output`. This logged a misleading incident but did **not** cause the kill (the observer only emits events; it does not SIGTERM — the `context canceled` it recorded came *after* the bridge already returned 81). |
| **Why it looked like a stall** | Both the bridge artifact-wait deadline and the observer stall threshold default to 600s and both start at launch, so they fired at the same wall-clock instant (12:04:38). The loud `stall_no_output` line drew the eye away from the real `exit 81` artifact-wait failure. |
| **Fix** | (1) `artifactReady` now also searches `<worktree>/<base>` and `<worktree>/workspace/<base>` and relocates to canonical (mirrors the cycle-108 fallback, extended to the builder's real cwd). (2) The observer treats a fresh write anywhere under `Config.WorkspaceDir` as progress, so tmux agents are no longer falsely reported as stalled; its own events sink is excluded so it can never mask a real stall. |

## 2. Evidence (from cycle-141 forensics)

- `cycle-141/build-observer-events.ndjson`: `started` @ 03:54:38Z → `stall_no_output "no stdout growth for 10m0s"` @ 04:04:38Z → `stopped "context canceled"` @ 04:04:48Z.
- `cycle-141/workspace/build-reflection.yaml`: `convergence_verdict: converged`, "5-line diff", "100% coverage on first run" — written ~11:57 local, **3 minutes after launch**. The agent *finished*.
- `git log cycle-141`: `3580efc fix(phaseorder): defensive copy + registry deduplication [worktree-build]`, parent `fbf9ebb`, authored 11:56:15 — 8 lines in `phaseorder.go` + 73 lines of new tests. The build was committed.
- `build-report.md`: **absent** at the canonical path AND at `<workspace>/workspace/` (where `build-reflection.yaml` *did* land). Never in commit `3580efc`. → the agent's `build-report.md` write went somewhere neither polled location covered (the worktree cwd), or was the next action after the reflection when the wait gave up.
- `build-stdout.log`: **never created** — a symptom of the tmux scrollback never draining (the run was abandoned before clean exit), not the cause.

## 3. The mechanism

```
runTmuxREPL: workingDir = cfg.Worktree           # agent cwd = .evolve/worktrees/cycle-141
prompt: "Write build-report.md"                  # bare relative name; no $ARTIFACT_PATH
agent writes  <worktree>/build-report.md         # relative to cwd → into the worktree
artifactReady polls:
    <workspace>/build-report.md                  # canonical — absent
    <workspace>/workspace/build-report.md        # cycle-108 fallback — absent
                                                  # <worktree>/... NEVER CHECKED
StopReviewer @ interval 2 (600s): pane static (agent idle) → ReviewPause → break
→ ExitArtifactTimeout (81) → FinalVerdict=SKIPPED → worktree torn down → report lost
```

The observer's `stall_no_output` fired in parallel on the flat stdout-log; it is a logger, not a killer.

## 4. The fix (`e5883ab`)

**Primary — `go/internal/bridge/driver_common.go` `artifactReady`:** ordered fallback search, first non-empty wins, relocate to canonical:
1. `<workspace>/workspace/<base>` (cycle-108)
2. `<worktree>/<base>` (cycle-141 — the builder cwd)
3. `<worktree>/workspace/<base>` (the "workspace/" literal-subdir misread, relative to cwd)

Worktree candidates are searched only when `cfg.Worktree != ""`, so headless drivers/probes are byte-identical to before.

**Secondary — `go/internal/adapters/observer/{observer.go,core_adapter.go}`:** new `Config.WorkspaceDir`; `newestActivity()` walks the workspace tree for the newest mtime (excluding the `-observer-events.ndjson` sink, capped at 500 files). The `Watch` loop resets the stall timer when **either** the stdout-log grows **or** the workspace mtime advances. `WorkspaceDir==""` ⇒ zero-time ⇒ stdout-log growth governs alone (pre-fix behavior).

**Tests:** `driver_artifact_relocate_test.go` (+4: worktree-root, worktree-workspace-subdir, workspace-subdir-wins-priority, no-worktree-unchanged); `observer_test.go` (+3: workspace-activity-resets, idle-still-stalls, events-file-does-not-mask). `go test` + `-race` green; reviewed by `code-simplifier` + `go-reviewer` (0 CRITICAL/HIGH).

## 5. Lessons

1. **The loud symptom is not always the cause.** A `stall_no_output` INCIDENT line outshouted the real `exit 81` artifact-wait failure. Always trace the exit code to its emitter (`ExitArtifactTimeout` ≠ observer) before fixing.
2. **A converged, committed build can still be discarded.** Quality was perfect (`3580efc`); the pipeline lost it purely on artifact *location*. Artifact-path tolerance is load-bearing, not cosmetic.
3. **cwd assumptions drift.** The cycle-108 fallback assumed cwd≈workspace; the builder's cwd is actually the worktree, so the same class reopened. When a phase changes its cwd, every cwd-relative path contract must be re-derived.
4. **Same-default deadlines collide.** Two independent 600s timers starting at launch fired at the same instant, conflating two mechanisms. Distinct, staggered, or causally-linked deadlines would have made the diagnosis obvious.

## 6. References

- Fix commit: `e5883ab` (CI green: `CI` + `go` workflows).
- Lost-but-recoverable work: branch `cycle-141` @ `3580efc` (phaseorder defensive copy — a legitimate improvement; can be cherry-picked or re-derived by a fresh cycle).
- Prior art: cycle-108 `workspace/`-subdir fallback (`driver_artifact_relocate_test.go` original cases); ADR-0030 observer auto-spawn; ADR-0026 artifact-wait-reviews-before-kill (`stopreview.go`); ADR-0024 (artifactReady Step 0).
- Code: `go/internal/bridge/driver_common.go`, `go/internal/adapters/observer/observer.go`, `go/internal/adapters/observer/core_adapter.go`.
