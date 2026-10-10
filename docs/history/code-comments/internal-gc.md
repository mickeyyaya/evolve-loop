# Comment history: `internal/gc`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## phase 3d r3 (gc, audit)

### `go/internal/gc/discover.go:38` — above `var runMarkers = append([]string{`

```text
// runMarkers are the files whose presence identifies a directory as a run
// workspace. run.json is the canonical post-CB.4 marker (the WriteCycleState
// guard mirror); phase-timing/interaction-summary and the registry's required
// phase artifacts cover pre-CB.4 runs that only hold phase artifacts.
//
// Cycle-1141: the artifact half is DERIVED from phasecontract.RequiredArtifacts()
// — the report-filename SSOT — not re-typed. A frozen copy would stop
// recognizing a run dir the moment the registry's required set changed, and
// discovery is what protects a dir from the retention engine: a marker list
// that silently falls behind the registry turns into deleted run evidence.
```

### `go/internal/gc/discover_fleet_lease_test.go:11` — above `func TestDiscover_EmptyCurrentWorkspace_FreshLeaseStaysLive(t *testing.T) {`

```text
// discover_fleet_lease_test.go — regression pin for the fleet cycle-state
// isolation fix (2026-07-03). Under fleet each lane writes cycle state to its
// OWN per-run file (ipcenv.CycleStateFileKey), so the host-global
// .evolve/cycle-state.json is absent/stale and currentWorkspace() returns "".
// The GC must NOT reap a live lane's run dir on that basis: the per-run .lease
// (ADR-0049 G16) is the independent liveness signal in
// `Live: dir == currentWS || leaseFresh(dir)`. This test proves that with an
// EMPTY currentWorkspace, a fresh-lease lane stays Live and a stale-lease lane
// does not — the exact protection that keeps concurrent fleet lanes from being
// deleted mid-cycle.
```

### `go/internal/gc/discover_test.go:84` — above `writeFile(t, filepath.Join(dir, "cycle-state.json"),`

```text
// Global run state names cycle-7's workspace (non-terminal).
```

### `go/internal/gc/discover_test.go:87` — above `if err := runlease.Write(leased, runlease.Lease{RunID: "r6"}, t0.Add(-time.Minute)); err != nil {`

```text
// cycle-6 has a fresh lease; cycle-5 a stale one.
```

### `go/internal/gc/gc.go:42` — above `type (`

```text
// Policy, RunsPolicy and WorktreesPolicy are the `.evolve/policy.json` gc
// block, re-exported from the zero-dependency internal/gcpolicy leaf. The
// types moved out (cycle-1141) so internal/policy — the config SSOT — can hold
// the block without depending on this engine; these aliases keep every
// existing gc.Policy / gc.RunsPolicy call site working unchanged.
```

### `go/internal/gc/worktrees.go:143` — above `func LeafCycleNumber(leaf string) (int, bool) {`

```text
// LeafCycleNumber extracts the trailing cycle number from a worktree leaf after
// stripping a swarm suffix. cycle-aaa1111-570 -> 570;
// cycle-legacyB-8-integration -> 8; cycle-legacyC-9-w0 -> 9.
```

### `go/internal/gc/worktrees_apply_flagonly_test.go:3` — above `import (`

```text
// worktrees_apply_flagonly_test.go — builder-added coverage (cycle 570) for a
// safety property the RED suite pins only in PlanWorktrees, never in
// ApplyWorktrees: flag-dirty / flag-unmerged items are report-only, so applying
// a manifest that contains ONLY flags must perform ZERO git mutations (no
// worktree remove, no branch delete, no prune) and no trailing prune. Also the
// one place the WorktreeManifest type is constructed by name.
```

### `go/internal/gc/worktrees_fuzz_test.go:3` — above `import (`

```text
// worktrees_fuzz_test.go — property-based RED test (cycle 570,
// workspace-hygiene-s4-worktree-gc-planner) mirroring
// TestPlanNeverTouchesLiveDirs_Property (discover_fuzz_test.go): random
// synthetic worktree populations -> PlanWorktrees -> the three invariants
// that must hold no matter what the random draw produces.
//
// RED now: PlanWorktrees/WorktreeOptions do not exist yet (compile failure).
// Do NOT modify this file.
```

