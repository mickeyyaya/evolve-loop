# Comment history: `internal/cli/opscmd`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h2

### `go/internal/cli/opscmd/changelog_test.go:103` — above `if !strings.Contains(out, "### Fixed") {`

```text
// v0.1.0..HEAD excludes commits AT v0.1.0 (the feat one), so only "fix: second" is in range.
```

### `go/internal/cli/opscmd/cli_error_paths_test.go:12` — above `func TestRunDoctor_ArgAndProbeBranches(t *testing.T) {`

```text
// cli_error_paths_test.go — cycle-543 Task 3 (cli-command-layer-test-coverage).
// In-package exit-code / stderr-content tests for the opscmd wrapper
// arg-parsing + error branches (moving past the package apicover-smoke test's
// "produced some output" bar). Kept in-package so they count toward opscmd's
// coverage floor; the ACS predicates in go/acs/cycle543 assert the same
// behaviors but live in a different package.
```

### `go/internal/cli/opscmd/console_lease.go:3` — above `import (`

```text
// console_lease.go — ADR-0080 S4 writer: `evolve console-lease <path>...
// [--ttl 30m] [--reason text] [--project-root P]` records a time-bounded,
// exact-path operator lease in the git COMMON dir (the hub — outside every
// worktree, so no lane phase can author or shadow it; review BLOCK).
// `--clear` removes it. The reader (internal/core) adopts the lease once at
// cycle start and treats absent/expired/malformed as an armed guard.
```

### `go/internal/cli/opscmd/console_lease_test.go:3` — above `import (`

```text
// console_lease_test.go — ADR-0080 S4 writer surface: the lease lands in the
// git COMMON dir (outside every worktree), paths are normalized to the exact
// repo-relative form the guard compares, value flags survive the documented
// paths-first invocation (the ReorderArgs regression class), and --clear
// removes the lease — and only the lease.
```

### `go/internal/cli/opscmd/doctor_live.go:135` — above `func runDoctorLive(args []string, stdout, stderr io.Writer) int {`

```text
// runDoctorLive implements `evolve doctor live <driver> [--json]`: a REAL
// launch that submits one trivial prompt via bridge.LiveSmokeTest — the only
// probe shape that can see a provider quota wall (boot smoke passes against a
// rate-limited CLI; the wall appears only after work is submitted —
// cycle-283). Exit: 0 healthy, 1 walled (the escalation pattern is printed,
// e.g. rate_limit) or failed, 10 usage (unknown or non-tmux driver).
```

### `go/internal/cli/opscmd/doctor_plane.go:3` — above `import (`

```text
// doctor_plane.go — ADR-0080 S2: `evolve doctor plane [root]` reports which
// worktree plane a checkout occupies (runtime linked worktree vs the primary
// console checkout) and whether loop state lives there. The operator-facing
// half of the boot-time tripwire in cmd_loop.
```

### `go/internal/cli/opscmd/doctor_plane_test.go:3` — above `import (`

```text
// doctor_plane_test.go — ADR-0080 S2 CLI surface: the plane report and the
// exit-2 WARN when loop state lives in the primary (console) checkout.
```
