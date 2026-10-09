# internal/events/wake

> Spec of record: [event-channels.md](../event-channels.md) §7 (the wake) and §15 (timers). Plan: [event-notification-protocol-2026-10.md](../../plans/event-notification-protocol-2026-10.md), component E3. Decision: [ADR-0127](../adr/0127-push-only-event-channels.md).

## Purpose

`internal/events/wake` blocks a channel reader until the kernel posts an event. It never polls. It owns:

- the waiter (`Waiter`, `New`, `Arm`, `Wait`, `Close`);
- the watch set (`Targets`: the directories, the files and the pids);
- the result of a wait (`Wake`: `Changed`, `Exited`, `Hangup`, `Deadline`);
- the refusal (`ErrRefused`).

Nothing in production calls the package yet. The reader (E5) uses it, and E10 wires the reader.

## Design

- **Arm, catch up, wait.** The caller arms the watch set, reads every complete line, and then calls `Wait`. After each wake, the caller reads again. A wake only means "something can be new".
- **`Arm` sets the whole watch set.** Each call replaces the path watches of the call before it. The caller thus arms a new tail segment after a rotation, and the old tail stops its wakes on darwin. An event that comes between the two calls is not lost, because the caller reads after each `Arm`.
- **One port for each kernel.** A struct of syscall functions (`kqueuePort` on darwin, `inotifyPort` on Linux) holds every kernel call. The host port uses the `syscall` package. The tests replace it with a fake, so every branch has a test on its own system. `New` takes the host port.
- **darwin (kqueue).**
  - A directory gets `EVFILT_VNODE` with `NOTE_WRITE`. A file gets `NOTE_WRITE`, `NOTE_EXTEND`, `NOTE_DELETE`, `NOTE_RENAME` and `NOTE_ATTRIB`. A truncation gives only `NOTE_ATTRIB`. Both have `EV_CLEAR`, so one write gives one wake.
  - The waiter opens each path with `O_EVTONLY`, so a watch never keeps a volume busy.
  - `EVFILT_USER` with `NOTE_TRIGGER` is the cancel. `EVFILT_PROC` with `NOTE_EXIT` is the exit watch. `EVFILT_WRITE` on the output, with `EV_EOF`, is the hangup. A write event without `EV_EOF` is no wake.
  - `kevent` gets no timeout, or the time left to the deadline.
- **Linux (inotify and epoll).**
  - Each `Arm` makes a new inotify instance. The instance watches each directory and the directory of each file once, with `IN_MODIFY`, `IN_CREATE`, `IN_DELETE`, `IN_MOVED_FROM`, `IN_MOVED_TO`, `IN_DELETE_SELF` and `IN_MOVE_SELF`. Then the waiter puts it in the epoll set and closes the old instance.
  - The epoll set holds the inotify instance and the read end of a self-pipe for the cancel. It also holds one `pidfd` for each pid, and the output with `EPOLLHUP|EPOLLERR`.
  - A wake reads the inotify queue and the self-pipe until `EAGAIN`, because epoll is level-triggered. It reads 16 times at most in one wake. The rest of the queue wakes the next `Wait`. `IN_Q_OVERFLOW` is an event in the queue, so it is a wake.
  - The filesystem magic is masked to 32 bits, because a 32-bit system extends the sign of a large magic such as the one of `btrfs`.
  - `epoll_wait` gets the time left in milliseconds, rounded up, so it never returns before the deadline.
