# Build Explanation — Cycle 2625

## Build Binding
- Cycle: 2625
- Base SHA: e854f84dd40b11daed964c5a3d0665d14c709408

## Summary
The orphan socket reaper now stops the tmux server of a dead test binary. A test binary registers its socket directory, and the reaper lists the registered directories.

## Rationale
A test binary runs its tmux server in its own `TMUX_TMPDIR`. The reaper read only the current directory, so 282 servers outlived their test binaries on 2026-11-20. A registry file per directory fixes this with no new setting.

## Changed Areas
- `go/internal/swarm/test_socket_dirs.go` — adds the registry: `RegisterTestSocketDir` writes one file per directory, and the listing prunes an entry whose directory is gone.
- `go/internal/swarm/reap_orphans.go` — lists the sockets of each registered directory, reads the owner pid from the base name, and stops a full-path socket with `tmux -S`.
- `go/internal/tmuxtest/tmuxtest.go` — `Main` registers its socket directory, and the test-socket filter reads the base name.
- `go/internal/swarm/test_socket_dirs_test.go` — covers registration, idempotence and pruning.
- `go/internal/swarm/reap_socket_file_test.go` — isolates `TMPDIR` so the real registry does not enter the test.
- `go/internal/swarm/reap_test_sockets_test.go` — isolates `TMPDIR` for the same reason.
- `go/internal/tmuxtest/tmuxtest_test.go` — isolates `TMPDIR` in the reap test for the same reason.
- `docs/architecture/packages/internal-swarm.md` — records the registry design.

## Design Decisions
The registry sits in `os.TempDir()`, so a reaper in another process finds it. A live-owner socket stays, because the owner-pid rule is unchanged. The existing killer seam stays, so a test never signals a real process.

## Verification
The ACS predicates `TestC2625_001` to `TestC2625_004` pass. The `internal/swarm` and `internal/tmuxtest` suites pass.

## Compatibility
Sockets of the current directory keep their bare names. Only registered directories return full paths.

## Limitations
A directory that exists but holds no sockets stays registered until it is removed.
