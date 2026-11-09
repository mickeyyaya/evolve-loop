# Build Explanation — Cycle 2237

## Build Binding
- Cycle: 2237
- Base SHA: 0208b0658f294f08d344c3efa67dcf03f4c57d78

## Summary
The tmuxtest package now records the process identifier of each started tmux server. StopServer and the orphan guard stop the server process directly when socket communication fails.

## Rationale
When a test cleanup removes the temporary socket directory, tmux socket commands fail. The server continues to run with an unlinked socket. Recording the process identifier allows direct process stop without socket dependency.

## Changed Areas
- `go/internal/tmuxtest/tmuxtest.go` — addresses the socket with `-L socket` instead of `bridge.TmuxSocketArgs`, records the server PID, signals it on stop, and updates the guard.
- `go/internal/tmuxtest/tmuxtest_orphan_test.go` — adds regression tests for server stop and orphan guard teardown when the socket directory is removed.
- `go/internal/tmuxtest/tmux_package_test.go` — skips the acs directory during test package scans so acceptance test fixtures do not trigger the orphan guard check.
- `go/internal/repocontract/testhelper_imports_test.go` — skips the acs directory during importer scans so acceptance test helpers do not trigger production import checks.

## Design Decisions
The implementation stores server process identifiers in an internal map protected by a mutex. Process stop attempts socket shutdown first, then falls back to direct process signals. The orphan guard shell script includes the server process identifier in its teardown command.

## Verification
Unit tests in tmuxtest verify server stop when the socket directory is removed. ACS predicates TestC2237_001, TestC2237_002, and TestC2237_003 pass with zero failures.

## Compatibility
Existing callers of StartServer, StopServer, and Main maintain identical signatures and behavior. Sockets with intact directories continue to use the standard tmux kill-server path.

## Limitations
Direct process signaling applies only to tmux servers that the current process starts through tmuxtest.
