# Build Explanation — Cycle 2218

## Build Binding
- Cycle: 2218
- Base SHA: 9c45731d429d7553fbd4f1bbf7ad443d879760c1

## Summary
The probe sessions of recipe_adapter and capture_models now export EVOLVE_DISPATCH_ID and EVOLVE_PROJECT_ROOT to their panes. Recipe adapter sessions install a dispatch sweep, and ephemeral sessions reap processes upon completion. The launch_model_verification path needed no code change because launchSwept already tags it.

## Rationale
Probe sessions previously bypassed pane shell priming, so child processes lacked dispatch tags. When a probe leaked, the garbage collector cannot identify the associated dispatch or find a persisted process tree. The primePaneShell function exports the dispatch ID and project root before the CLI command starts. A dispatch sweep on the session driver records the process tree and persists it for garbage collection before the tmux session is killed.

## Changed Areas
- `go/internal/bridge/capture_models.go` — reaps the ephemeral model picker probe session through reapEphemeralSession.
- `go/internal/bridge/recipe_adapter.go` — primes the probe pane shell with primePaneShell and creates a dispatch sweep to record and reap probe processes.

## Design Decisions
Reusing primePaneShell instead of writing manual pane initialization ensures identical environment variables across all tmux launches. Using reapEphemeralSession in capture_models removes duplicate session cleanup code.

## Verification
Tests in probe_dispatch_tag_test.go verify that pane shells are tagged before kill-session and that leaked probe processes are reaped by garbage collection. All five cycle-2218 ACS predicates pass.

## Compatibility
Public function signatures remain unchanged. EnsureSession now returns an error when pane shell detach fails, and newRecipeDriver mints a dispatch ID into the caller's Config when empty.

## Limitations
Ephemeral probe sessions do not clean up if the parent process receives an immediate SIGKILL; the persisted tree allows garbage collection to reap survivors.
