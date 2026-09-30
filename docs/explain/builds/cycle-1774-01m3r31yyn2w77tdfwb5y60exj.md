# Build Explanation — Cycle 1774

## Build Binding
- Cycle: 1774
- Base SHA: 7207ba8d232e8a0be61766ad144a28cff3783568

## Summary
The `go/cmd/evolve` test suite no longer touches the host's tmux socket directory. The package's `TestMain` now points `TMUX_TMPDIR` at a private directory for the whole test run. The loop's startup and post-cycle orphan-socket sweeps therefore list and unlink `evolve-bridge-p<pid>` files only in that private directory, never in `/tmp/tmux-<uid>/`.

## Rationale
`gcOrphanSessions` is reached from `runLoopBatch`'s startup sweep and from `reapCycleSessions`' post-cycle sweep. The tdd phase measured 65 distinct tests reaching it. Thirteen of them never call `installStubDeps`, so the planned per-helper `t.Setenv` fix left the host dir exposed. One package-wide redirect in the existing `TestMain` covers every current and future caller with a single edit. It changes only test code.

## Changed Areas
- `go/cmd/evolve/main_test.go` — `TestMain` creates a short `/tmp/evtmux*` directory, sets `TMUX_TMPDIR` to it before `m.Run()`, and removes it afterwards. This isolates every test's orphan-socket sweep from the host's tmux sockets.
- `go/cmd/evolve/cmd_loop_orphan_socket_gc_test.go` — this is the tdd phase's in-package guard, committed unchanged. A real batch must still reap a dead-pid socket and spare a live-pid one in the directory that `TMUX_TMPDIR` names, so the isolation redirects the sweep instead of disabling it.
- `go/acs/cycle1774/predicates_test.go` — these are the tdd phase's acceptance predicates, committed unchanged. A canary socket must survive the whole `./cmd/evolve` suite while the 18 sweep-reaching tests pass, and the in-package guard must stay green.
- `.evolve/evals/loop-batch-tests-reap-host-tmux-sockets.md` — the tdd phase's eval for this task, committed unchanged. Its score caps point at the two predicates above.

## Design Decisions
The redirect lives in `TestMain`, not in `installStubDeps`, because 13 sweep-reaching tests never call that helper. The directory is created under `/tmp`, not `$TMPDIR`: on macOS `$TMPDIR` is a long `/var/folders/...` path, and a real tmux `-L` socket beneath it could exceed the 104-byte `AF_UNIX` path limit. Tests that set their own `TMUX_TMPDIR` with `t.Setenv` still override it and restore the package value afterwards. Production code (`cmd_loop.go`, `cmd_loop_control.go`, `internal/swarm`) is untouched.

## Verification
- `go test -tags acs -count=1 -v ./acs/cycle1774`: both predicates pass. `TestC1774_001` was RED at base.
- `go test -count=1 ./cmd/evolve/ ./internal/swarm/...`: both packages `ok`.
- `evolve acs suite --cycle 1774`: `red=0`.
- After the runs, no `/tmp/evtmux*` directories remain.

## Compatibility
Only test code changed. Production behavior is the same: the sweep still reads `TMUX_TMPDIR` and falls back to `/tmp` when it is unset.

## Limitations
The isolation covers only the `go/cmd/evolve` package. Other packages that start the loop's sweep would need the same redirect. The tdd phase's measurement found no such package.
