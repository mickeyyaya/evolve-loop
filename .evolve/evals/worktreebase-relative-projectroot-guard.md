# Eval: worktreebase-relative-projectroot-guard

> Pins the closure of the `swarm-tests-relative-worktree-base` inbox defect. Cycle 296 moved
> the absolute-path guard into `worktreeBase()` but only on the `EVOLVE_WORKTREE_BASE`
> (env-override) branch; the DEFAULT branch still returned
> `filepath.Join(projectRoot, ".evolve", "worktrees")` verbatim — relative when `projectRoot`
> is relative (e.g. `"."`). A relative worktree base resolves against an unintended cwd in
> `git worktree add` and defeats the tree-diff guard. Cycle 297 adds the
> `filepath.IsAbs(projectRoot)` check to the default branch too. Source incident: soak batch #6
> (R8.4), cycle 297.

## Task
Add a `filepath.IsAbs(projectRoot)` guard to `worktreeBase()` in `go/internal/swarm/provision.go`
for the default-path case (no `EVOLVE_WORKTREE_BASE` set). Return `("", error)` mentioning
"absolute" when `projectRoot` is not absolute.

## Acceptance Criteria

### [code] worktreeBase refuses a relative projectRoot on the default (no-env) path

```bash
cd go && go test ./internal/swarm/... -run TestWorktreeBase_RelativeProjectRootRefused -count=1 -v 2>&1 | grep -E "^--- (PASS|FAIL)"
```

Expected output contains `--- PASS: TestWorktreeBase_RelativeProjectRootRefused`.

### [code] Absolute projectRoot still honored (regression)

```bash
cd go && go test ./internal/swarm/... -run TestWorktreeBase_DefaultPath -count=1 -v 2>&1 | grep -E "^--- (PASS|FAIL)"
```

Expected output contains `--- PASS: TestWorktreeBase_DefaultPath`.

### [code] Full swarm package passes

```bash
cd go && go test ./internal/swarm/... -count=1 2>&1 | tail -1
```

Expected output begins with `ok` for `github.com/mickeyyaya/evolve-loop/go/internal/swarm`.

## Anti-gaming check

A build that still returns `".evolve/worktrees"` verbatim from `worktreeBase(".")` without an
error fails `TestWorktreeBase_RelativeProjectRootRefused`: that test clears
`EVOLVE_WORKTREE_BASE`, calls the real unexported `worktreeBase(".")`, and asserts `err != nil`,
the returned path is empty, and the error mentions "absolute". A magic string in source cannot
satisfy it.