### `go/internal/gc/worktrees_realgit_test.go:5` — above `import (`

```text
// worktrees_realgit_test.go — real-git end-to-end RED test (cycle 570,
// workspace-hygiene-s4-worktree-gc-planner), mirroring the integration-tagged
// convention core/worktree_realgit_integration_test.go already uses for the
// sibling S1/S3 slices: exercise PlanWorktrees + ApplyWorktrees against a
// REAL git repo + real `git worktree add`, not the scripted fake, so the
// evidence-pipeline's actual git plumbing (porcelain parsing, merge-base
// membership, status) is proven end-to-end at least once.
//
// RED now: PlanWorktrees/ApplyWorktrees do not exist yet (compile failure).
// Do NOT modify this file. Run with: go test -tags integration ./internal/gc/...
```

### `go/internal/gc/worktrees_test.go:3` — above `import (`

```text
// worktrees_test.go — RED test (cycle 570, triage-committed task
// workspace-hygiene-s4-worktree-gc-planner; docs/plans/workspace-hygiene-2026-07.md
// Slice S4). 65 worktrees + 106 never-deleted cycle-* branches have
// accumulated (S3 stops NEW debt at cycle-exit; this slice drains the
// EXISTING backlog as a gc-sibling Plan/Apply planner, same shape as gc.go's
// Plan/Apply for run dirs).
//
// Evidence pipeline this file's fixtures assume (evidence-based, never
// name-parsed, per the plan):
//   - `git worktree list --porcelain` (run in ProjectRoot) is the source of
//     truth for registered worktrees + their branch (parsed from the
//     "branch refs/heads/<name>" line, NOT the directory leaf).
//   - only entries whose path sits under WorktreeBase AND whose directory
//     leaf has the "cycle-" prefix are candidates (mirrors the existing
//     gitWorktree.Cleanup / deleteCycleBranch gate in core/worktree.go).
//   - merged = the branch appears in `git branch --merged HEAD` (run in
//     ProjectRoot).
//   - dirty = `git status --porcelain` (run IN the worktree dir) is
//     non-empty.
//   - dead = NOT live. Live is proven by ANY of: (a) a fresh runlease.OwnerLive
//     .lease at <EvolveDir>/runs/cycle-<N>/.lease, where N is the trailing
//     numeric segment of the leaf after stripping a swarm suffix
//     ("-integration" or "-w<digits>") — so an integration/worker worktree
//     correlates to its parent cycle's lease; (b) EvolveDir/cycle-state.json's
//     active_worktree equals this path; (c) any EvolveDir/runs/*/run.json's
//     active_worktree equals this path (fleet per-run mirrors). An
//     unparseable leaf (no trailing numeric segment) skips lease evidence and
//     is only ever collectable once its own directory mtime is >7 days old.
//   - KeepRecent / MinAgeMinutes are grace periods on top of the above,
//     mirroring gc.go's RunsPolicy.KeepFull ladder: the MinAgeMinutes-youngest
//     candidates are never touched (covers the create->lease-write race
//     window) and, among the survivors, the KeepRecent newest (by mtime) are
//     always kept even if fully merged+clean+dead.
//   - a branch with the "cycle-" prefix that has NO corresponding
//     `git worktree list` entry at all (its worktree dir was already removed
//     some other way — the literal 65-worktree/106-branch skew) is swept by
//     the SAME merged check, emitting delete-branch (no remove-worktree,
//     nothing to remove) or flag-unmerged, with no liveness check needed —
//     nothing can be "checked out live" without a worktree entry.
//
// Contract (do NOT modify this file — implement production code instead):
//   WorktreesPolicy{KeepRecent, MinAgeMinutes}   — new gc.Policy.Worktrees field
//   WorktreeAction / WorktreeItem / WorktreeManifest
//   WorktreeOptions{ProjectRoot, WorktreeBase, EvolveDir, Policy, Now, Exec,
//                   PidAlive, LeaseTTL}
//   PlanWorktrees(WorktreeOptions) (WorktreeManifest, error)
//   ApplyWorktrees(WorktreeOptions, WorktreeManifest) error
//
// RED now: none of the above exist in this package (compile failure).
```
