# Plan: proctree reads the process table from the kernel and not from ps

- **Status:** approved, built 2026-11-21.
- **Design set:** this file is the design of record. It extends [dispatch-process-hygiene-2026-10.md](dispatch-process-hygiene-2026-10.md) (decision D2) and the package note [internal-proctree.md](../architecture/packages/internal-proctree.md).
- **Inbox items:** `proctree-reads-processes-without-ps` and `faketmux-reaper-needs-no-ps-binary`. One build PR consumes both items. It carries one `Closes-Inbox` trailer for each item.
- **Research:** the facts below. No separate dossier is needed.

## 1. Request

The console builder run `agent-console-builder-20261119T125612Z-7945` reported this blocker on 2026-11-19:

> "A lane cell denies ps ('fork/exec /bin/ps: operation not permitted'), so faketmux (session.go:31 commandOf, format.go:80 runPS) and every caller of the proctree listers fail inside a cell. 8 faketmux tests are red in every lane cell."

The operator asked on 2026-11-21: "Full speed to consume and finish all the features". This plan is the design step that the protected path `go/internal/proctree` needs before the build.

## Terms used throughout

- **Lane cell:** the sandbox in which a build lane runs. On macOS, it cannot exec the setuid `ps` binary (section 3).
- **Lister:** the function type `proctree.Lister`. It returns the process table of the current user.
- **Kernel reader:** code that reads the process table from `sysctl` (macOS) or from `/proc` (Linux). It starts no program.
- **Parity:** a kernel reader gives the same pid, ppid, pgid, comm and start second as `ps` for the same process.

## 2. Facts

