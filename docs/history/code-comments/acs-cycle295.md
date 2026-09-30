# Comment history: `acs/cycle295`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle295/predicates_test.go:3` — above `package cycle295`

```text
// Package cycle295 materializes the cycle-295 acceptance criteria for the two
// committed top_n tasks (scout-report.md):
//
//	T1  checkpoint-clobber-fix             — FilesystemStorage.WriteCycleState
//	    whole-struct-replaces cycle-state.json, and core.CycleState has no
//	    "checkpoint" field, so every pre-dispatch write ERASES the "checkpoint"
//	    block PhaseBoundaryCheckpointer wrote after the prior phase. A crash
//	    mid-phase then leaves no checkpoint and `evolve loop --resume` fails
//	    (live incident: host reboot during cycle-294 mutation-gate). Fix: make
//	    WriteCycleState a read-merge-write that carries "checkpoint" through.
//	T2  core-worktree-relative-base-guard  — core/gitWorktree.Create() MkdirAll's
//	    the worktree base with NO filepath.IsAbs check (swarm/provision.go got
//	    that guard in cycle 294). A relative base silently creates dirs under cwd.
//	    Fix: add the same absolute-path guard before MkdirAll.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test:
//   - C295_001/002 call the REAL storage.FilesystemStorage.WriteCycleState and
//     assert on the REAL cycle-state.json bytes (checkpoint preserved / not
//     duplicated). A magic string in a .go file cannot make the on-disk JSON
//     keep a key the production write erases.
//   - C295_003/004 run the REAL core package tests (`go test -v -run ...`) that
//     drive the unexported gitWorktree.Create with a relative base and assert on
//     the real `--- PASS:` line. gitWorktree is unexported, so a subprocess test
//     run is the behavioral seam; a source string cannot produce a named PASS.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1 checkpoint preserved        → C295_001 (direct WriteCycleState call)
//	T1 no spurious / not dup'd     → C295_002 (direct WriteCycleState call)
//	T1 storage suite green         → manual+checklist (auditor)
//	T2 relative env base refused   → C295_003 (go test -run, PASS line)
//	T2 relative projectRoot refused→ C295_004 (go test -run, PASS line)
//	T2 core suite green            → manual+checklist (auditor)
```
