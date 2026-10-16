# internal/verifylock

> The decision: [ADR-0080](../adr/0080-runtime-console-plane-separation.md) (P1, the verification single flight; the hub location follows its plane discipline). Its callers: the ACS suite (`internal/acssuite`) and the build-floor coverage run (`internal/core/build_floor_reviewer.go`).

## Purpose

`internal/verifylock` serializes the expensive `go test` verification runs of one host. Fleet lanes are separate OS processes. Each lane runs full package suites during the ACS run and the build handoff floor. If they run at the same time, one host is oversubscribed and a long suite turns into a false RED.

One `flock`, resident in the hub, is shared by every lane of a repository. LLM phases stay parallel. Only verification is single-flight.

## Design

- **The lock file is in the git common dir** (`evolve-verify.lock`), the hub that every worktree of the repository shares. This is the same cross-plane location discipline as the console lease.
- **`Acquire` blocks until the lock is free, or until the context ends.** `flock` has no context-aware blocking mode, so a waiter polls with a non-blocking try every `pollInterval` (250 ms). The coarse poll is fine, because a verification run takes tens of seconds.
- **A waiter is never silent.** After `waitNoteAfter` (5 s) it prints a queue note. It repeats the note with the elapsed time every `renoteEvery` (60 s). A suite hold can last tens of minutes, and a single note reads as a hang to an operator and to the stall observer.
- **`AcquireWithin` bounds the wait** with `MaxWait` (15 minutes). The caller gets an error that says the run goes unserialized. A deadline of the caller itself is not reported as that bound.
- **`AcquireAt` is the one flock discipline**, taking an explicit lock path. `Acquire` resolves the path and calls it.
- **The release function is idempotent** (`sync.Once`).

## Invariants

- **At most one holder.** Pinned by `TestAcquire_SerializesAcrossHolders`.
- **A canceled context frees a waiter.** Pinned by `TestAcquire_CtxCancelUnblocksWaiter`.
- **A second release is a safe no-op.** Pinned by `TestAcquire_ReleaseIsIdempotent`.
- **The lock lands in the hub.** For a primary checkout it lands inside `.git`, shared by every worktree. That is what makes the single flight host-wide. Pinned by `TestAcquire_ResolvesTheHubLockPath`.
- **A degraded lock is loud.** A root whose hub cannot be resolved falls back to a lock file beside the project root. That is correct for one checkout, but it is per worktree and gives no exclusion between lanes. So `Acquire` prints a `PER-WORKTREE` warning. A silent fallback looks like working single-flight until the false REDs return. Pinned through `AcquireWithin` by `TestAcquireWithin_ResolvesTheLockAsAcquireDoes`.
- **A locking failure never skips the verification.** `Acquire` degrades and `AcquireWithin` reports that the run goes unserialized. The caller runs the suite either way.
- **A bound shorter than a hung holder.** `MaxWait` ends before a hung holder uses its whole `go test` budget. Pinned by `TestMaxWait_EndsBeforeAHungHolderUsesItsWholeGoTestBudget`.
- **The bound is reported as the bound.** Pinned by `TestAcquireWithin_AWaiterBehindAHeldLockGivesUpAtTheBoundAndSaysSo` and `TestAcquireWithin_TheCallersOwnDeadlineIsNotReportedAsTheBound`.
- **The queue note is unpinned.** No test reads the `waiting` note or its repeat interval.

## Findings

- A batch halt had one fingerprint: a package suite red on three lanes and green in the preserved worktree. Two lanes plus the ACS run oversubscribed one host and turned a 43-second suite into false REDs. Verification correctness outranks wave throughput. The detail is in the [comment history archive](../../history/code-comments/internal-verifylock.md).
