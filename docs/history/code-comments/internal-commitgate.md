# Comment history: `internal/commitgate`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/commitgate/apicover_named_test.go:13` — above `func TestExitCodeContract(t *testing.T) {`

```text
// apicover_named_test.go names + exercises the exported symbols that the
// behavior tests don't already reach, satisfying the ADR-0050 Phase 5 hard gate
// (./internal/commitgate is enrolled in go/.apicover-enforce). The Exit*
// constants ExitPass/ExitFail/ExitToolMissing are named by commitgate_test.go;
// the two below are pinned here. Options.Run, Attestation.Marshal, Options,
// Result, Runner, and Attestation are all exercised by the behavior/parity
// tests.
```

### `go/internal/commitgate/lanes.go:66` — above `var unformatted []string`

```text
// gofmt -s -l: matches CI's `gofmt -d -s`. Plain gofmt would pass code that
// CI then rejects (recurring gofmt-not-simplify incident).
```

### `go/internal/commitgate/lanes_test.go:3` — above `import (`

```text
// lanes_test.go — RED contract for cycle-549's cli-command-layer-test-coverage
// task (triage-report.md top_n item, fleet_scope
// cli-command-layer-test-coverage-worktree-swarm's "commitgate (incl. non-Go
// lane fixtures/removal)" clause). lanePython, isPyTest, laneNode, and
// laneRust — the non-Go commit-gate lanes — had ZERO direct test coverage
// (0.0% per `go tool cover -func`) even though laneGo (their sibling) is
// already well covered in commitgate_test.go, whose baseOpts/scriptRunner
// fixture harness this file reuses verbatim.
```
