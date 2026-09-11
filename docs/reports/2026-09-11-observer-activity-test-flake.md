# Observer activity test flake investigation

Date: 2026-09-11

## Outcome

The macOS race-and-coverage job for pull request #554 exposed two pre-existing
timing defects in the observer test fixtures:
`TestWatch_GrowthResetsStallTimer` and
`TestWatch_WorkspaceActivityResetsStallTimer`. The production observer correctly
resets its stall clock after stdout or workspace growth. The tests attempted to
prove that behavior by racing a 50 ms writer ticker against a 200 ms stall
deadline and a 300 ms context deadline.

The repaired tests drive the observer's existing package-local clock seam and
write activity synchronously from that clock. They still execute the real
`Watch` polling loop, stdout size check, workspace scan, and event emission, but
their result no longer depends on scheduler latency. No production code changed.

## Failure evidence

GitHub Actions run `34566356939`, job `103159179072`, failed in the macOS
`test (race + cover, incl. integration tier)` step:

```text
--- FAIL: TestWatch_WorkspaceActivityResetsStallTimer (0.31s)
    observer_test.go:260: stall fired while the agent was writing workspace artifacts
--- FAIL: TestWatch_GrowthResetsStallTimer (0.31s)
    observer_test.go:120: stall fired during continuous growth
FAIL github.com/mickeyyaya/evolve-loop/go/internal/adapters/observer
```

The same job passed `go/internal/bridge` with the race detector, integration
tag, and 93.2% statement coverage. Ubuntu Go, repository validation, and the
durable ACS job also passed. The checkpoint extraction therefore remains an
independent batch and is held until this observer fixture repair merges and a
fresh checkpoint-branch CI run passes.

## Root cause

Each old test launched a goroutine that tried to write five times from a 50 ms
ticker. The watcher used a 20 ms polling ticker, a 200 ms stall threshold, and a
300 ms context deadline. Both tests also ran in parallel with the repository's
race-and-coverage suite.

Those durations did not establish a synchronization relationship between the
writer and watcher. Under macOS CI load, the watcher could observe 200 ms of
elapsed wall time before the writer goroutine was scheduled. A
`stall_no_output` event was then correct for the state the watcher actually
observed, even though the test described the fixture as continuous activity.
The shared 0.31 s failure duration and simultaneous failure of both independent
writers match scheduler starvation of the fixture. Repeating the old tests at
low local load did not reproduce the failure, which is why a blind retry would
not validate the contract.

## Deterministic contract

The shared test helper constructs the following logical timeline:

1. `Watch` emits `started` and records `lastGrowth` at `T+0 ms`.
2. On the first no-growth poll, the clock callback writes the configured
   activity synchronously and returns `T+150 ms`.
3. The next real poll observes the stdout size or workspace mtime change. Its
   clock read returns `T+210 ms`, beyond the original 200 ms deadline, and the
   production branch resets `lastGrowth`.
4. The following no-growth poll returns `T+300 ms` and cancels the context.
   Only 90 logical milliseconds have elapsed since the reset, so no stall may
   be emitted.

This sequence proves the reset is necessary: crossing the original deadline
would emit a stall if the activity observation did not update `lastGrowth`.
The five-second context is a deadlock safety bound; it does not determine the
assertion outcome.

The stdout case appends to the real log file. The workspace case creates a real
artifact and advances its mtime beyond the scan baseline, avoiding filesystem
timestamp-resolution assumptions.

## TDD sensitivity and verification

Both repaired tests first passed through the unchanged production path. A
temporary mutation then removed the `lastGrowth` assignment from the production
growth branch. Both tests failed deterministically with `stall_no_output`,
demonstrating that they protect the intended behavior rather than merely
waiting for cancellation. The mutation was removed, and a focused source diff
confirmed that `observer.go` returned to its original contents.

Verification before review:

```text
go test -race -count=100 ./internal/adapters/observer \
  -run 'TestWatch_(GrowthResetsStallTimer|WorkspaceActivityResetsStallTimer)$'
PASS

go test -race -count=20 ./internal/adapters/observer
PASS
```

Additional local verification completed before review:

```text
gofmt -l .
PASS (no files listed)

go vet ./internal/adapters/observer/...
PASS

golangci-lint run ./internal/adapters/observer/...
PASS (0 issues)

go test -count=1 ./...
PASS

go vet ./...
PASS
```

The staged tree is then bound to the required architecture, Go-test,
code-review, simplification, and commit-gate evidence before shipping.

## Independent review

The architecture reviewer, Go-test reviewer, and combined defensive
code-review/simplification reviewer inspected staged tree
`2bba8a2fb5560ca6bff88d868a3bbb63bbb63823`. All three returned PASS with no
findings. They independently confirmed that the logical timeline crosses the
original deadline, removing the production reset makes the test fail, fixture
state remains on the `Watch` goroutine, the filesystem operations are portable,
and the batch changes no production behavior. The reviewers will reconfirm the
final tree after this factual review record is staged.
