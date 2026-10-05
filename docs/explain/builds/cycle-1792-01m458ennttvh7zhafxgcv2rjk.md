# Build Explanation — Cycle 1792

## Build Binding
- Cycle: 1792
- Base SHA: 0246cdcfbd7cecb7e2c0ca36f6fcd2334448db8c

## Summary
Three operator verbs join the `evolve` CLI. `evolve pr merge <n>... [--update-branch] [--wait D]` merges reviewed PRs at a wave boundary. It refuses while any run lease in the plane is live, and it merges only at a head whose newest `required.yml` run is green. `evolve ci classify <run-id|pr:N|sha:H> [--json] [--rerun]` labels every failing test of a red run as real, pre-existing, flake-evidence or unknown. Each label comes from four evidence facts and one rule table. `evolve comments ...` is the commentaudit CLI, run through the same `commentaudit.Main`.

## Rationale
Three wave-boundary steps were manual: merging (`gh pr checks`, then `update-branch`, then `merge`), classifying CI failures (`gh run view --log-failed` read by eye) and running the comment proofs (`go run ./cmd/commentaudit`). Two of these failed in practice: a PR was merged mid-wave, and another was merged over a red required check. Each verb reuses an existing seam:
- `runlease.LiveRuns` for loop liveness;
- `ciwatch.NewGHFetcher` for the required run;
- `commentaudit.Main` for the comment proofs.

So no step gets a second definition. A shell wrapper was rejected because the runtime reference requires boundary controls to be CLI verbs. An allowlist of commentaudit subcommands was rejected because it would make the two entry points diverge.

## Changed Areas
- `go/cmd/evolve/cmd_pr.go` — `evolve pr merge`.
  - Processes PRs one at a time, in operand order, and stops at the first refusal.
  - Scans the plane's leases across every `git worktree list` entry: at launch, and again right before each merge.
  - Checks state, draft and conflicts, then compares against the base for `behind_by`. `--update-branch` runs `gh pr update-branch`, which merges and never rebases.
  - Waits for a green required run, polling at `ci_watch.poll_s` up to `--wait`. A red run refuses at once.
  - Merges with `gh pr merge --<pr.merge_method> --match-head-commit <verified head>`, then confirms that the PR is `MERGED` with a 40-hex merge SHA.
  - Exit codes: 1 refused, 2 I/O or policy error, 10 usage.
  - It also holds the small interleaved flag parser that `ci classify` shares.
- `go/cmd/evolve/cmd_pr_test.go` — covers argument parsing, the wait-then-merge path under an injected clock, every refusal class stopping before any later PR, gh and merge-confirmation failures exiting 2, and the sibling-worktree lease scan.
- `go/cmd/evolve/cmd_ci.go` — `evolve ci classify`. It parses the target, reads `ci_watch` timing and the `go/go.mod` module path, and calls `ciwatch.ClassifyTarget` with the gh source.
  - Output: the run line, the tab-separated table and `retry-safe: yes|no`, or the JSON report.
  - Exit codes: 0 retry-safe, 1 not retry-safe, unfinished or no run, 2 I/O, 10 usage.
- `go/cmd/evolve/cmd_ci_test.go` — covers target and flag parsing, usage exit 10, module-path reading, the exact text table and JSON shape, and `runComments` parity with `commentaudit.Main`.
- `go/cmd/evolve/cmd_comments.go` — `evolve comments` forwards its arguments verbatim to `commentaudit.Main` with `commentaudit.ExecGit{}`.
- `go/cmd/evolve/registry.go` — adds the `pr`, `ci` and `comments` rows to the dispatch table.
- `go/cmd/evolve/main.go` — lists the three verbs in the usage text.
- `go/internal/ciwatch/target.go` — the target grammar, `ParseTarget`:
  - a bare number is a run id;
  - a PR needs the `pr:` prefix;
  - SHAs are lowercased;
  - an all-digit short SHA needs `sha:`.
- `go/internal/ciwatch/classify.go` — the classification vocabulary, the ordered `classifyRules` table (base-red, touched, rerun-green, recurred-on-main, default) and `ClassifyTarget`.
  - Evidence is gathered only for red runs.
  - A recurrence on main excludes runs on the head and runs created at or after the target run.
  - A rerun result counts only from a newer attempt.
  - A red run whose log names nothing keeps one job-level `unknown` failure.
- `go/internal/ciwatch/classify_source.go` — the gh-backed `ClassifySource`.
  - Resolves run, PR and SHA targets. The base is the PR's merge base, else the head's first parent when main contains the head, else the merge base with main.
  - Lists main runs before a time and reruns failed jobs.
  - Parses `--log-failed` into package/test/job failures with `parseFailingTests`.
  - Every gh error propagates, unlike `failedLogSummary`, which swallows them.
