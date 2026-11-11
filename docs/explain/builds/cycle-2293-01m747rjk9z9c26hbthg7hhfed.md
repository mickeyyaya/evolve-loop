# Build Explanation — Cycle 2293

## Build Binding
- Cycle: 2293
- Base SHA: 4ea0406bf9817b7c99036124e689a2b838d529fc

## Summary
The origin/main ref constant now has one home, gitexec.RemoteMain. The core and inboxmover packages read it.

## Rationale
Two packages held their own copy of the string origin/main. One shared constant removes the drift risk.

## Changed Areas
- `go/internal/gitexec/backup.go` — exports Remote, MainBranch and RemoteMain; replaces the private defaultRemoteMain.
- `go/internal/core/resume_base.go` — drops resumeMainRef and reads gitexec.RemoteMain.
- `go/internal/inboxmover/curate.go` — drops verifiedMainRef and reads gitexec.RemoteMain.
- `go/internal/gitexec/remote_main_test.go` — names the three new exports in a real assertion.

## Design Decisions
RemoteMain is built from Remote and MainBranch, so the three values cannot disagree.

## Verification
The gitexec, core and inboxmover tests pass without edits to existing tests. The cycle 2293 predicates pass.

## Compatibility
The value stays origin/main, so no log text or git argument changes.

## Limitations
Other literals of origin/main in strings and tests remain; the criterion covers constants only.