- **The cancel.** `Wait` arms `context.AfterFunc` on its context. The function fires the cancel event of the kernel. If the function started, `Wait` waits for it before it returns, so no trigger comes after `Close`. A failed trigger is joined only to a context error.
- **`EINTR`.** If the kernel call returns `EINTR`, `Wait` computes the time left again from its clock and calls again. An empty return before the deadline also calls again.
- **The deadline.** A zero deadline means no timeout. The clock of `Wait` decides when the deadline passed. The one-shot kernel timeout is the only timer.
- **The exit watch.** A pid that is gone at the arm (`ESRCH`) is an exit. The next `Wait` returns it at once, with no kernel call. An exit removes the pid from the watch set.
- **A pid left out.** The next `Arm` removes each pid that the caller leaves out. darwin sends `EV_DELETE`, and a knote that is already gone is no error. Linux closes the `pidfd`. A pid armed before a failure in the same `Arm` stays armed.
- **No half-applied `Arm` on Linux.** If the close of the old inotify instance fails, `Arm` still arms the pids, and it returns the joined errors.
- **The wake contract.** `Dirs` wake on changes to the set of entries. `Files` wake on changes to the content. A spurious wake is allowed. After a `Changed` wake on a file that a rename can replace, the caller runs `Arm` again. After each `Arm`, also one that only adds a pid, the caller reads again.
- **`Hangup` is terminal.** On darwin, `EV_EOF` can come only once, so the caller ends the watch at the first `Hangup`.
- **The output hangup.** `New(output)` arms the hangup only when the output is a pipe or a terminal. A regular file and a device that is not a terminal get no hangup. The terminal check is an `ioctl` (`TIOCGETA` on darwin, `TCGETS` on Linux).
- **Refusals.** Before it arms a path, the waiter reads its filesystem with `statfs`. darwin accepts `apfs` and `hfs`. Linux accepts `ext4`, `xfs`, `btrfs`, `tmpfs` and `overlay` (by the magic number). Each other filesystem, each other system, and each kernel call that refuses a watch give `ErrRefused` (exit 1 in the CLI). A path that does not exist gives a `statfs` path error that is not a refusal (exit 2).

## Invariants

- One write gives one wake, and the second wait times out (`TestWaiter_ASecondWaitAfterOneWriteTimesOut`). This proves `EV_CLEAR` and the drained inotify queue.
- An append, a first segment, a new directory and a new tail after a rotation wake the waiter (`TestWaiter_AnAppendWakesTheWaiter`, `TestWaiter_AWatchOnAnAbsentDirectoryWakesOnTheFirstSegment`, `TestWaiter_ADirectoryCreatedAfterArmWakesTheParentWatch`, `TestWaiter_RotationArmsTheNewTailSegment`).
- A process exit, a gone pid and an output hangup wake the waiter (`TestWaiter_ProcessExitWakesTheWaiter`, integration tier; `TestWaiter_AnAbsentPidWakesAtOnceAsAnExit`; `TestWaiter_AStdoutHangupWakesTheWaiter`).
- A cancel returns at once (`TestWaiter_CancelReturnsAtOnce`, `TestWaiter_ACancelDuringTheKernelWaitReturnsTheContextError`).
- `EINTR` retries with the time left (`TestWaiter_AnEINTRRetriesWithTheTimeLeft`).
- A network filesystem, another system and a kernel refusal fail loudly (`TestWaiter_RefusesANetworkFilesystem`, `TestNewWaiter_RefusesASystemWithoutABackend`, `TestWaiter_AKernelRefusalFailsLoudly`).
- No file in `internal/events/...` calls `time.Sleep`, `time.Tick`, `time.NewTicker`, `time.After` or `time.NewTimer` (`TestNoPollTimerInTheEventChannelSources`, `go/test/structure/nopoll_guard_test.go`). Its list of directories is data. E2, E12 and E14 add `flock.LockWithin` and the migrated watch files to it.
- Each change of a watched file wakes the waiter on both systems (`TestWaiter_AFileWatchWakesOnEveryChangeOfTheTail`). The changes are an append, a replacement by rename, a delete, a rename away and a truncation.
- `Wait` never returns while its cancel trigger runs, so no cancel comes after `Close` (`TestWaiter_AWakeAtTheCancelInstantWaitsForTheTriggerBeforeItReturns`).

## Limits

- A darwin waiter holds one descriptor for each watched directory and file, plus the kqueue.
- A Linux waiter uses one inotify instance, and two for a short time during `Arm`. The kernel default is 128 instances for each user.
- `pidfd_open` needs Linux 5.3 or later. An older kernel refuses the exit watch (`ENOSYS`), and the waiter fails with `ErrRefused`. No fallback exists.
- On Linux, a watch on a directory also wakes on a write to a file in it that is not the tail. The caller reads again and finds nothing new.
