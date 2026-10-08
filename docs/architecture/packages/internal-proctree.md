# internal/proctree

> Plan of record: [dispatch-process-hygiene-2026-10.md](../../plans/dispatch-process-hygiene-2026-10.md) (lane H2, decisions K8 to K11). It has the security model of the kill path, with one test for each way to kill a foreign process.

## Purpose

`internal/proctree` is the one kill path for the processes that a dispatch leaves behind. It owns:

- the process table (`Process`, `Lister`, `NewLister`, `ExecLister`);
- the identity of a process (`Identity`: the pid plus the start time);
- the ownership proofs (`Proof`, `TaggedWith`, `Recorded`, `AnyOf`, `Descendants`, `Select`);
- the dispatch id (`DispatchID`, `ParseDispatchID`), the value of `EVOLVE_DISPATCH_ID`;
- the shared class (`SharedHelper`) and the dispatch ownership proof (`OwnedByDispatch`);
- the persisted tree of a dispatch (`TreeDir`, `TreeFile`, `SaveTree`, `LoadTree`, `RemoveTree`);
- the reaper (`Reaper.Reap`, `Report`).

The bridge uses it at the end of each dispatch (`internal/bridge/dispatch_reap.go`). `evolve gc` uses it for its backstop rules (`internal/gc/dispatch_processes.go`).

## Design

- **The table.** `ps -A -o pid=,ppid=,pgid=,uid=,lstart=,comm=` runs with `LC_ALL=C`, so `lstart` has one format. The table keeps only the rows of the current uid.
- **Arguments and environment come from the kernel, apart.** On macOS, `sysctl KERN_PROCARGS2` gives the argument count, the arguments and then the environment. On Linux, `/proc/<pid>/cmdline` and `/proc/<pid>/environ` give them. `ps -E` is not used: it prints the arguments and the environment on one line, so an argument such as `grep EVOLVE_DISPATCH_ID=x` can look like a tag. The table keeps only the `EVOLVE_` keys of each environment.
- **macOS hides the environment of Apple binaries** (`/bin/zsh`, `/bin/sleep`, `/usr/bin/tail`). Their arguments stay visible. A process with a hidden or unreadable environment has an empty `Env`, so no tag proof can match it.
- **Each proof is a predicate** (the Specification pattern). `TaggedWith(id)` reads only the environment map, and an empty id matches nothing. `Recorded(ids)` matches the pid and the start time together. `Descendants` refuses pid 0, pid 1 and negative pids as a root, because the tree of init holds every orphan on the host.
- **The reaper is one sequence** (a template method). It lists the table and selects the proof. It sends SIGTERM to each match and waits the grace time. It lists again and sends SIGKILL to each process that still has the same identity and still matches the proof. It lists again, and each process that is still there is a survivor.
- **Signals go to single pids only.** The reaper refuses pid 0, pid 1, negative pids and the pids in `Protected` (the caller passes its own pid and its parent's pid). It never signals a process group. `ESRCH` means that the process is gone, and it is not an error.
- **A listing that fails after SIGTERM stops the sequence.** The reaper sends no SIGKILL without a fresh proof.
- **The tag proves descent, not ownership.** A process that the dispatch started can be a helper that other clients now use. `SharedHelper` reads one table: the program `tmux`, the Claude Code helpers (`daemon run`, `bg-pty-host`, `bg-spare`) and Chrome or Chromium. `OwnedByDispatch(id, tree, live)` matches a tagged process outside the shared class, or a member of the recorded tree. A member that is a shared helper must also be in `live`: the live descendants of the recorded panes at sweep time.
- **The persisted tree** is `.evolve/dispatch-trees/<id>.json` (pid and start time). `TreeFile` maps each character outside `[A-Za-z0-9-]` to `_`, so a crafted id cannot leave the directory. A missing file is an empty tree.
- **The reaper protects its ancestors.** It never signals `Self` or any pid on the ppid chain of `Self` in the table, and never the pids in `Protected`. When `Self` is not in the table, it refuses to run.
- **The tree directory** is mode 0700.
- **`DispatchID` is the single source of the tag format**: `<run>/<cycle>/<agent>/p<owner-pid>n<nonce>`. The bridge mints it with `String()`, and gc reads it with `ParseDispatchID`. A slash in the run or the agent becomes `_`, so the parts cannot shift. `ParseDispatchID` refuses an owner pid of 0 or 1.

## Invariants

- The arguments and the environment stay apart (`TestParseProcArgs2_KeepsArgumentsAndEnvironmentApart`, `TestTaggedWith_ATagInTheArgumentsIsNoProof`).
- The table keeps only the rows of the given uid (`TestParsePS_KeepsOnlyTheRowsOfTheGivenUser`).
- A reused pid is a different process (`TestRecorded_MatchesPidAndStartTimeTogether`, `TestReap_APidReusedByANewProcessIsNotKilled`).
- Each round checks the proof again (`TestReap_AProcessThatLostItsProofIsNotKilled`).
- pid 0, pid 1 and the protected pids never get a signal (`TestReap_NeverSignalsInitItselfOrItsParent`, `TestReap_SignalsSinglePidsOnly`, `TestDescendants_RefusesInitAndPidZeroAsARoot`).
- A tagged shared helper outside the tree is never selected (`TestOwnedByDispatch_ATaggedSharedHelperNeedsTheRecordedTree`, `TestSharedHelper_MatchesTheClassOfHelpersOthersUse`).
- No ancestor of the reaper gets a signal (`TestReap_NeverSignalsAnAncestorOfTheReaper`).
- A tree file stays in its directory (`TestTreeFile_StaysInsideItsDirectory`).
- No SIGKILL after a failed listing (`TestReap_AListingFailureAfterTermSendsNoKill`).
- The tag format round-trips (`TestDispatchID_RoundTripsThroughItsString`, `TestParseDispatchID_RefusesEveryMalformedTag`).
- The real lister reads the tag of a real child, and the reaper stops only that child (`TestExecLister_ReadsTheTagOfARealChildAndTheReaperStopsOnlyThatChild`, integration tier).

## Limits

- A descendant that calls `setsid`, leaves the pane tree before the record and is an Apple binary has no tag and no record. A Linux cgroup per dispatch can close this gap. The host is macOS, so this is a later item.
- The start time has a resolution of one second.
- A process in uninterruptible sleep survives SIGKILL. The caller reports it.