- `go/internal/ciwatch/classify_test.go` — one test per rule row, all 81 evidence combinations against the flake invariant, the target grammar, the log parser, the orchestrator over a fake source, and the gh source through the `execCapture` seam.
- `go/internal/ciwatch/ciwatch.go` — `RunStatus` gains additive fields: `RunID`, `HeadSHA`, `Attempt`, `CreatedAt`, `FailingTests`.
- `go/internal/ciwatch/ghfetcher.go` — the fetcher now also fills `RunID`. Its calls and its other behavior are unchanged.
- `go/internal/commentaudit/execgit.go` — `ExecGit`, the real git seam, moved verbatim from `cmd/commentaudit/main.go` so both binaries can import it.
- `go/internal/commentaudit/execgit_test.go` — the two moved `ExecGit` tests, rebuilt on `gittest.Fixture`, plus a Root/Show/ChangedFiles test.
- `go/cmd/commentaudit/main.go` — now a thin wrapper over `commentaudit.Main` with `commentaudit.ExecGit{}`.
- `go/cmd/commentaudit/main_test.go` — removed. Its tests moved into `internal/commentaudit` with the type.
- `go/internal/rawgitratchet/baseline.json` — drops the deleted file's entry (2 raw `git init` sites). The moved tests use `gittest`.
- `go/internal/policy/policy_pr.go` — the `pr.merge_method` policy key and `Policy.PRMergeMethod`. It defaults to `merge`, accepts `squash` and `rebase` case-sensitively, and fails closed on any other word.
- `go/internal/policy/policy.go` — adds the `PR` field to `Policy`.
- `go/internal/policy/policy_pr_test.go` — covers the default, the three accepted methods and the rejected words.
- `docs/operations/runtime-reference.md` — boundary step 2 now runs `evolve pr merge`, and `evolve ci classify` and `evolve comments` are documented beside `evolve status`.
- `CLAUDE.md` — the CI Failures rule cites `evolve ci classify`.
- `docs/conventions/code-comments.md` — the enforcement bullets run `evolve comments ...` and note that both entry points share one `Main`.
- `.evolve/evals/cli-pr-merge.md` — the eval for the pr-merge item (TDD-authored, tracked with the build).
- `.evolve/evals/cli-ci-classify.md` — the eval for the ci-classify item (TDD-authored).
- `.evolve/evals/cli-comments.md` — the eval for the comments item (TDD-authored).
- `go/acs/cycle1792/predicates_test.go` — the 23 acceptance predicates (TDD-authored). They drive the built binary against a fake `gh` and git fixtures.

## Design Decisions
- **What "required CI" means.** It means the newest `required.yml` run on the head, read through `ciwatch.NewGHFetcher`. The repository reports no branch-protection required checks, so `gh pr checks --required` would always fail.
- **The plane is every worktree of the shared store.** A merge started from the console must see the runtime loop's leases.
- **Merge methods.** `pr.merge_method` is a new policy key rather than part of `merge_gate`, which governs lane promotion. An action that writes to main fails closed on a bad value.
- **One rule table.** The table in `ciwatch` is the single source of labels. Flake-evidence requires positive evidence (a green newer attempt, or recurrence on main with no red rerun), and an untouched package alone never qualifies. Missing evidence is `unknown`, never `no`.
- **Comment operands.** `evolve comments` adds no argument rewriting, because `commentaudit` already resolves relative directories (cwd-relative, else repo-root-relative).

## Verification
- The cycle1792 ACS predicates pass 23/23.
- New unit tests pass in `./internal/ciwatch`, `./internal/policy`, `./internal/commentaudit` and `./cmd/evolve`.
- `gofmt -l` is clean, and so is `go vet ./...`.
- `commentaudit comments -base HEAD` reports no added comments.

## Compatibility
The change is additive: new verbs, one new optional policy block and new `RunStatus` fields on keyed literals. `go run ./cmd/commentaudit` keeps its arguments, output and exit codes. The checked-in `.evolve/policy.json` is unchanged, so the merge method defaults to `merge`.

## Limitations
- **Update-branch timing.** GitHub may apply `gh pr update-branch` asynchronously. If the re-viewed head has not moved yet, the pre-merge behind check refuses ("base moved during the wait") instead of merging, and the operator reruns.
- **Classify's view of main.** Classify reads the compare API's file list, capped at 300 files (treated as truncated, so `touched` becomes unknown), and only the 10 newest completed main runs before the target.
