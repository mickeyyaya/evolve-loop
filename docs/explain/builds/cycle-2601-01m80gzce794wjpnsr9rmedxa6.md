# Build Explanation — Cycle 2601

## Build Binding
- Cycle: 2601
- Base SHA: f34d4bc1d52cc4b4b7d5eb13d2306889ade61b33

## Summary
The `gc` package now provides a check registry and the `evolve gc check` CLI subcommand, starting with the worktree check judged against the fetched `origin/main` landing target.

## Rationale
Judging cycle branches against the hub `HEAD` let unpushed integration merges look landed and delete branches prematurely. Adding `WorktreeOptions.Landing` lets the check and re-check judge against fetched `origin/main` and treat a failed fetch as not landed. Keeping the zero value on `HEAD` preserves existing sweep callers.

## Changed Areas
- `go/internal/gc/worktrees.go` — adds `LandingTarget` and `WorktreeOptions.Landing`. It fetches the landing target, degrades a failed fetch to `flag-unmerged`, records tips in `Head`, and re-checks landing before `update-ref -d`.
- `go/internal/gc/branchsweep.go` — passes the landing target display label to `removalReasonFor`.
- `go/internal/gc/devleftovers.go` — extracts `gateLeftovers` from `*devSweep` into a package-level helper while keeping the `devSweep` adapter method.
- `go/internal/gc/check.go` — adds `Check`, `CheckContext`, `CheckItem`, `CheckResult`, `CheckReport`, `DefaultChecks`, and `RunChecks`.
- `go/internal/gc/check_worktrees.go` — adapts `PlanWorktrees` and `ApplyWorktrees` behind `worktreeCheck` with `Landing{Remote: "origin", Branch: "main"}`, mapping each item to `subject`, `action`, `rule`, and `evidence`.
- `go/cmd/evolve/cmd_gc.go` — routes `evolve gc check` to `runGCCheck`.
- `go/cmd/evolve/cmd_gc_check.go` — implements `evolve gc check [--project-root DIR] [--json]` as a read-only check command.
- `go/cmd/evolve/cli_contract.go` — prevents `coreVerbOf` from wrapping `gc check --json` inside the `evolve.gc/1` envelope so stdout emits raw `CheckReport` JSON.
- `go/internal/gc/check_test.go` — adds unit and fixture tests covering `DefaultChecks`, `RunChecks`, `LandingTarget` validation and failure paths, `ApplyWorktrees` landing re-checks, and `gateLeftovers`.
- `go/cmd/evolve/cmd_gc_check_test.go` — adds CLI mode, flag, cwd fallback, and error tests for `runGCCheck`.
- `docs/architecture/packages/internal-gc.md` — documents the check registry, `LandingTarget`, fetch-failure safety, and `gateLeftovers` extraction.

## Design Decisions
`WorktreeOptions.Landing` defaults to zero (`HEAD`) so existing in-loop and unit-test sweep callers stay unchanged, while `worktreeCheck` sets `origin/main` explicitly. When `Landing` is set, `ApplyWorktrees` fetches and verifies landing by ancestor or squash check before running `update-ref -d <ref> <expected>`, refusing any branch not landed on `origin/main`. `Check.Apply` has no CLI caller yet because `evolve gc check` is report-only; a later item wires the sweep.

## Verification
All 9 cycle 2601 ACS predicates (`go test -tags acs -count=1 ./acs/cycle2601`), `go/internal/gc` tests, `go/cmd/evolve` tests, `apicover-enforce` on `./internal/gc`, and `ste-lint --strict` on `docs/architecture/packages/internal-gc.md` verify the change.

## Compatibility
Zero-value `WorktreeOptions` keeps `HEAD` semantics and `git branch -d` behavior, preserving all existing worktree sweep tests unchanged. `evolve gc check --json` emits the top-level `{"checks":[...]}` schema directly.

## Limitations
`Check.Apply` is implemented on `worktreeCheck` and covered by unit tests, but no `evolve gc` mutation path invokes the check registry yet. `evolve gc check` remains class D in `agentclass` until its table entry is added separately.