| Id | Fact | Source |
|---|---|---|
| F1 | `NewLister` runs `ps -A -o pid=,ppid=,pgid=,uid=,lstart=,comm=` through `sysexec`. `ExecLister` wraps it. | `go/internal/proctree/list.go:14-44` |
| F2 | The per-pid readers `readProcArgs` (darwin and linux) are unexported. `ArgsReader` is an exported type with no exported implementation. | `go/internal/proctree/procargs_darwin.go:17`, `procargs_linux.go:10`, `list.go:12` |
| F3 | `StartOf` and `StartTime` already read the kernel and run no program. The package note says that a sandbox that denies `ps` does not affect them (cycles 2122 and 2133). | `go/internal/proctree/procstart.go:5-11`, `docs/architecture/packages/internal-proctree.md` (section "Design") |
| F4 | Two production callers use `ExecLister`: `gcProcesses` and `Deps.withProcessDefaults`. A third path reaches the lister through `gcProcesses().list`. One consumer of `Comm` exists: the tail rule compares the base of `Comm` with the base of `Args[0]`. These two values become equal on darwin under D6, so its behavior does not change. | `go/cmd/evolve/cmd_gc_dispatch.go:32`, `go/internal/bridge/dispatch_reap.go:243`, `go/cmd/evolve/cmd_loop_lane_reap.go:28`, `go/internal/gc/dispatch_processes.go:55` |
| F5 | `faketmux` runs `ps` in two places. `commandOf` runs `ps -o comm= -p <pid>`. `runPS` runs `ps -A -o pid= -o ppid= -o comm=`. | `go/internal/faketmux/session.go:31-33`, `go/internal/faketmux/format.go:79-81` |
| F6 | `faketmux` already imports `proctree` (`processStart = proctree.StartOf`). It adds no new package edge. | `go/internal/faketmux/session.go:15,35` |
| F7 | One more production caller runs `ps` in the observer: `ps -o time= -p <pid>`. Another runs `lsof` in `ExecCwdLister`. Both are outside this item. | `go/internal/adapters/observer/cpu_probe.go:15`, `go/internal/proctree/cwd.go:24-32` |
| F8 | `Process.Started` is part of `Identity`. A map key compares it for equality. A persisted tree and a new listing must give the same value for one process. `ps lstart` has a resolution of one second. | `go/internal/proctree/proof.go:12-17,36-43`, `go/internal/proctree/table.go:19-22` |
| F9 | `go.mod` has two libraries (`rapid` and `yaml.v3`) and no `golang.org/x/sys`. A new library needs one adapter home and a row in `thirdPartyHomes`. | `go/go.mod:5,7`, `docs/conventions/third-party-dependencies.md` |
| F10 | The darwin code already calls `sysctl` through `syscall.SYS___SYSCTL` and decodes `kinfo_proc` by byte offset. The record size is 648 bytes and `p_starttime` is at offset 0. | `go/internal/proctree/procargs_darwin.go:37-43`, `go/internal/proctree/procstart_darwin.go:12-16,42-52` |
| F11 | The Go `x/sys/unix` module wraps `kern.proc.all` as `SysctlKinfoProcSlice`. A caller of the raw call must size the buffer with a first call and read each field at a fixed offset. | [gopsutil process_darwin.go](https://github.com/shirou/gopsutil/blob/master/process/process_darwin.go), [Apple QA1123](https://developer.apple.com/library/archive/qa/qa2001/qa1123.html) |
| F12 | On Linux, the `comm` field of `/proc/<pid>/stat` can hold spaces and `)`. A reader must find the last `)`. A split on spaces moves the later columns. | [proc_pid_stat(5)](https://manpages.ubuntu.com/manpages/noble/man5/proc_pid_stat.5.html), [gopsutil PR 2181](https://github.com/shirou/gopsutil/pull/2181) |
| F13 | The package already parses stat after the last `)` (`startTicks`) and computes the boot time (`bootTime`). | `go/internal/proctree/procstart_linux.go:46-52,82-99` |
| F14 | A second `kinfo_proc` decoder exists in a test file of `bridge`. It declares the offsets comm=243, uid=420, ppid=560 and pgid=564, and `darwinSysctlListProcs` decodes `KERN_PROC_ALL`. `fallbackLister`, `darwinProcArgsListing` and `psArgsOrFallback` fall back to it when `ps` fails. The non-darwin twin runs `ps -axww -o args`. The caller of `psArgsOrFallback` is the hermetic secret test of `bridge`. | `go/internal/bridge/dispatch_busyloop_reap_darwin_test.go:18-108`, `go/internal/bridge/dispatch_busyloop_reap_other_test.go:11-17`, `go/internal/bridge/tmux_hermetic_env_integration_test.go:47` |
| F20 | Probe of the round-2 reviewer on this host, with the cell preamble of `sandbox.go:204-214` under `sandbox-exec`: the exec of `/bin/ps` gives `EPERM`. `KERN_PROC_UID` returned 1234 rows. The offsets 40, 560 and 564 gave the right pid, ppid and pgid for the process itself, a child and the parent. `KERN_PROCARGS2` succeeded for 1234 of 1234 rows, including a renamed child (`argv[0]` `renamedproc`) and a parent outside the sandbox. | Reviewer probe, 2026-11-21 |
| F21 | The darwin set generator lists a package only when its darwin and linux file sets differ, or when a file branches on `runtime.GOOS`. A hand edit of `.darwin-set` makes `SyncDarwinSet` return `ErrDarwinSetStale`. After D10, `faketmux` has none of these traits. | `go/internal/ciparity/darwin_set.go:74-127,220-240` |
| F22 | `LiteralFiles` matches with `strings.Contains`, so the needle `ps` matches `steps` or `https`. The precedent for a program name is `MCPRegistrars`: `runsAProcess` together with `hasLiteralEqual`. | `go/test/structure/binary.go:25,43-49`, `go/test/structure/mcp.go:23-26` |
| F15 | On macOS, `ps -o comm=` prints `argv[0]`. It does not print the executable path of `KERN_PROCARGS2`. Check on macOS 27.2: after `bash -c 'exec -a renamedproc /bin/sleep 2'`, `ps -o comm=` prints `renamedproc` and `ps -o ucomm=` prints `sleep`. `faketmux` already expects this form, because `isPaneLeader` strips the login-shell `-` prefix. | Command output on this host, `go/internal/faketmux/session.go:130-137` |
| F16 | The uid filter has a pin today: `TestParsePS_KeepsOnlyTheRowsOfTheGivenUser`. Security row S3 and the invariant of the package note rest on it. On Linux, the owner of `/proc/<pid>` is root for a non-dumpable process, and `ps` prints the effective uid. The `Uid:` line of `/proc/<pid>/status` holds the real uid first and the effective uid second. | `go/internal/proctree/table_test.go:9`, `dispatch-process-hygiene-2026-10.md:88`, `internal-proctree.md:48`, proc(5) |
| F17 | The sandbox preamble allows `process-exec` and `sysctl-read`. `ExecDeny` lists only the LLM binaries. `/bin/ps` is setuid root, and macOS refuses the exec of a setuid binary from a process under Seatbelt. The Linux cell (`bwrap --unshare-pid --proc /proc`) can run `ps`, which is not setuid. | `go/internal/adapters/sandbox/sandbox.go:207,210,324-326`, `go/internal/bridge/no_nested_agency.go:59-76` |
| F18 | The structure package has the helpers `ExecImporters` and `LiteralFiles`, with a precedent test. | `go/test/structure/binary.go:15,25`, `go/test/structure/binary_test.go:55` |
| F19 | `.darwin-set` lists `./cmd/evolve`, `./internal/bridge` and `./internal/proctree`. It does not list `./internal/faketmux`. | `go/.darwin-set:6,12,22` |

*Implication:* the kernel already answers every question that `ps` answers. Two readers exist (F3, F13). The gap is the table scan and the export of the per-pid readers. F20 shows that the kernel route works in a cell, and F14 shows that the repo holds a second copy of the decoder (follow-up F-3).

## 3. Root cause

The package reads one process table through two routes. Start times and arguments come from the kernel. The pid, ppid, pgid, uid and comm come from a child program. The second route fails in a cell, and so the whole `Lister` fails (F1). `faketmux` copied the same route twice (F5). Each new caller of the table inherits the dependency on the `ps` binary.

The root cause is a design gap, not a cell policy. Evolve does not deny `ps` on purpose (F17). `/bin/ps` is setuid root, and macOS refuses a setuid exec from a Seatbelt process. So the failure happens on macOS only. The same cell allows `sysctl-read` (`sandbox.go:210`), and this is the evidence that the kernel route is allowed in a cell. The Linux cell can run `ps`, so the Linux `/proc` reader exists for D1 (one table source), not because a cell denies `ps`.

The process table must have one source in the package, so the fix makes the kernel that source.

## 4. Goals and non-goals

Goals:

- G1. `proctree` lists the table of the current user with no exec of any program, on macOS and Linux.
- G2. `proctree` exports one per-pid reader for the command name (`CommOf`). The reader of arguments stays unexported.
- G3. `faketmux` uses those readers and runs no program to read the process table.
- G4. Parity with `ps` for pid, ppid, pgid, comm and start second.
Non-goals:

- N0. The removal of the second decoder in `bridge` test code (F14). See follow-up F-3.

- N1. The `lsof` cwd lister (F7). It is a separate defect. See section 12.
- N2. The observer CPU probe (F7). See section 12.
- N3. A new platform. Other systems return an error, as they do today.

## 5. Candidate designs

| Id | Design |
|---|---|
| A | Allow `ps` in the cell. Change the cell policy. |
| B | Export the per-pid readers. Keep `ps` for the table. |
| C | Kernel table readers per OS behind the `Lister` seam. Export the per-pid readers. Remove the `ps` route. |
| D | Add `golang.org/x/sys/unix` and call `SysctlKinfoProcSlice` on darwin. |
| E | Add a host helper outside the cell. The cell asks it for the table. |

Scores: 2 is good, 1 is partial, 0 is a failure.

| Criterion | A | B | C | D | E |
|---|---|---|---|---|---|
| Acceptance 1: no exec of `ps` in a cell | 1 | 0 | 2 | 2 | 2 |
| Acceptance 2: parity with `ps` | 2 | 2 | 2 | 2 | 1 |
| Acceptance 3: `faketmux` passes in a cell | 1 | 0 | 2 | 2 | 1 |
| No feature flag, no control value from the environment | 2 | 2 | 2 | 2 | 1 |
| One home for each rule (one table source) | 1 | 0 | 2 | 2 | 1 |
| Protected surface stays small | 2 | 1 | 1 | 0 | 0 |
| CI parity on Linux and Go 1.23 without cgo | 2 | 2 | 2 | 1 | 1 |
| Third-party rule (F9) | 2 | 2 | 2 | 0 | 2 |
| Total (of 16) | 13 | 9 | 15 | 11 | 9 |

Why each design loses or wins:

- **A** cannot work as a policy change. The cell does not deny `ps` (F17). macOS refuses the setuid exec, and no sandbox rule changes that. A also leaves `ps` as a dependency of each caller.
- **B** fixes `commandOf` only. The reaper and the gc lister still need the table (F4). It does not meet acceptance 1.
- **D** removes the hand decode of the 648-byte record. It adds a library for one function and needs an adapter home plus a `thirdPartyHomes` row (F9). The package already decodes `kinfo_proc` by offset (F10), so the new code is small and local.
- **E** adds a process, a socket and a trust boundary to fix a read. The helper also reads the whole host.
- **C** wins. It meets the three criteria, adds no dependency, and gives the package one source for the table.

## 6. Decisions

| Id | Decision | Because | Rejected options |
|---|---|---|---|
| D1 | `proctree` has one table source: the kernel. The `ps` route is deleted: `NewLister`, `psArgs`, `parsePS`, `parsePSRow`. | F1, F3, F13. Two routes for one table can drift. | B keeps the `ps` route. A keeps the dependency. |
| D2 | `ExecLister` keeps its name and its signature `func() Lister`. In this codebase the prefix `Exec` means the real host implementation, as in `swarm.ExecPidAlive` and `ExecListBridgeSockets`. | Convention. The two production callers (F4) and the package notes do not change. T1 then fails on `origin/main` for the stated behavioral reason. | Rename to `KernelLister`: it breaks the convention, and T1 fails with a compile error. |
| D3 | Each OS has a function `readTable(uid int) ([]Process, error)` behind a build tag. `ExecLister` calls `newLister(readTable, readProcArgs, os.Getuid())`. The unexported `newLister(readTable, readArgs, uid)` is the seam for the tests of the error branch. It attaches the arguments with `withArgs`, which then calls the per-OS hook `commFrom(p Process) string` (D6). A row whose argument read fails stays in the table with an empty `Env`, so no tag proof can match it. | F2, F10. It follows the existing per-OS split of `procargs_*.go`. | One file with `runtime.GOOS`: the package note says the readers use build tags. |
| D4 | On darwin, the table comes from `sysctl` with the MIB `{CTL_KERN(1), KERN_PROC(14), KERN_PROC_UID(5), uid}`. The kernel filters by uid, so the code does not decode `cr_uid`. T10 pins the MIB. | F10, F11, F16. `KERN_PROC_UID` matches the effective uid, as `ps` does. | `KERN_PROC_ALL` with a decode of the uid at offset 420 (F14): one more offset, and the filter moves into the decoder. |
| D5 | The darwin decoder reads a record at the stride of 648 bytes. It reads `p_starttime` (offset 0), `p_pid` (offset 40), `p_comm` (offset 243), `e_ppid` (offset 560) and `e_pgid` (offset 564). The offsets are named constants with one layout test (T3). The decoder reuses `ctlKern`, `kernProc` and `kinfoProcBytes` from `procstart_darwin.go`. | F10, F14. The offsets of ppid and pgid come from the working in-repo decoder. | A new library (D candidate): F9. |
| D6 | Each OS has a hook `commFrom(p Process) string`. Darwin gives `Args[0]`, with `p_comm` as the fallback. Linux gives the `stat` comm. `withArgs` and `CommOf` both call it. Detail: "Detail of D6" below. | F15. This is what `ps` does. | The executable path of `KERN_PROCARGS2`: it gives `/bin/sleep` where `ps` gives `renamedproc`. A second `KERN_PROCARGS2` read for each pid. |
| D7 | On linux, `readTable` scans the numeric entries of `/proc`. It uses one stat parser and the effective uid of the `Uid:` line. It skips a pid on `ENOENT`, `ESRCH`, `EACCES` and `EPERM`. Detail: "Detail of D7" below. | F12, F13, F16. The owner of `/proc/<pid>` is root for a non-dumpable process. Such a row drops out and cuts its descendants off in `Descendants`, which leaks them. A fixture cannot set the owner without root, but it can set the `Uid:` line. A host with `hidepid=1` lists other users and gives `EACCES` on their files. | The owner of `/proc/<pid>`: it diverges from `ps` and has no test. Return every error but `ENOENT` and `ESRCH`: the reaper and gc stop on a `hidepid=1` host. |
| D8 | `Started` of each row has the resolution of one second and the same value that `StartTime` gives for the pid. | F8. A persisted tree must match a new listing. | Keep microseconds on darwin: `Identity` does not match a tree file, and it does not match `ps`. |
| D9 | `proctree` exports `CommOf(pid) (string, error)`. It reads the sources of D6 and D7 for one pid. `ArgsOf` is not exported, because no caller needs it. `ArgsReader` becomes the unexported `argsReader`. | F2, F5. A dead export on a protected surface has a cost and no user. | Export `ArgsOf` and `ArgsReader`: no consumer. |
| D10 | `faketmux` replaces `commandOf` with `proctree.CommOf`. It replaces `runPS` and `parseProcesses` with a `proctree.Lister` seam. The type `process` and `parseProcesses` are deleted. The `bridge` test code does not change in this build (follow-up F-3). The own-uid narrowing is stated in "Detail of D10". | F5, F6. The acceptance needs only `faketmux`. The `bridge` copy of the decoder works and is not on the acceptance path (F-3). | Copy the readers into `faketmux`: two homes for one rule. Rebuild the `bridge` helpers now: it adds a weaker secret check unless a positive control comes with it (F-3). |
| D15 | Before darwin `readTable` returns, it checks the row of its own process. The pid must be `os.Getpid()`, the ppid `os.Getppid()`, the pgid `syscall.Getpgrp()`, and the start second `StartTime(self)` truncated to the second. A mismatch returns an error that names the layout. The reaper then refuses to run, which is the safe failure. T3 pins it with a fake `sysctlCall`. | F10, F11. The operator host updates macOS before the `macos-latest` image, so a CI-only control comes too late. A wrong ppid offset corrupts `Descendants` and so the tree that `OwnedByDispatch` can kill. The own-pid check of the reaper (`go/internal/proctree/reap.go:41`) catches a wrong pid offset only. | A check in CI only: it fires after the operator host has run with the wrong layout. A feature flag: forbidden (D12). |
| D11 | Other platforms return an error that names the platform, as `procargs_other.go` does. | Existing precedent. | A silent empty table: a reaper reaps nothing. The failure must be loud. |
| D12 | There is no feature flag and no shadow switch. The change is `enforce` from the first build. | Repo rule. The parity test is the evidence (section 10). | A flag: forbidden. |
| D13 | `faketmux-reaper-needs-no-ps-binary` is consumed by the same build PR. D10 is the whole fix of that item. Its three acceptance rows map to T9, T8 and T11. Row 3 changes its test: it maps to `TestSeams_TheFakeSignalsOnlyALiveGroupLeaderThatIsNotItsOwnGroup` and not to `TestClaimIn_*` (section 8). The consuming PR records the change and its reason. | The item lists this item as a dependency, and its only change is D10. A separate build builds D10 twice. | Two PRs: a second lane builds the same code. |
| D14 | The constant `psStartSlack` in `bridge/dispatch_reap.go:31` keeps its name. D23 of the hygiene plan says that the slack covers the one-second resolution of the start second. | A rename touches the reaper for no change in behavior. | Rename to `startSlack`: a protected-surface diff with no gain. |

Detail of D6:

- On darwin, `commFrom` returns `p.Args[0]` when `Args` is not empty. Otherwise it returns `p.Comm`, the `p_comm` of offset 243.
- `withArgs` sets `Args` and `Env`, and then sets `Comm` with `commFrom`. Darwin makes one `KERN_PROCARGS2` read for each pid.
- `CommOf` reads one `Process` and calls `commFrom`. Its `p_comm` fallback needs a single-pid record.
- For that record, `kinfoStart` (`procstart_darwin.go:42-52`) becomes `kinfoRecord(call, pid) ([]byte, error)`. `StartOf`, `StartTime` and the fallback share it.
- `parseProcArgs2` and `procargs.go` do not change.

Detail of D7:

- One stat parser serves `StartTime` and `readTable`. It splits after the last `)`, and returns comm, ppid, pgrp and start ticks. It replaces `startTicks` (`procstart_linux.go:46-53`).
- The scan reads the boot time once.
- The scan has a `readFile` seam, so a test can inject `ESRCH`.
- A skipped pid cannot be in the own-uid table. `hidepid` also hides non-dumpable processes of the own uid from `ps`, so parity holds.

Detail of D10:

- `faketmux` lists only own-uid rows, where `ps -A` listed all users.
- A setuid foreground program such as `sudo` drops out of `pane_current_command`, and the parent shell shows instead.
- `internal-faketmux.md` states this.

Consequences:

- Good: `ExecLister` runs in a cell. The production callers do not change. The reaper, gc and faketmux share one source.
- Bad: the darwin decoder depends on the `kinfo_proc` layout. A new macOS release can change it. D15 refuses the table on that host at run time, and T3 fails on `macos-latest`.
- Condition that changes this decision: if Apple changes the layout twice, adopt `x/sys/unix` (candidate D) behind a `proctree` adapter and add its row to `thirdPartyHomes`.

## 7. The exact files

Change:

| File | Change |
|---|---|
| `go/internal/proctree/list.go` | Delete `NewLister` and `psArgs`. Add the unexported `newLister(readTable, readArgs, uid)`. `ExecLister` calls it. Rename `ArgsReader` to `argsReader`. Keep `withArgs`, and make it set `Comm` with `commFrom` after it sets `Args` and `Env`. |
| `go/internal/proctree/procstart_darwin.go` | Turn `kinfoStart` (lines 42-52) into `kinfoRecord(call, pid) ([]byte, error)`. `StartOf` and `StartTime` read the start fields from the record. |
| `go/internal/proctree/procstart_linux.go` | Replace `startTicks` (lines 46-53) with one stat parser that returns comm, ppid, pgrp and start ticks. |
| `go/internal/proctree/table.go` | Delete `parsePS`, `parsePSRow` and the `ps*` constants. Keep `Process`. |
| `go/internal/proctree/table_test.go` | Delete `TestParsePS*`. T10 replaces the uid test (`table_test.go:9`). |
| `go/internal/proctree/cover_test.go` | Delete `TestParsePSRow_*` (lines 60 and 76) and the tests of `NewLister`. |
| `go/internal/proctree/list_test.go` | Keep the `withArgs` pin (line 13) and give it an injected `argsReader`. Add the test that a failed argument read keeps the row with an empty `Env`. |
| `go/internal/proctree/list_integration_test.go` | Delete the `ps` assumptions. T1, T4 and T6 live here or in the new test files. |
| `go/internal/faketmux/session.go` | `commandOf` calls `proctree.CommOf`. Remove the `sysexec` import if it is unused. |
| `go/internal/faketmux/format.go` | Replace `runPS` and `parseProcesses` with a `proctree.Lister`. Delete `process`. |
| `go/internal/faketmux/server.go` | Line 19: the field `procs func() ([]process, error)` takes the lister type. |
| `go/internal/faketmux/faketmux.go` | Line 59: wire the lister. |
| `go/internal/faketmux/faketmux_test.go` | Line 51 keeps `procs: listProcesses`, and `listProcesses` keeps its name. Line 329 (the `[]process` literal) uses `proctree.Process`. Lines 343-349 (`parseProcesses`) are deleted. |
| `go/internal/faketmux/faults_test.go` | Lines 60, 99 and 194-196: swap the `runPS` seam for the lister seam. Lines 215-219 (`setSeams`): save and restore the lister seam in place of `runPS`. |
| `go/.darwin-set` | Add `./internal/faketmux`. The new `format_darwin_test.go` (F21) makes the generator list it. Run `make -C go darwin-set`, and check that the stale-file test passes. Do not edit the file by hand. |
| `docs/incidents/2026-11-18-faketmux-tests-need-a-ps-binary-that-a-lane-cell-denies.md` | In the row "What changed", name T8 and T9 as the regression tests. Replace the status `open`. |
| `docs/incidents/REGRESSION-COVERAGE-INDEX.md` | Line 151: set the status to ✅, and name T8 and T9. |
| `go/test/structure/binary.go` | Add `ProgramLiteralFiles(root, name string, dirs ...string)`. It keeps a file when `runsAProcess` and `hasLiteralEqual(file, name)` hold, as `MCPRegistrars` does (F22). |
| `docs/architecture/packages/internal-proctree.md` | Rewrite the bullet "The table" and the purpose list. Add `CommOf`. Line 48: name T10 as the pin of the uid invariant. |
| `docs/architecture/packages/internal-faketmux.md` | Line 64 (`ps -o comm= names a shell`): say that `CommOf` gives `argv[0]` on macOS and the `stat` comm on Linux. Also replace the `ps` text at line 45 and the "No platform branch" text at line 83. State that the table has the own-uid rows only (D10). |
| `docs/plans/dispatch-process-hygiene-2026-10.md` | Add one line under D2 that points to this plan. Line 67 (D23, "ps dates"): the start second comes from the kernel, and the slack of `psStartSlack` stays (D14). Line 88 (S3): name T10 as the pin of the uid filter. |

Add:

| File | Content |
|---|---|
| `go/internal/proctree/table_darwin.go` | `readTable`, the `kinfo_proc` decoder and its offset constants. It reuses `ctlKern`, `kernProc` and `kinfoProcBytes` from `procstart_darwin.go:12-16`. |
| `go/internal/proctree/table_linux.go` | `readTable` and the `/proc` scan, with a `root` seam as in `procstart_linux.go`. |
| `go/internal/proctree/table_other.go` | `readTable` that returns the platform error. |
| `go/internal/proctree/comm.go` | `CommOf`. |
| `go/internal/proctree/comm_darwin.go`, `comm_linux.go`, `comm_other.go` | `commFrom` for each OS (D6). |
| `go/internal/faketmux/format_darwin_test.go` | T16. |
| `go/internal/proctree/table_darwin_test.go`, `table_linux_test.go`, `kernel_lister_test.go`, `kernel_parity_test.go` | The tests of section 8. |
| `go/test/structure/proctree_ps_test.go` | T9. |

The package `proctree` is a protected surface. The change needs the protected review set (full seats, adversarial review and security review), as the rule for protected items says.

## 8. Red tests

Each test, except the guards T4, T6 and T11, must fail on `origin/main` for the stated reason, and pass after the build. A guard passes today and must keep passing. T6 is a guard on darwin only.

| Id | Name | Package | What it proves | Acceptance |
|---|---|---|---|---|
| T1 | `TestExecLister_RunsNoProgramAndListsThisProcess` | `proctree` | The test sets `PATH` to an empty directory, so each exec fails. `ExecLister()` still lists the table, and the table has `os.Getpid()`. Red today: `ps` is not found. | 1 |
| T2 | `TestCommOf_ReadsOnePidWithoutExec` | `proctree` | With an empty `PATH`, `CommOf(os.Getpid())` gives the name of the test binary. Red today: the function does not exist (compile error, accepted for a new export). | 1 |
| T3 | `TestKinfoLayout_MatchesThisProcess` (darwin) | `proctree` | The decoder reads `os.Getpid()`, `os.Getppid()` and `Getpgrp()` from the offsets of D5 (560 and 564). It fails when a macOS release moves a field. A second case with a fake `sysctlCall` that moves the ppid offset must make `readTable` return the layout error of D15. | 2 |
| T4 | `TestExecLister_MatchesPSForTheOwnUIDTable` (guard) | `proctree` (integration tier) | Three snapshots: `ps -A -o pid=,ppid=,pgid=,lstart=,comm=`, the kernel lister, then `ps` again. Only pids with equal `ps` rows in both `ps` snapshots are compared: ppid, pgid, comm, and the start second (Linux: at most 1 s apart). The set must hold the test process, its parent and a child that runs `exec.Cmd{Path: "/bin/sleep", Args: ["renamedproc", "30"]}`. That child has `Comm` `renamedproc` on darwin and `sleep` on Linux. The PR records the counts and the timing. | 2 |
| T5 | `TestReadTableAt_SkipsAPidThatVanishesAndKeepsACommWithSpacesAndParens` (linux, `root` seam) | `proctree` | A fixture `/proc` has four pids. One has `(a b) c)` as comm in `stat`. The others give `ENOENT`, `ESRCH` (through the `readFile` seam) and `EACCES` (directory mode 000). The first row is right and the other three are skipped. A malformed `status` or `stat` returns an error. | 2 |
| T6 | `TestExecLister_RowsHaveTheStartSecondOfStartTime` (guard on darwin) | `proctree` | For this pid, `Started` equals `StartTime(pid)` truncated to the second. | 2 |
| T7 | `TestReadTable_AnUnsupportedPlatformFailsLoudly` (`other` build) | `proctree` | The error names the platform. The table is not empty. This is compile-only evidence: no CI job builds the `other` tag. Step 6 of section 9 compiles it with `GOOS=freebsd go vet`. | 2 |
| T8 | `TestFakeTmux_PaneLeaderAndForegroundCommandRunWithoutPS` | `faketmux` | With `PATH` set to an empty directory, the pane runs `exec /bin/sleep 30` with absolute paths. `isPaneLeader` and the `pane_current_command` expansion give the right shell and child name. This test fails today with `fork/exec`: `exec: "ps": executable file not found in $PATH`. The 8 red tests of the item pass with it. | 3 |
| T9 | `TestStructure_ProctreeAndFaketmuxRunNoPS` | `go/test/structure` | `ExecImporters` on `internal/proctree` finds that the non-test exec importers are exactly `{cwd.go}`. This is an allow-list until F-1. `ProgramLiteralFiles` with the name `ps` (F22) finds no non-test file in `internal/faketmux`. T4 runs `ps` in a test file of `proctree`, so the scan skips `_test` files. The model is `binary_test.go:55`. | 1 |
| T10 | `TestReadTable_KeepsOnlyTheRowsOfTheGivenUID` (darwin and linux) | `proctree` | On darwin, a fake `sysctlCall` asserts the MIB `{1, 14, 5, uid}`. On Linux, a fixture `/proc` has rows with different real and effective uids in the `Uid:` line. Only the rows whose effective uid is the given uid stay. This test replaces `TestParsePS_KeepsOnlyTheRowsOfTheGivenUser` as the pin of S3. | 1 |
| T11 | `TestSeams_TheFakeSignalsOnlyALiveGroupLeaderThatIsNotItsOwnGroup` (existing, `faketmux`, guard) | `faketmux` | A mutation check on the group-lead rule of `isPaneLeader`. The test uses the injected `recordingSender` and has the cases "the pid leads another group" and "the pid leads our own group" (`runroot_test.go:373`). The build seat removes the `pgid != pid` check, and then the own-group check, and runs only this test with `-run` each time. It records both red results. It never mutates a guard under the real `signalGroup` (`session.go:27`), which sends `SIGKILL` to a real group. `TestClaimIn_ReapsOnlyTheMarkedRunsWhoseOwnerIsGone` (`runroot_test.go:55`) stays as the positive check on real processes: the stale pane is reaped, so `CommOf` names the pane shell `sh`. | 3 |
| T12 | `TestReadTableVia_RetriesOnENOMEM` (darwin) | `proctree` | A fake `sysctlCall` returns `ENOMEM` for the first buffer and then succeeds. A persistent `ENOMEM` returns an error after the fixed count. An error other than `ENOMEM` on the first sizing call returns at once. | 2 |
| T13 | `TestNewLister_ReturnsTheReadTableError` | `proctree` | `newLister` with a failing `readTable` returns that error and no rows. It covers the error branch for the 100% rule. | 2 |
| T14 | `TestCommOfVia_FallsBackToPCommAndMapsAGonePid` (darwin) | `proctree` | A fake `sysctlCall` makes the argument read fail, and `CommOf` returns `p_comm`. A record with a size below `kinfoProcBytes` gives `ESRCH`. | 2 |
| T15 | `TestCommOfAt_ReadsStatComm` (linux, `root` seam) | `proctree` | `CommOf` returns the `stat` comm of a fixture pid. A missing pid gives `ESRCH`. | 2 |
| T16 | `TestFakeTmux_PaneCommandIsArgv0OnDarwin` (darwin, `format_darwin_test.go`) | `faketmux` | After the pane runs `exec -a renamedproc /bin/sleep 30`, `#{pane_current_command}` gives `renamedproc`. This pins D6 end to end. The darwin file makes the darwin file set differ from the linux set, so the generator lists `./internal/faketmux` (F21). | 3 |

Acceptance 3 of this item is also a command: `go test -count=1 ./internal/faketmux` inside a lane cell, with the result `N/N PASS`.

The three acceptance rows of `faketmux-reaper-needs-no-ps-binary` map as follows:

- Row 1 (no exec of `ps` in `go/internal/faketmux`): T9.
- Row 2 (the suite passes in a cell): T8 and the cell command.
- Row 3 (the reaper test fails on a group that the pane does not lead): T11.

CI evidence for faketmux: the build adds `./internal/faketmux` to `go/.darwin-set` (F19) through the generator, because T16 is a darwin-only file (F21). So `macos-latest` runs T8, T11 and T16.

The Linux CI job runs T1, T2, T4, T5, T6, T8, T9, T10, T11, T13 and T15. It uses `-race`, the integration tier and Go 1.23 (`GOTOOLCHAIN=go1.23.12`). The darwin set runs T1 to T4, T6, T8, T10 to T14 and T16. T3, T12, T14 and T16 are darwin-only. T7 is compile-only.

T4 needs `ps`, so it cannot run in a cell. It sits in the integration tier, which the CI jobs run (`make test-integration`, `make test-integration-darwin`). A cell does not run that tier, and the run log of the cell must say so. The test must not skip itself silently.

## 9. Order

1. T3, T1, T2 and T10 first. They fail for the stated reasons. The reason for the order: they pin the layout and the seam before any code.
2. `table_darwin.go`, `table_linux.go`, `table_other.go`, `comm.go`, `newLister`. Delete the `ps` route (D1).
3. Move `faketmux` (D10). Add T16. Regenerate `.darwin-set` with `make -C go darwin-set`.
4. T4 on both CI jobs. Fix any offset from its output.
5. Update the package notes. Run `evolve docs ste-lint --strict` on them.
6. Run the whole-tree ratchets before the review seats: `rawgitratchet`, `testdup`, `sizeratchet`, the guards, `acs protectedsurface` and `cover changed`. Use `GOTOOLCHAIN=go1.23.12`. Run `GOOS=freebsd go vet ./internal/proctree` to compile the `other` build (T7). The new code needs 100% coverage. Then run the protected review set.

## 10. Rollout and evidence

- **Stage: enforce.** The rule forbids a feature flag. The repo selects shadow and enforce with policy `stage` keys. A dual-read shadow keeps the `ps` route that D1 deletes. That route cannot run in a cell. A parity test is the shadow in this case, because the change swaps one source for another of the same type.
- **Evidence 1 (parity):** T4 green on `ubuntu-latest` and `macos-latest`, over the full own-uid table. The PR records the counts and the timing.
- **Evidence 2 (cell):** `go test -count=1 ./internal/faketmux` in a lane cell prints `ok` with all tests. Before the build it prints 8 failures that name `fork/exec /bin/ps`.
- **Evidence 3 (no regression):** the existing reaper tests and `TestExecLister_ReadsTheTagOfARealChildAndTheReaperStopsOnlyThatChild` pass. They prove that the reaper kills only the tagged child with the kernel table.
- **Rollout of persisted trees:** a tree file that an older binary wrote holds a start second from `ps`. The new reader gives the same second for the same process (T6). On Linux, `ps` and `/proc/stat` can differ by one second at the boot time rounding. T4 allows a difference of one second against `ps` only, and T6 holds exact equality against `StartTime`. A mismatch makes `Recorded` match nothing, so the reaper spares the process. The error is on the safe side.

## 11. Risks

| Risk | Control |
|---|---|
| A macOS release moves a `kinfo_proc` field, and the decoder reads a wrong pid or ppid. A reaper can then select a wrong process. | D15 refuses the table at run time on the host that updates first. T3 and T4 on `macos-latest` fail on that release. The reaper keeps its proof checks: the tag, the identity and `Protected` (the kill path stays safe). |
| The table is bigger than the first buffer, because a process starts between the two `sysctl` calls. | Size the buffer with the first call, add slack, and retry on `ENOMEM` up to a fixed count. T12 covers the retry. |
| `comm` differs from `ps`, for a renamed `argv[0]` or a long name. | D6 uses `argv[0]`, as `ps` does. T4 compares exact values, with a renamed child. |
| Linux `hidepid` hides other users. | With `hidepid=1`, other users stay listed but their files give `EACCES`. D7 skips such a pid, as `ps` does, and T5 covers it. |
| A hot scan costs more than one `ps`. | One `sysctl` on darwin. On Linux, two small reads for each pid. T4 measures it, and the PR reports the time. |
| Other callers still use `ps` or `lsof` in a cell. | Section 12 files them. T9 holds the two packages of this item. |
| The uid filter loses its pin when `parsePS` goes. | T10 pins the filter on both platforms. |

## 12. Open questions and follow-up items

- **Q1 (closed).** The offsets of `e_ppid` and `e_pgid` are 560 and 564. The in-repo decoder of F14 uses them, and it runs on macOS today. T3 still pins them.
- **Q2 (closed).** `ps comm=` on macOS prints `argv[0]` (F15). D6 follows it, and T4 checks it with a renamed child.
- **Follow-up F-1.** The `lsof` cwd lister (`ExecCwdLister`) fails in a cell for a reason not yet found (`/usr/sbin/lsof` is not setuid). Its kernel route on darwin is `proc_info` with `PROC_PIDVNODEPATHINFO`.
- **Follow-up F-2.** The observer CPU probe runs `ps -o time=`. It moves to `proc_pidinfo` or `/proc`.
- **Follow-up F-3.** Remove the second `kinfo_proc` decoder in the `bridge` test files (F14): `darwinSysctlListProcs`, both `fallbackLister` files and `darwinProcArgsListing`. Replace them with one untagged helper beside its only caller (`tmux_hermetic_env_integration_test.go:47`). The helper needs a positive control: the listing must hold the args of the process that the test started, or `os.Args` of the test. Without it, a row with no `Args` makes the secret check pass with nothing checked. The reason for the delay: this build does not need the change, and it touches a security check.
- **Inbox filing.** The console files F-1, F-2 and F-3 as inbox items with lane-safe routes before the build starts. It links their ids here. This plan does not create them, because the review seat runs no verb that changes the inbox.
- **Consumes:** `faketmux-reaper-needs-no-ps-binary`. The build PR closes both items (D13). No lane builds the faketmux change a second time.
