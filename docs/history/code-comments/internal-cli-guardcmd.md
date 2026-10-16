# Comment history: `internal/cli/guardcmd`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d2

### `go/internal/cli/guardcmd/cli_error_paths_test.go:13` — above `func TestRunEval_ArgBranches(t *testing.T) {`

```text
// cli_error_paths_test.go — cycle-543 Task 3 (cli-command-layer-test-coverage).
// Exercises the guardcmd wrapper arg-parsing + error branches at the exit-code /
// stderr-content level (the "doesn't panic" package-smoke test this task moves
// past). These are IN-PACKAGE tests so they count toward guardcmd's coverage
// floor (the ACS predicates in go/acs/cycle543 exercise the same behavior but,
// living in a different package, do not raise this package's coverage number).
```

### `go/internal/cli/guardcmd/cli_error_paths_test.go:463` — above `func TestRunEval_QualityCheckPredicatesFlagAfterPositional(t *testing.T) {`

```text
// TestRunEval_QualityCheckPredicatesFlagAfterPositional pins the flag-order
// trap: stdlib flag stops at the first positional, so the documented
// `quality-check <eval.md> -predicates <dir>` form silently dropped the flag
// and the lint ran on nothing while reporting PASS (probed live 2026-07-30 —
// the same class as opscmd's console-lease value-flag bug).
```

### `go/internal/cli/guardcmd/eval.go:76` — above `func parseInterspersed(fs *flag.FlagSet, args []string) (positional []string, ok bool) {`

```text
// parseInterspersed parses args into fs, accepting flags BEFORE or AFTER the
// positionals, and returns the positionals (ok=false on a flag error, already
// reported to fs.Output()).
//
// stdlib flag stops at the first positional, so the documented
// `quality-check <eval.md> -predicates <dir>` order silently DROPPED the flag —
// probed live 2026-07-30: the lint ran on nothing and reported a clean PASS.
// cmdutil.ReorderArgs is bool-flag-only by contract and would scatter
// -predicates' VALUE into the positional list, so this collects positionals one
// at a time and re-parses the remainder (the opscmd console-lease shape). Each
// iteration strictly shrinks `remaining`, so the loop terminates.
//
// Contract note: an unknown flag placed after a positional was previously
// absorbed as an ignored extra positional and now errors (rc 10) — stricter, and
// the only in-repo caller passes a single positional.
```

### `go/internal/cli/guardcmd/guard.go:22` — above `var guardLogTag = map[string]string{`

```text
// guardLogTag maps the Go guard subcommand name to the audit-trail tag
// used in .evolve/guards.log by the bash counterparts. Preserves the
// log format byte-equivalent across the v11.4.0 bash→Go cutover so
// operators / parity audits / external tooling that grep guards.log
// see the same lines.
```

### `go/internal/cli/guardcmd/guard.go:114` — above `if name == "list-audit-fails" {`

```text
// Read-only enumeration subcommand, distinct from the kernel-hook
// guards (ship/phase/role/…). Doesn't take stdin, doesn't emit
// allow/deny — just lists pending audit failures so an operator can
// triage them (issue surfaced by cycle-107 retro: "16 non-expired
// code-audit-fail entries (within 30d retention)" with no way to see
// which entries).
```

### `go/internal/cli/guardcmd/guard_integrity_alarm_test.go:30` — above `func TestRunGuard_RoleControlPlaneEdit_DeniesAndAlarms(t *testing.T) {`

```text
// TestRunGuard_RoleControlPlaneEdit_DeniesAndAlarms is the end-to-end cycle-20
// regression at the hook layer: a build-phase Edit of a protected gate file is
// DENIED (exit 2) AND writes a CRITICAL record to .evolve/integrity-alarm.jsonl.
```

### `go/internal/cli/guardcmd/guard_triage_floors_test.go:11` — above `func triageFloorsFixture(t *testing.T, report, companion string) string {`

```text
// cmd_guard_triage_floors_test.go — ADR-0046 Layer 1 (cycle 305): the
// agent-runnable `evolve guard triage-floors <workspace>` self-check. It reads
// the workspace's triage-report.md + triage-decision.json companion, reports
// committed/deferred declaration-vs-prose divergence, and exits non-zero when
// they disagree so the triage persona can repair its artifact before the
// correction ladder fires (ADR-0045 gate-admission rule).
//
// RED: `triage-floors` is not a recognized subcommand, so runGuard falls through
// to buildGuard, which errors → exit 10 for every case below. Each pin asserts
// the BUILT behavior (exit 0 clean / non-zero divergence / informative --help),
// so all are RED until Builder adds the subcommand branch before stdin parsing.
```

### `go/internal/cli/guardcmd/guard_triage_floors_test.go:23` — above `func triageFloorsFixture(t *testing.T, report, companion string) string {`

```text
// triageFloorsFixture lays out a self-contained project root so the guard can
// source its package vocabulary from go/internal regardless of cwd, with the
// workspace at <root>/.evolve/runs/cycle-305. Returns the workspace path.
```

### `go/internal/cli/guardcmd/preflight.go:37` — above `root := cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")`

```text
// envOrCwd absolutizes a relative $EVOLVE_PROJECT_ROOT (cycle-119 class).
```
